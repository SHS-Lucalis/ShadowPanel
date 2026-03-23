package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	defaultChunkSize    = 64 * 1024
	maxFileSize         = 10 * 1024 * 1024 * 1024
	chunkReceiveTimeout = 60 * time.Second
	tempFilePrefix      = ".tmp_"
	tempFileMaxAge      = 24 * time.Hour
)

type Service struct {
	proto.UnimplementedFileTransferServiceServer

	basePath string
	logger   *slog.Logger

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

func NewService(basePath string, logger *slog.Logger) *Service {
	if logger == nil {
		logger = slog.Default()
	}
	return &Service{
		basePath:        basePath,
		logger:          logger,
		activeTransfers: make(map[string]*activeTransfer),
	}
}

func (s *Service) CleanupStaleTempFiles(ctx context.Context) error {
	if s.basePath == "" {
		return nil
	}

	var cleaned int
	err := filepath.Walk(s.basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if info.IsDir() {
			return nil
		}

		if !strings.HasPrefix(info.Name(), tempFilePrefix) {
			return nil
		}

		if time.Since(info.ModTime()) > tempFileMaxAge {
			if err := os.Remove(path); err != nil {
				s.logger.Warn("failed to remove stale temp file",
					"path", path,
					"error", err,
				)
			} else {
				cleaned++
				s.logger.Debug("removed stale temp file", "path", path)
			}
		}

		return nil
	})

	if cleaned > 0 {
		s.logger.Info("cleaned up stale temp files", "count", cleaned)
	}

	return err
}

func (s *Service) StartCleanupWorker(ctx context.Context, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		if err := s.CleanupStaleTempFiles(ctx); err != nil {
			s.logger.Error("initial temp file cleanup failed", "error", err)
		}

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := s.CleanupStaleTempFiles(ctx); err != nil {
					s.logger.Error("temp file cleanup failed", "error", err)
				}
			}
		}
	}()
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

	safePath, err := s.safePath(metadata.Path)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid path")
	}

	dir := filepath.Dir(safePath)
	if metadata.CreateDirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return status.Error(codes.Internal, "failed to create directories")
		}
	}

	result, err := s.processUpload(ctx, stream, metadata, safePath, dir, firstChunk.Data)
	if err != nil {
		return err
	}

	return stream.SendAndClose(result)
}

func (s *Service) processUpload(
	ctx context.Context,
	stream proto.FileTransferService_UploadFileServer,
	metadata *proto.UploadMetadata,
	safePath, dir string,
	firstChunkData []byte,
) (*proto.UploadResult, error) {
	tempPath := filepath.Join(dir, ".tmp_"+metadata.TransferId)

	file, err := os.OpenFile(tempPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(metadata.Mode))
	if err != nil {
		s.logger.Error("failed to open temp file for writing",
			"path", tempPath,
			"error", err,
		)
		return nil, status.Error(codes.Internal, "failed to create file")
	}

	transferCtx, cancel := context.WithCancel(ctx)
	s.registerTransfer(metadata.TransferId, safePath, metadata.TotalSize, cancel)

	cleanup := func() {
		_ = file.Close()
		_ = os.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()
	}

	totalWritten, checksum, err := s.receiveChunks(transferCtx, stream, file, metadata, firstChunkData)
	if err != nil {
		cleanup()
		return nil, err
	}

	_ = file.Close()

	if metadata.ChecksumSha256 != "" && checksum != metadata.ChecksumSha256 {
		_ = os.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()
		return nil, status.Error(codes.DataLoss, "checksum mismatch")
	}

	if metadata.TotalSize > 0 && totalWritten != metadata.TotalSize {
		_ = os.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()
		return nil, status.Error(codes.DataLoss, "incomplete upload")
	}

	if err := os.Rename(tempPath, safePath); err != nil {
		_ = os.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()
		return nil, status.Error(codes.Internal, "failed to finalize file")
	}

	s.unregisterTransfer(metadata.TransferId)
	cancel()

	s.logger.Info("file upload completed",
		"transfer_id", metadata.TransferId,
		"path", safePath,
		"bytes_written", totalWritten,
		"checksum", checksum,
	)

	return &proto.UploadResult{
		Success:        true,
		BytesWritten:   totalWritten,
		ChecksumSha256: checksum,
	}, nil
}

