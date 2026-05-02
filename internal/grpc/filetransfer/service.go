package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/internal/transfers"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultChunkSize    = 64 * 1024
	chunkReceiveTimeout = 60 * time.Second
	// TODO: enforce transferMaxAge in cleanupStaleTransfers. Currently unused —
	// requires extending files.StreamFileManager with a Stat / mtime API so the
	// cleanup worker can distinguish recent abandoned transfers from old ones.
	// Today cleanup only skips transfers present in the in-memory activeTransfers
	// map; anything else gets removed regardless of age.
	transferMaxAge       = 24 * time.Hour
	transferPrefix       = "transfers/"
	sentinelWriteRetries = 3
	sentinelRetryDelay   = 500 * time.Millisecond
)

type Service struct {
	proto.UnimplementedFileTransferServiceServer

	storage     files.StreamFileManager
	pub         pubsub.Publisher
	transferReg *transfers.Registry
	logger      *slog.Logger

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

func NewService(
	storage files.StreamFileManager,
	pub pubsub.Publisher,
	transferReg *transfers.Registry,
	logger *slog.Logger,
) *Service {
	if logger == nil {
		logger = slog.Default()
	}

	return &Service{
		storage:         storage,
		pub:             pub,
		transferReg:     transferReg,
		logger:          logger,
		activeTransfers: make(map[string]*activeTransfer),
	}
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

	transferCtx, cancel := context.WithCancel(ctx)
	s.registerTransfer(metadata.TransferId, metadata.Path, metadata.TotalSize, cancel)
	defer func() {
		s.unregisterTransfer(metadata.TransferId)
		cancel()
	}()

	totalWritten, checksum, partsWritten, err := s.receiveAndStore(transferCtx, stream, metadata, firstChunk.Data)
	if err != nil {
		s.logger.Error("file upload failed",
			"transfer_id", metadata.TransferId,
			"parts_written", partsWritten,
			"bytes_written", totalWritten,
			"error", err,
		)
		s.writeSentinel(metadata.TransferId, partsWritten, "", err.Error())
		if state, ok := s.transferReg.Get(metadata.TransferId); ok {
			state.SetError(err)
		}
		s.publishTransferComplete(metadata.TransferId, false, "", err.Error())

		return err
	}

	if metadata.ChecksumSha256 != "" && checksum != metadata.ChecksumSha256 {
		errMsg := fmt.Sprintf("checksum mismatch: expected %s, got %s", metadata.ChecksumSha256, checksum)
		s.logger.Error("file upload checksum mismatch",
			"transfer_id", metadata.TransferId,
			"expected", metadata.ChecksumSha256,
			"actual", checksum,
		)
		s.writeSentinel(metadata.TransferId, partsWritten, checksum, errMsg)
		if state, ok := s.transferReg.Get(metadata.TransferId); ok {
			state.SetError(errors.New(errMsg))
		}
		s.publishTransferComplete(metadata.TransferId, false, checksum, errMsg)

		return status.Error(codes.DataLoss, errMsg)
	}

	s.writeSentinel(metadata.TransferId, partsWritten, checksum, "")
	if state, ok := s.transferReg.Get(metadata.TransferId); ok {
		state.Complete()
	}
	s.publishTransferComplete(metadata.TransferId, true, checksum, "")

	s.logger.Info("file upload completed",
		"transfer_id", metadata.TransferId,
		"parts_written", partsWritten,
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
) (int64, string, int, error) {
	hasher := sha256.New()
	buf := make([]byte, 0, transfers.MaxPartSize+defaultChunkSize)

	var (
		totalWritten    int64
		partNum         int
		currentPartSize = transfers.PartSizeForNum(0)
	)

	flushPart := func(data []byte) error {
		hasher.Write(data)
		partPath := transfers.TransferPartPath(metadata.TransferId, partNum)

		if err := s.storage.Write(ctx, partPath, data); err != nil {
			return errors.Wrapf(err, "write part %d to storage", partNum)
		}

		if state, ok := s.transferReg.Get(metadata.TransferId); ok {
			state.AddPart()
		} else {
			s.logger.Warn("transfer state not found in registry",
				"transfer_id", metadata.TransferId, "part", partNum)
		}

		totalWritten += int64(len(data))
		s.updateTransferProgress(metadata.TransferId, metadata.Offset+totalWritten)
		partNum++

		return nil
	}

	if len(firstChunkData) > 0 {
		buf = append(buf, firstChunkData...)

		for len(buf) >= currentPartSize {
			if err := flushPart(buf[:currentPartSize]); err != nil {
				return totalWritten, "", partNum, err
			}
			buf = append(buf[:0], buf[currentPartSize:]...)
			currentPartSize = transfers.PartSizeForNum(partNum)
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
			return totalWritten, "", partNum,
				status.Errorf(codes.Canceled, "transfer canceled (transfer %s, written %d)", metadata.TransferId, totalWritten)
		case <-time.After(chunkReceiveTimeout):
			return totalWritten, "", partNum,
				status.Errorf(codes.DeadlineExceeded, "chunk receive timeout after %s (transfer %s, written %d)",
					chunkReceiveTimeout, metadata.TransferId, totalWritten)
		case res = <-ch:
		}

		if errors.Is(res.err, io.EOF) {
			break
		}
		if res.err != nil {
			return totalWritten, "", partNum,
				status.Errorf(codes.Internal, "receive chunk: %s", res.err)
		}

		if len(res.chunk.Data) == 0 {
			continue
		}

		buf = append(buf, res.chunk.Data...)

		for len(buf) >= currentPartSize {
			if err := flushPart(buf[:currentPartSize]); err != nil {
				return totalWritten, "", partNum, err
			}
			buf = append(buf[:0], buf[currentPartSize:]...)
			currentPartSize = transfers.PartSizeForNum(partNum)
		}
	}

	if len(buf) > 0 {
		if err := flushPart(buf); err != nil {
			return totalWritten, "", partNum, err
		}
	}

	return totalWritten, hex.EncodeToString(hasher.Sum(nil)), partNum, nil
}

type recvResult struct {
	chunk *proto.UploadChunk
	err   error
}

func (s *Service) DownloadFile(
	req *proto.DownloadRequest,
	stream proto.FileTransferService_DownloadFileServer,
) error {
	storagePath := transfers.TransferDataPath(req.Path)

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

	hasher, err := s.newDownloadHasher(stream.Context(), storagePath, req)
	if err != nil {
		return err
	}

	buf := make([]byte, defaultChunkSize)
	offset := req.Offset
	var totalSent int64
	finalSent := false
	finalChecksum := ""

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
				finalChecksum = chunk.ChecksumSha256
			}

			if err := stream.Send(chunk); err != nil {
				return status.Error(codes.Internal, "failed to send chunk")
			}
			if isFinal {
				finalSent = true
			}

			offset += int64(n)
			totalSent += int64(n)
		}

		if readErr == io.EOF {
			if !finalSent {
				sentChecksum, finalErr := sendFinalDownloadChunk(stream, hasher, offset)
				if finalErr != nil {
					return finalErr
				}
				finalChecksum = sentChecksum
			}

			break
		}
		if readErr != nil {
			return status.Error(codes.Internal, "failed to read file")
		}
	}

	s.logger.Debug("download stream finished",
		"transfer_id", req.Path,
		"offset", req.Offset,
		"bytes_sent", totalSent,
		"checksum_sha256", finalChecksum,
	)

	return nil
}

