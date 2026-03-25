package filetransfer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
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

	root   *os.Root
	logger *slog.Logger

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

	root, err := os.OpenRoot(basePath)
	if err != nil {
		panic(fmt.Sprintf("failed to open root directory %q: %v", basePath, err))
	}

	return &Service{
		root:            root,
		logger:          logger,
		activeTransfers: make(map[string]*activeTransfer),
	}
}

func (s *Service) Close() error {
	return s.root.Close()
}

func (s *Service) CleanupStaleTempFiles(ctx context.Context) error {
	var cleaned int

	err := fs.WalkDir(s.root.FS(), ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip entries with walk errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if d.IsDir() {
			return nil
		}

		if !strings.HasPrefix(d.Name(), tempFilePrefix) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // skip entries we can't stat
		}

		if time.Since(info.ModTime()) > tempFileMaxAge {
			if err := s.root.Remove(path); err != nil {
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

	path := metadata.Path
	dir := filepath.Dir(path)

	if metadata.CreateDirs {
		if err := s.root.MkdirAll(dir, 0755); err != nil {
			return status.Error(codes.Internal, "failed to create directories")
		}
	}

	result, err := s.processUpload(ctx, stream, metadata, path, dir, firstChunk.Data)
	if err != nil {
		return err
	}

	return stream.SendAndClose(result)
}

func (s *Service) processUpload(
	ctx context.Context,
	stream proto.FileTransferService_UploadFileServer,
	metadata *proto.UploadMetadata,
	path, dir string,
	firstChunkData []byte,
) (*proto.UploadResult, error) {
	tempPath := filepath.Join(dir, tempFilePrefix+metadata.TransferId)

	flags := os.O_CREATE | os.O_WRONLY
	if metadata.Offset == 0 {
		flags |= os.O_TRUNC
	}

	file, err := s.root.OpenFile(tempPath, flags, safeFileMode(metadata.Mode))
	if err != nil {
		s.logger.Error("failed to open temp file for writing",
			"path", tempPath,
			"error", err,
		)

		return nil, status.Error(codes.Internal, "failed to create file")
	}

	if metadata.Offset > 0 {
		if _, err := file.Seek(metadata.Offset, io.SeekStart); err != nil {
			_ = file.Close()

			return nil, status.Error(codes.Internal, "failed to seek to offset")
		}
	}

	transferCtx, cancel := context.WithCancel(ctx)
	s.registerTransfer(metadata.TransferId, path, metadata.TotalSize, cancel)

	cleanup := func() {
		_ = file.Close()
		_ = s.root.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()
	}

	totalWritten, checksum, err := s.receiveChunks(transferCtx, stream, file, metadata, firstChunkData)
	if err != nil {
		cleanup()

		return nil, err
	}

	_ = file.Close()

	if metadata.Offset == 0 && metadata.ChecksumSha256 != "" && checksum != metadata.ChecksumSha256 {
		_ = s.root.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()

		return nil, status.Error(codes.DataLoss, "checksum mismatch")
	}

	finalSize := metadata.Offset + totalWritten
	if metadata.TotalSize > 0 && finalSize != metadata.TotalSize {
		_ = s.root.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()

		return nil, status.Error(codes.DataLoss, "incomplete upload")
	}

	if err := s.root.Rename(tempPath, path); err != nil {
		_ = s.root.Remove(tempPath)
		s.unregisterTransfer(metadata.TransferId)
		cancel()

		return nil, status.Error(codes.Internal, "failed to finalize file")
	}

	s.unregisterTransfer(metadata.TransferId)
	cancel()

	s.logger.Info("file upload completed",
		"transfer_id", metadata.TransferId,
		"path", path,
		"bytes_written", finalSize,
		"checksum", checksum,
	)

	return &proto.UploadResult{
		Success:        true,
		BytesWritten:   finalSize,
		ChecksumSha256: checksum,
	}, nil
}

type recvResult struct {
	chunk *proto.UploadChunk
	err   error
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
		s.updateTransferProgress(metadata.TransferId, metadata.Offset+totalWritten)
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
			return totalWritten, "", status.Error(codes.Canceled, "transfer canceled")
		case <-time.After(chunkReceiveTimeout):
			return totalWritten, "", status.Error(codes.DeadlineExceeded, "chunk receive timeout")
		case res = <-ch:
		}

		if errors.Is(res.err, io.EOF) {
			break
		}
		if res.err != nil {
			s.logger.Error("failed to receive chunk",
				"transfer_id", metadata.TransferId,
				"error", res.err,
			)

			return totalWritten, "", status.Error(codes.Internal, "failed to receive chunk")
		}

		if len(res.chunk.Data) == 0 {
			continue
		}

		n, err := file.Write(res.chunk.Data)
		if err != nil {
			return totalWritten, "", status.Error(codes.Internal, "failed to write data")
		}
		hasher.Write(res.chunk.Data)
		totalWritten += int64(n)
		s.updateTransferProgress(metadata.TransferId, metadata.Offset+totalWritten)
	}

	return totalWritten, hex.EncodeToString(hasher.Sum(nil)), nil
}

