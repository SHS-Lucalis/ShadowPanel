package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultChunkSize    = 64 * 1024
	maxFileSize         = 10 * 1024 * 1024 * 1024
	chunkReceiveTimeout = 60 * time.Second
	transferMaxAge      = 24 * time.Hour
	transferPrefix      = "transfers/"
)

type Service struct {
	proto.UnimplementedFileTransferServiceServer

	storage files.StreamFileManager
	pub     pubsub.Publisher
	logger  *slog.Logger

	mu              sync.RWMutex
	activeTransfers map[string]*activeTransfer
}

type activeTransfer struct {
	TransferID string
	Path       string
	TotalSize  int64
	Written    int64
	StartedAt  time.Time
	Cancel     context.CancelFunc
}

func NewService(storage files.StreamFileManager, pub pubsub.Publisher, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		storage:         storage,
		pub:             pub,
		logger:          logger,
		activeTransfers: make(map[string]*activeTransfer),
	}
}

func transferDataPath(transferID string) string {
	return transferPrefix + transferID + "/data"
}

func (s *Service) UploadFile(stream proto.FileTransferService_UploadFileServer) error {
	ctx := stream.Context()

	firstChunk, err := stream.Recv()
	if err != nil {
		return status.Error(codes.InvalidArgument, "failed to receive first chunk")
	}

	metadata := firstChunk.Metadata
	if metadata == nil {
		return status.Error(codes.InvalidArgument, "first chunk must contain metadata")
	}

	if metadata.TotalSize > maxFileSize {
		return status.Error(codes.InvalidArgument, "file too large")
	}

	transferCtx, cancel := context.WithCancel(ctx)
	s.registerTransfer(metadata.TransferId, metadata.Path, metadata.TotalSize, cancel)
	defer func() {
		s.unregisterTransfer(metadata.TransferId)
		cancel()
	}()

	totalWritten, checksum, err := s.receiveAndStore(transferCtx, stream, metadata, firstChunk.Data)
	if err != nil {
		_ = s.storage.Delete(context.Background(), transferDataPath(metadata.TransferId))
		s.publishTransferComplete(metadata.TransferId, false, "", err.Error())

		return err
	}

	if metadata.ChecksumSha256 != "" && checksum != metadata.ChecksumSha256 {
		_ = s.storage.Delete(context.Background(), transferDataPath(metadata.TransferId))
		s.publishTransferComplete(metadata.TransferId, false, checksum, "checksum mismatch")

		return status.Error(codes.DataLoss, "checksum mismatch")
	}

	s.publishTransferComplete(metadata.TransferId, true, checksum, "")

	s.logger.Info("file upload completed",
		"transfer_id", metadata.TransferId,
		"bytes_written", totalWritten,
		"checksum", checksum,
	)

	return stream.SendAndClose(&proto.UploadResult{
		Success:        true,
		BytesWritten:   totalWritten,
		ChecksumSha256: checksum,
	})
}

func (s *Service) receiveAndStore(
	ctx context.Context,
	stream proto.FileTransferService_UploadFileServer,
	metadata *proto.UploadMetadata,
	firstChunkData []byte,
) (int64, string, error) {
	pr, pw := io.Pipe()
	hasher := sha256.New()
	writer := io.MultiWriter(pw, hasher)

	var storeErr error
	var storeWg sync.WaitGroup

	storeWg.Go(func() {
		storagePath := transferDataPath(metadata.TransferId)
		storeErr = s.storage.WriteStream(ctx, storagePath, pr)
	})

	var totalWritten int64

	writeData := func(data []byte) error {
		n, err := writer.Write(data)
		if err != nil {
			return status.Error(codes.Internal, "failed to write data")
		}
		totalWritten += int64(n)
		s.updateTransferProgress(metadata.TransferId, metadata.Offset+totalWritten)

		return nil
	}

	if len(firstChunkData) > 0 {
		if err := writeData(firstChunkData); err != nil {
			pw.CloseWithError(err)
			storeWg.Wait()

			return totalWritten, "", err
		}
	}

	for {
		ch := make(chan recvResult, 1)
		go func() {
			chunk, err := stream.Recv()
			ch <- recvResult{chunk, err}
		}()

		var res recvResult
		select {
		case <-ctx.Done():
			pw.CloseWithError(ctx.Err())
			storeWg.Wait()

			return totalWritten, "", status.Error(codes.Canceled, "transfer canceled")
		case <-time.After(chunkReceiveTimeout):
			pw.CloseWithError(errors.New("chunk receive timeout"))
			storeWg.Wait()

			return totalWritten, "", status.Error(codes.DeadlineExceeded, "chunk receive timeout")
		case res = <-ch:
		}

		if errors.Is(res.err, io.EOF) {
			break
		}
		if res.err != nil {
			pw.CloseWithError(res.err)
			storeWg.Wait()

			return totalWritten, "", status.Error(codes.Internal, "failed to receive chunk")
		}

		if len(res.chunk.Data) == 0 {
			continue
		}

		if err := writeData(res.chunk.Data); err != nil {
			pw.CloseWithError(err)
			storeWg.Wait()

			return totalWritten, "", err
		}
	}

	_ = pw.Close()
	storeWg.Wait()

	if storeErr != nil {
		return totalWritten, "", status.Error(codes.Internal, "failed to store file")
	}

	return totalWritten, hex.EncodeToString(hasher.Sum(nil)), nil
}