func (s *Service) newDownloadHasher(
	ctx context.Context,
	storagePath string,
	req *proto.DownloadRequest,
) (hash.Hash, error) {
	hasher := sha256.New()
	if req.Offset <= 0 {
		return hasher, nil
	}

	prefixReader, openErr := s.storage.ReadStream(ctx, storagePath)
	if openErr != nil {
		return nil, status.Error(codes.Internal, "failed to open file for checksum prehash")
	}

	prefixedBytes, copyErr := io.CopyN(hasher, prefixReader, req.Offset)
	closeErr := prefixReader.Close()
	if copyErr != nil {
		if errors.Is(copyErr, io.EOF) {
			return nil, status.Error(codes.InvalidArgument, "offset exceeds file size")
		}

		return nil, status.Error(codes.Internal, "failed to prehash file prefix")
	}
	if closeErr != nil {
		return nil, status.Error(codes.Internal, "failed to close prehash stream")
	}

	s.logger.Debug("download stream resumed",
		"transfer_id", req.Path,
		"offset", req.Offset,
		"prefixed_bytes", prefixedBytes,
	)

	return hasher, nil
}

func sendFinalDownloadChunk(
	stream proto.FileTransferService_DownloadFileServer,
	hasher hash.Hash,
	offset int64,
) (string, error) {
	checksum := hex.EncodeToString(hasher.Sum(nil))
	finalChunk := &proto.DownloadChunk{
		Data:           nil,
		Offset:         offset,
		IsFinal:        true,
		ChecksumSha256: checksum,
	}

	if err := stream.Send(finalChunk); err != nil {
		return "", status.Error(codes.Internal, "failed to send final chunk")
	}

	return checksum, nil
}

func (s *Service) writeSentinel(transferID string, totalParts int, checksum, errMsg string) {
	info := transfers.DoneInfo{
		Success:    errMsg == "",
		Checksum:   checksum,
		TotalParts: totalParts,
		Error:      errMsg,
	}

	data, marshalErr := json.Marshal(info)
	if marshalErr != nil {
		s.logger.Error("failed to marshal sentinel", "transfer_id", transferID, "error", marshalErr)

		return
	}

	donePath := transfers.TransferDonePath(transferID)

	for attempt := range sentinelWriteRetries {
		if err := s.storage.Write(context.Background(), donePath, data); err != nil {
			s.logger.Error("failed to write sentinel file",
				"transfer_id", transferID, "attempt", attempt+1, "error", err)

			if attempt < sentinelWriteRetries-1 {
				time.Sleep(sentinelRetryDelay)
			}

			continue
		}

		return
	}

	s.logger.Error("sentinel write failed after retries",
		"transfer_id", transferID, "retries", sentinelWriteRetries)
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

	seen := make(map[string]bool)
	cleaned := 0

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		transferID := transferIDFromListEntry(entry)
		if transferID == "" || seen[transferID] {
			continue
		}
		seen[transferID] = true

		s.mu.RLock()
		_, inFlight := s.activeTransfers[transferID]
		s.mu.RUnlock()
		if inFlight {
			continue
		}

		prefix := transferPrefix + transferID + "/"
		if err := s.storage.DeleteByPrefix(ctx, prefix); err != nil {
			s.logger.Warn("failed to cleanup transfer", "transfer_id", transferID, "error", err)

			continue
		}
		cleaned++
	}

	if cleaned > 0 {
		s.logger.Info("cleaned up stale transfers", "count", cleaned)
	}

	return nil
}

// transferIDFromListEntry extracts the transfer id from a List() entry,
// tolerating any of the three storage backend conventions:
//   - LocalFileManager: "<id>" (basename, non-recursive listing)
//   - S3FileManager:    "<id>/" (relative key with trailing slash)
//   - InMemoryFileManager: "transfers/<id>/parts/000000" (full recursive path)
func transferIDFromListEntry(entry string) string {
	rel := strings.TrimPrefix(entry, transferPrefix)
	id, _, _ := strings.Cut(rel, "/")

	return id
}