func (s *Service) DownloadFile(
	req *proto.DownloadRequest,
	stream proto.FileTransferService_DownloadFileServer,
) error {
	file, err := s.root.Open(req.Path)
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

	remaining := totalSize - req.Offset
	if req.Length > 0 && req.Length < remaining {
		remaining = req.Length
	}

	hasher := sha256.New()
	buf := make([]byte, defaultChunkSize)
	offset := req.Offset

	for remaining > 0 {
		select {
		case <-stream.Context().Done():
			return status.Error(codes.Canceled, "transfer canceled")
		default:
		}

		toRead := min(int64(len(buf)), remaining)

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

func (s *Service) FileOperation(
	_ context.Context,
	req *proto.FileOperationRequest,
) (*proto.FileOperationResponse, error) {
	errResp := func(msg string) (*proto.FileOperationResponse, error) {
		return &proto.FileOperationResponse{
			Success:   false,
			Error:     msg,
			RequestId: req.RequestId,
		}, nil
	}

	switch p := req.Parameters.(type) {
	case *proto.FileOperationRequest_DeleteParams:
		return s.handleDelete(p.DeleteParams.Path, p.DeleteParams.Recursive)

	case *proto.FileOperationRequest_MoveParams:
		return s.handleMove(p.MoveParams.Source, p.MoveParams.Destination)

	case *proto.FileOperationRequest_CopyParams:
		return s.handleCopy(
			p.CopyParams.Source,
			p.CopyParams.Destination,
			p.CopyParams.Recursive,
		)

	case *proto.FileOperationRequest_ChmodParams:
		return s.handleChmod(p.ChmodParams.Path, p.ChmodParams.Mode)

	case *proto.FileOperationRequest_ChownParams:
		return errResp("chown is not supported")

	case *proto.FileOperationRequest_MkdirParams:
		return s.handleMkdir(
			p.MkdirParams.Path,
			p.MkdirParams.Mode,
			p.MkdirParams.Recursive,
		)

	case *proto.FileOperationRequest_TouchParams:
		return s.handleTouch(p.TouchParams.Path)

	case *proto.FileOperationRequest_StatParams:
		return s.handleStat(p.StatParams.Path)

	case *proto.FileOperationRequest_ExistsParams:
		return s.handleExists(p.ExistsParams.Path)

	default:
		return errResp("unsupported operation")
	}
}

func (s *Service) ListDirectory(
	ctx context.Context,
	req *proto.ListDirectoryRequest,
) (*proto.ListDirectoryResponse, error) {
	path := req.Path
	if path == "" {
		path = "."
	}

	if !req.Recursive {
		return s.listDirectoryFlat(path, req)
	}

	return s.listDirectoryRecursive(ctx, path, req)
}

func (s *Service) listDirectoryFlat(
	dirPath string,
	req *proto.ListDirectoryRequest,
) (*proto.ListDirectoryResponse, error) {
	dir, err := s.root.Open(dirPath)
	if err != nil {
		return &proto.ListDirectoryResponse{
			Success: false,
			Error:   errors.Wrap(err, "failed to open directory").Error(),
		}, nil
	}
	defer func() { _ = dir.Close() }()

	entries, err := dir.ReadDir(-1)
	if err != nil {
		return &proto.ListDirectoryResponse{
			Success: false,
			Error:   errors.Wrap(err, "failed to read directory").Error(),
		}, nil
	}

	var files []*proto.FileStat
	var totalCount int32

	for _, entry := range entries {
		if req.Pattern != "" {
			matched, _ := filepath.Match(req.Pattern, entry.Name())
			if !matched {
				continue
			}
		}

		totalCount++

		if req.Offset > 0 && totalCount <= req.Offset {
			continue
		}

		if req.Limit > 0 && len(files) >= int(req.Limit) {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		files = append(files, makeFileStat(entry.Name(), info))
	}

	return &proto.ListDirectoryResponse{
		Success:    true,
		Files:      files,
		TotalCount: totalCount,
	}, nil
}

func (s *Service) listDirectoryRecursive(
	ctx context.Context,
	dirPath string,
	req *proto.ListDirectoryRequest,
) (*proto.ListDirectoryResponse, error) {
	var files []*proto.FileStat
	var totalCount int32

	err := fs.WalkDir(s.root.FS(), dirPath, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return nil //nolint:nilerr // skip entries with walk errors
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if path == dirPath {
			return nil
		}

		if req.Pattern != "" {
			matched, _ := filepath.Match(req.Pattern, d.Name())
			if !matched {
				return nil
			}
		}

		totalCount++

		if req.Offset > 0 && totalCount <= req.Offset {
			return nil
		}

		if req.Limit > 0 && len(files) >= int(req.Limit) {
			return nil
		}

		info, infoErr := d.Info()
		if infoErr != nil {
			return nil //nolint:nilerr // skip entries we can't stat
		}

		relPath, _ := filepath.Rel(dirPath, path)

		files = append(files, makeFileStat(relPath, info))

		return nil
	})

	if err != nil {
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

func (s *Service) handleDelete(
	path string,
	recursive bool,
) (*proto.FileOperationResponse, error) {
	var err error
	if recursive {
		err = s.root.RemoveAll(path)
	} else {
		err = s.root.Remove(path)
	}

	if err != nil {
		return opErr(err), nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleMove(
	src, dst string,
) (*proto.FileOperationResponse, error) {
	if err := s.root.Rename(src, dst); err != nil {
		return opErr(err), nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleCopy(
	src, dst string,
	recursive bool,
) (*proto.FileOperationResponse, error) {
	srcInfo, err := s.root.Stat(src)
	if err != nil {
		return opErr(err), nil
	}

	if srcInfo.IsDir() {
		if !recursive {
			return opErr(errors.New("source is directory, use recursive")), nil
		}
		if err := s.copyDir(src, dst); err != nil {
			return opErr(err), nil
		}
	} else {
		if err := s.copyFile(src, dst); err != nil {
			return opErr(err), nil
		}
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleChmod(
	path string,
	mode int32,
) (*proto.FileOperationResponse, error) {
	if err := s.root.Chmod(path, safeFileMode(mode)); err != nil {
		return opErr(err), nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleMkdir(
	path string,
	mode int32,
	recursive bool,
) (*proto.FileOperationResponse, error) {
	perm := safeFileMode(mode)
	if perm == 0 {
		perm = 0755
	}

	var err error
	if recursive {
		err = s.root.MkdirAll(path, perm)
	} else {
		err = s.root.Mkdir(path, perm)
	}

	if err != nil {
		return opErr(err), nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleTouch(path string) (*proto.FileOperationResponse, error) {
	now := time.Now()
	if err := s.root.Chtimes(path, now, now); err != nil {
		if os.IsNotExist(err) {
			file, createErr := s.root.Create(path)
			if createErr != nil {
				return opErr(createErr), nil
			}
			_ = file.Close()

			return &proto.FileOperationResponse{Success: true}, nil
		}

		return opErr(err), nil
	}

	return &proto.FileOperationResponse{Success: true}, nil
}

func (s *Service) handleStat(path string) (*proto.FileOperationResponse, error) {
	info, err := s.root.Lstat(path)
	if err != nil {
		return opErr(err), nil
	}

	stat := makeFileStat(path, info)

	if info.Mode()&os.ModeSymlink != 0 {
		target, readlinkErr := s.root.Readlink(path)
		if readlinkErr == nil {
			stat.SymlinkTarget = target
		}
	}

	return &proto.FileOperationResponse{
		Success: true,
		Result: &proto.FileOperationResponse_StatResult{
			StatResult: &proto.StatResult{Stat: stat},
		},
	}, nil
}

func (s *Service) handleExists(path string) (*proto.FileOperationResponse, error) {
	_, err := s.root.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &proto.FileOperationResponse{
				Success: true,
				Result: &proto.FileOperationResponse_ExistsResult{
					ExistsResult: &proto.ExistsResult{Exists: false},
				},
			}, nil
		}

		return opErr(err), nil
	}

	return &proto.FileOperationResponse{
		Success: true,
		Result: &proto.FileOperationResponse_ExistsResult{
			ExistsResult: &proto.ExistsResult{Exists: true},
		},
	}, nil
}

func (s *Service) copyFile(src, dst string) error {
	srcFile, err := s.root.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = srcFile.Close() }()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	tmpDst := dst + ".tmp"
	dstFile, err := s.root.OpenFile(tmpDst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		_ = dstFile.Close()
		_ = s.root.Remove(tmpDst)

		return err
	}

	if err := dstFile.Sync(); err != nil {
		_ = dstFile.Close()
		_ = s.root.Remove(tmpDst)

		return err
	}

	if err := dstFile.Close(); err != nil {
		_ = s.root.Remove(tmpDst)

		return err
	}

	return s.root.Rename(tmpDst, dst)
}

func (s *Service) copyDir(src, dst string) error {
	return fs.WalkDir(s.root.FS(), src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(dst, relPath)

		if d.IsDir() {
			info, err := d.Info()
			if err != nil {
				return err
			}

			return s.root.MkdirAll(dstPath, info.Mode())
		}

		return s.copyFile(path, dstPath)
	})
}

func opErr(err error) *proto.FileOperationResponse {
	return &proto.FileOperationResponse{Success: false, Error: err.Error()}
}

func makeFileStat(path string, info os.FileInfo) *proto.FileStat {
	return &proto.FileStat{
		Name:       info.Name(),
		Path:       path,
		Size:       uint64(max(0, info.Size())),
		Mode:       uint32(info.Mode()),
		ModifiedAt: timestamppb.New(info.ModTime()),
		Type:       fileTypeFromMode(info.Mode()),
	}
}

func safeFileMode(mode int32) os.FileMode {
	return os.FileMode(mode) & os.ModePerm //nolint:gosec // masked to permission bits
}

func fileTypeFromMode(m os.FileMode) proto.FileType {
	switch {
	case m.IsDir():
		return proto.FileType_FILE_TYPE_DIRECTORY
	case m&os.ModeSymlink != 0:
		return proto.FileType_FILE_TYPE_SYMLINK
	case m&os.ModeSocket != 0:
		return proto.FileType_FILE_TYPE_SOCKET
	case m&os.ModeNamedPipe != 0:
		return proto.FileType_FILE_TYPE_FIFO
	case m&os.ModeDevice != 0 && m&os.ModeCharDevice != 0:
		return proto.FileType_FILE_TYPE_CHAR_DEVICE
	case m&os.ModeDevice != 0:
		return proto.FileType_FILE_TYPE_BLOCK_DEVICE
	default:
		return proto.FileType_FILE_TYPE_REGULAR
	}
}