type recvResult struct {
	chunk *proto.UploadChunk
	err   error
}

func (s *Service) DownloadFile(
	req *proto.DownloadRequest,
	stream proto.FileTransferService_DownloadFileServer,
) error {
	storagePath := transferDataPath(req.Path)

	if !s.storage.Exists(stream.Context(), storagePath) {
		return status.Error(codes.NotFound, "transfer file not found")
	}

	var reader io.ReadCloser
	var err error

	if req.Offset > 0 {
		reader, err = s.storage.ReadStreamAt(stream.Context(), storagePath, req.Offset)
	} else {
		reader, err = s.storage.ReadStream(stream.Context(), storagePath)
	}
	if err != nil {
		return status.Error(codes.Internal, "failed to open file for reading")
	}
	defer func() { _ = reader.Close() }()

	hasher := sha256.New()
	buf := make([]byte, defaultChunkSize)
	offset := req.Offset
	var totalSent int64

	for {
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "transfer canceled")
		default:
		}

		n, readErr := reader.Read(buf)
		if n > 0 {
			hasher.Write(buf[:n])
			isFinal := readErr == io.EOF

			chunk := &proto.DownloadChunk{
				Data:    buf[:n],
				Offset:  offset,
				IsFinal: isFinal,
			}

			if isFinal {
				chunk.ChecksumSha256 = hex.EncodeToString(hasher.Sum(nil))
			}

			if err := stream.Send(chunk); err != nil {
				return status.Error(codes.Internal, "failed to send chunk")
			}

			offset += int64(n)
			totalSent += int64(n)
		}

		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return status.Error(codes.Internal, "failed to read file")
		}
	}

	return nil
}

func (s *Service) publishTransferComplete(transferID string, success bool, checksum, errMsg string) {
	channel := channels.BuildDaemonFileTransferCompleteChannel(transferID)
	msg, err := messages.NewMessage(channel, messages.TypeDaemonFileTransferComplete, messages.FileTransferCompletePayload{
		TransferID: transferID,
		Success:    success,
		Error:      errMsg,
		Checksum:   checksum,
	})
	if err != nil {
		s.logger.Error("failed to create transfer complete message", "error", err)

		return
	}

	if err := s.pub.Publish(context.Background(), channel, msg); err != nil {
		s.logger.Error("failed to publish transfer complete event",
			"transfer_id", transferID,
			"error", err,
		)
	}
}

func (s *Service) registerTransfer(
	transferID, path string,
	totalSize int64,
	cancel context.CancelFunc,
) {
	s.mu.Lock()
	s.activeTransfers[transferID] = &activeTransfer{
		TransferID: transferID,
		Path:       path,
		TotalSize:  totalSize,
		StartedAt:  time.Now(),
		Cancel:     cancel,
	}
	s.mu.Unlock()
}

func (s *Service) unregisterTransfer(transferID string) {
	s.mu.Lock()
	delete(s.activeTransfers, transferID)
	s.mu.Unlock()
}

func (s *Service) updateTransferProgress(transferID string, written int64) {
	s.mu.Lock()
	if transfer, ok := s.activeTransfers[transferID]; ok {
		transfer.Written = written
	}
	s.mu.Unlock()
}

func (s *Service) CancelTransfer(transferID string) bool {
	s.mu.Lock()
	transfer, ok := s.activeTransfers[transferID]
	if ok && transfer.Cancel != nil {
		transfer.Cancel()
	}
	s.mu.Unlock()

	return ok
}

func (s *Service) CancelAll() {
	s.mu.Lock()
	for _, transfer := range s.activeTransfers {
		if transfer.Cancel != nil {
			transfer.Cancel()
		}
	}
	s.mu.Unlock()
}

func (s *Service) WaitForCompletion() {
	for {
		s.mu.RLock()
		count := len(s.activeTransfers)
		s.mu.RUnlock()

		if count == 0 {
			return
		}

		time.Sleep(100 * time.Millisecond)
	}
}

func (s *Service) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.cleanupStaleTransfers(ctx); err != nil {
					s.logger.Error("transfer cleanup failed", "error", err)
				}
			}
		}
	}()
}

func (s *Service) cleanupStaleTransfers(ctx context.Context) error {
	entries, err := s.storage.List(ctx, transferPrefix)
	if err != nil {
		return errors.Wrap(err, "list transfers for cleanup")
	}

	var cleaned int

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if !strings.HasSuffix(entry, "/data") {
			continue
		}

		path := transferPrefix + entry
		if !s.storage.Exists(ctx, path) {
			continue
		}

		_ = s.storage.Delete(ctx, path)
		cleaned++
	}

	if cleaned > 0 {
		s.logger.Info("cleaned up stale transfers", "count", cleaned)
	}

	return nil
}