func (s *Service) receiveChunks(
	ctx context.Context,
	stream proto.FileTransferService_UploadFileServer,
	file *os.File,
	metadata *proto.UploadMetadata,
	firstChunkData []byte,
) (int64, string, error) {
	hasher := sha256.New()
	var totalWritten int64

	if len(firstChunkData) > 0 {
		n, err := file.Write(firstChunkData)
		if err != nil {
			return 0, "", status.Error(codes.Internal, "failed to write data")
		}
		hasher.Write(firstChunkData)
		totalWritten += int64(n)
		s.updateTransferProgress(metadata.TransferId, totalWritten)
	}

	for {
		select {
		case <-ctx.Done():
			return totalWritten, "", status.Error(codes.Canceled, "transfer canceled")
		default:
		}

		chunk, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			s.logger.Error("failed to receive chunk",
				"transfer_id", metadata.TransferId,
				"error", err,
			)
			return totalWritten, "", status.Error(codes.Internal, "failed to receive chunk")
		}

		if len(chunk.Data) == 0 {
			continue
		}

		n, err := file.Write(chunk.Data)
		if err != nil {
			return totalWritten, "", status.Error(codes.Internal, "failed to write data")
		}
		hasher.Write(chunk.Data)
		totalWritten += int64(n)
		s.updateTransferProgress(metadata.TransferId, totalWritten)
	}

	return totalWritten, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *Service) DownloadFile(req *proto.DownloadRequest, stream proto.FileTransferService_DownloadFileServer) error {
	safePath, err := s.safePath(req.Path)
	if err != nil {
		return status.Error(codes.InvalidArgument, "invalid path")
	}

	file, err := os.Open(safePath)
	if err != nil {
		if os.IsNotExist(err) {
			return status.Error(codes.NotFound, "file not found")
		}
		return status.Error(codes.Internal, "failed to open file")
	}
	defer func() { _ = file.Close() }()

	stat, err := file.Stat()
	if err != nil {
		return status.Error(codes.Internal, "failed to stat file")
	}

	if stat.IsDir() {
		return status.Error(codes.InvalidArgument, "path is a directory")
	}

	totalSize := stat.Size()

	if req.Offset > 0 {
		if _, err := file.Seek(req.Offset, io.SeekStart); err != nil {
			return status.Error(codes.Internal, "failed to seek")
		}
	}

	var remaining int64 = totalSize - req.Offset
	if req.Length > 0 && req.Length < remaining {
		remaining = req.Length
	}

	hasher := sha256.New()
	buf := make([]byte, defaultChunkSize)
	var offset int64 = req.Offset

	for remaining > 0 {
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "transfer canceled")
		default:
		}

		toRead := int64(len(buf))
		if toRead > remaining {
			toRead = remaining
		}

		n, err := file.Read(buf[:toRead])
		if err != nil && err != io.EOF {
			return status.Error(codes.Internal, "failed to read file")
		}

		if n == 0 {
			break
		}

		hasher.Write(buf[:n])
		isFinal := remaining <= int64(n)

		chunk := &proto.DownloadChunk{
			Data:      buf[:n],
			Offset:    offset,
			TotalSize: totalSize,
			IsFinal:   isFinal,
		}

		if isFinal {
			chunk.ChecksumSha256 = hex.EncodeToString(hasher.Sum(nil))
		}

		if err := stream.Send(chunk); err != nil {
			return status.Error(codes.Internal, "failed to send chunk")
		}

		offset += int64(n)
		remaining -= int64(n)
	}

	return nil
}

func (s *Service) FileOperation(ctx context.Context, req *proto.FileOperationRequest) (*proto.FileOperationResponse, error) {
	safePath, err := s.safePath(req.Path)
	if err != nil {
		return &proto.FileOperationResponse{
			Success: false,
			Error:   "invalid path",
		}, nil
	}

	switch req.Operation {
	case proto.FileOperationType_FILE_OPERATION_TYPE_DELETE:
		return s.handleDelete(safePath, req.Recursive)

	case proto.FileOperationType_FILE_OPERATION_TYPE_MOVE:
		destPath, err := s.safePath(req.Destination)
		if err != nil {
			return &proto.FileOperationResponse{Success: false, Error: "invalid destination path"}, nil
		}
		return s.handleMove(safePath, destPath)

	case proto.FileOperationType_FILE_OPERATION_TYPE_COPY:
		destPath, err := s.safePath(req.Destination)
		if err != nil {
			return &proto.FileOperationResponse{Success: false, Error: "invalid destination path"}, nil
		}
		return s.handleCopy(safePath, destPath, req.Recursive)

	case proto.FileOperationType_FILE_OPERATION_TYPE_CHMOD:
		return s.handleChmod(safePath, req.Mode)

	case proto.FileOperationType_FILE_OPERATION_TYPE_MKDIR:
		return s.handleMkdir(safePath, req.Mode, req.Recursive)

	case proto.FileOperationType_FILE_OPERATION_TYPE_TOUCH:
		return s.handleTouch(safePath)

	case proto.FileOperationType_FILE_OPERATION_TYPE_STAT:
		return s.handleStat(safePath)

	case proto.FileOperationType_FILE_OPERATION_TYPE_EXISTS:
		return s.handleExists(safePath)

	default:
		return &proto.FileOperationResponse{Success: false, Error: "unsupported operation"}, nil
	}
}

func (s *Service) ListDirectory(ctx context.Context, req *proto.ListDirectoryRequest) (*proto.ListDirectoryResponse, error) {
	safePath, err := s.safePath(req.Path)
	if err != nil {
		return &proto.ListDirectoryResponse{
			Success: false,
			Error:   "invalid path",
		}, nil
	}

	var files []*proto.FileStat
	var totalCount int32

	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if path == safePath {
			return nil
		}

		if !req.Recursive && filepath.Dir(path) != safePath {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if req.Pattern != "" {
			matched, _ := filepath.Match(req.Pattern, info.Name())
			if !matched {
				return nil
			}
		}

		totalCount++

		if req.Limit > 0 && int32(len(files)) >= req.Limit {
			return nil
		}

		if req.Offset > 0 && totalCount <= req.Offset {
			return nil
		}

		relPath, _ := filepath.Rel(safePath, path)

		files = append(files, &proto.FileStat{
			Name:       info.Name(),
			Path:       relPath,
			Size:       info.Size(),
			Mode:       int32(info.Mode()),
			ModifiedAt: timestamppb.New(info.ModTime()),
			IsDir:      info.IsDir(),
		})

		return nil
	}

	if err := filepath.Walk(safePath, walkFn); err != nil {
		return &proto.ListDirectoryResponse{
			Success: false,
			Error:   errors.Wrap(err, "failed to list directory").Error(),
		}, nil
	}

	return &proto.ListDirectoryResponse{
		Success:    true,
		Files:      files,
		TotalCount: totalCount,
	}, nil
}

func (s *Service) safePath(path string) (string, error) {
	if s.basePath == "" {
		return filepath.Clean(path), nil
	}

	cleanPath := filepath.Clean(filepath.Join(s.basePath, path))

	if !filepath.HasPrefix(cleanPath, s.basePath) {
		return "", errors.New("path traversal attempt")
	}

	return cleanPath, nil
}

func (s *Service) registerTransfer(transferID, path string, totalSize int64, cancel context.CancelFunc) {
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

func (s *Service) CancelAll() {
	s.mu.Lock()
	for _, transfer := range s.activeTransfers {
		if transfer.Cancel != nil {
			transfer.Cancel()
		}
	}
	s.mu.Unlock()
}

func (s *Service) handleDelete(path string, recursive bool) (*proto.FileOperationResponse, error) {
	var err error
	if recursive {
		err = os.RemoveAll(path)
	} else {
		err = os.Remove(path)
	}

	if err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleMove(src, dst string) (*proto.FileOperationResponse, error) {
	if err := os.Rename(src, dst); err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}
	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleCopy(src, dst string, recursive bool) (*proto.FileOperationResponse, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}

	if srcInfo.IsDir() {
		if !recursive {
			return &proto.FileOperationResponse{Success: false, Error: "source is directory, use recursive"}, nil
		}
		if err := copyDir(src, dst); err != nil {
			return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
		}
	} else {
		if err := copyFile(src, dst); err != nil {
			return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
		}
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleChmod(path string, mode int32) (*proto.FileOperationResponse, error) {
	if err := os.Chmod(path, os.FileMode(mode)); err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}
	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleMkdir(path string, mode int32, recursive bool) (*proto.FileOperationResponse, error) {
	perm := os.FileMode(mode)
	if perm == 0 {
		perm = 0755
	}

	var err error
	if recursive {
		err = os.MkdirAll(path, perm)
	} else {
		err = os.Mkdir(path, perm)
	}

	if err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}
	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleTouch(path string) (*proto.FileOperationResponse, error) {
	now := time.Now()
	if err := os.Chtimes(path, now, now); err != nil {
		if os.IsNotExist(err) {
			file, err := os.Create(path)
			if err != nil {
				return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
			}
			_ = file.Close()
			return &proto.FileOperationResponse{Success: true}, nil
		}
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}
	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleStat(path string) (*proto.FileOperationResponse, error) {
	info, err := os.Stat(path)
	if err != nil {
		return &proto.FileOperationResponse{Success: false, Error: err.Error()}, nil
	}

	stat := &proto.FileStat{
		Name:       info.Name(),
		Path:       path,
		Size:       info.Size(),
		Mode:       int32(info.Mode()),
		ModifiedAt: timestamppb.New(info.ModTime()),
		IsDir:      info.IsDir(),
	}

	if info.Mode()&os.ModeSymlink != 0 {
		stat.IsSymlink = true
		target, err := os.Readlink(path)
		if err == nil {
			stat.SymlinkTarget = target
		}
	}

	return &proto.FileOperationResponse{Success: true, Stat: stat}, nil
}

func (s *Service) handleExists(path string) (*proto.FileOperationResponse, error) {
	_, err := os.Stat(path)
	exists := err == nil || !os.IsNotExist(err)
	return &proto.FileOperationResponse{Success: true, Exists: exists}, nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer func() { _ = dstFile.Close() }()

	_, err = io.Copy(dstFile, srcFile)
	return err
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}
