package daemon

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const (
	smallFileThreshold = 1 * 1024 * 1024 // 1MB
	chunkSize          = 64 * 1024       // 64KB
	transferPrefix     = "transfers/"
)

type daemonNotConnectedError struct{}

func (e *daemonNotConnectedError) Error() string   { return "daemon not connected" }
func (e *daemonNotConnectedError) HTTPStatus() int { return 502 }

var ErrDaemonNotConnected error = &daemonNotConnectedError{}

type FileService struct {
	gateway    FileGateway
	registry   ConnectionChecker
	dispatcher FileDispatcher
	storage    files.StreamFileManager
	logger     *slog.Logger
}

func NewFileService(
	gateway FileGateway,
	registry ConnectionChecker,
	dispatcher FileDispatcher,
	storage files.StreamFileManager,
	logger *slog.Logger,
) *FileService {
	if logger == nil {
		logger = slog.Default()
	}

	return &FileService{
		gateway:    gateway,
		registry:   registry,
		dispatcher: dispatcher,
		storage:    storage,
		logger:     logger,
	}
}

func (s *FileService) resolveRoute(nodeID uint64) (local bool, err error) {
	if s.registry.IsConnected(nodeID) {
		return true, nil
	}

	if !s.registry.IsConnectedAnywhere(nodeID) {
		return false, ErrDaemonNotConnected
	}

	return false, nil
}

func (s *FileService) ReadDir(
	ctx context.Context,
	node *domain.Node,
	directory string,
) ([]*FileInfo, error) {
	nodeID := uint64(node.ID)
	relDir := stripWorkPath(node.WorkPath, directory)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return nil, err
	}

	var resp *proto.FileListResponse
	if local {
		resp, err = s.gateway.RequestFileList(ctx, nodeID, relDir, false, "")
	} else {
		resp, err = s.dispatcher.DispatchFileList(ctx, nodeID, relDir, false, "")
	}
	if err != nil {
		return nil, errors.WithMessage(err, "file list request")
	}

	if !resp.Success {
		return nil, errors.Errorf("file list failed: %s", resp.Error)
	}

	result := make([]*FileInfo, 0, len(resp.Files))
	for _, f := range resp.Files {
		result = append(result, protoFileStatToFileInfo(f))
	}

	return result, nil
}

func (s *FileService) Download(ctx context.Context, node *domain.Node, filePath string) ([]byte, error) {
	nodeID := uint64(node.ID)
	relPath := stripWorkPath(node.WorkPath, filePath)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return nil, err
	}

	if local {
		resp, readErr := s.gateway.RequestFileRead(ctx, nodeID, relPath, 0, 0)
		if readErr != nil {
			return nil, errors.WithMessage(readErr, "file read request")
		}

		if !resp.Success {
			return nil, errors.Errorf("file read failed: %s", resp.Error)
		}

		return resp.Content, nil
	}

	result, err := s.dispatcher.DispatchFileRead(ctx, nodeID, relPath, 0, 0)
	if err != nil {
		return nil, errors.WithMessage(err, "dispatched file read")
	}

	if result.Content != nil {
		return result.Content, nil
	}

	data, err := s.storage.Read(ctx, result.StoragePath)
	if err != nil {
		return nil, errors.WithMessage(err, "reading transferred file from storage")
	}
	_ = s.storage.Delete(context.Background(), result.StoragePath)

	return data, nil
}

func (s *FileService) DownloadStream(
	ctx context.Context,
	node *domain.Node,
	filePath string,
) (io.ReadCloser, error) {
	nodeID := uint64(node.ID)
	relPath := stripWorkPath(node.WorkPath, filePath)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return nil, err
	}

	if local {
		return s.downloadStreamLocal(ctx, nodeID, relPath)
	}

	return s.downloadStreamRemote(ctx, nodeID, relPath)
}

func (s *FileService) downloadStreamLocal(ctx context.Context, nodeID uint64, path string) (io.ReadCloser, error) {
	pr, pw := io.Pipe()

	go func() {
		defer pw.Close()

		var offset int64

		for {
			select {
			case <-ctx.Done():
				pw.CloseWithError(ctx.Err())

				return
			default:
			}

			resp, err := s.gateway.RequestFileRead(ctx, nodeID, path, offset, int64(chunkSize))
			if err != nil {
				pw.CloseWithError(errors.WithMessage(err, "gateway chunked read"))

				return
			}

			if !resp.Success {
				pw.CloseWithError(errors.Errorf("gateway chunked read: %s", resp.Error))

				return
			}

			if len(resp.Content) == 0 {
				return
			}

			if _, err := pw.Write(resp.Content); err != nil {
				return
			}

			offset += int64(len(resp.Content))

			if int64(len(resp.Content)) < int64(chunkSize) {
				return
			}
		}
	}()

	return pr, nil
}

func (s *FileService) downloadStreamRemote(ctx context.Context, nodeID uint64, path string) (io.ReadCloser, error) {
	transferID := generateTransferID()

	if err := s.dispatcher.DispatchDownloadTask(ctx, nodeID, transferID, path); err != nil {
		return nil, errors.WithMessage(err, "dispatched download task")
	}

	storagePath := transferPrefix + transferID + "/data"

	reader, err := s.storage.ReadStream(ctx, storagePath)
	if err != nil {
		return nil, errors.WithMessage(err, "reading transferred file from storage")
	}

	return &cleanupReadCloser{
		ReadCloser:  reader,
		storagePath: storagePath,
		storage:     s.storage,
	}, nil
}

func (s *FileService) Upload(
	ctx context.Context,
	node *domain.Node,
	filePath string,
	content []byte,
	perms os.FileMode,
) error {
	nodeID := uint64(node.ID)
	relPath := stripWorkPath(node.WorkPath, filePath)
	mode := int32(perms & 0x1FF)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return err
	}

	if local {
		return s.gateway.RequestFileWrite(ctx, nodeID, relPath, content, mode, true)
	}

	return s.dispatcher.DispatchFileWrite(ctx, nodeID, relPath, content, mode, true)
}

func (s *FileService) UploadStream(
	ctx context.Context,
	node *domain.Node,
	filePath string,
	r io.Reader,
	size uint64,
	perms os.FileMode,
) error {
	nodeID := uint64(node.ID)
	relPath := stripWorkPath(node.WorkPath, filePath)
	mode := int32(perms & 0x1FF)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return err
	}

	if local && size > 0 && size <= uint64(smallFileThreshold) {
		content, readErr := io.ReadAll(r)
		if readErr != nil {
			return errors.Wrap(readErr, "reading upload content")
		}

		return s.gateway.RequestFileWrite(ctx, nodeID, relPath, content, mode, true)
	}

	transferID := generateTransferID()
	storagePath := transferPrefix + transferID + "/data"

	hasher := sha256.New()
	teeReader := io.TeeReader(r, hasher)

	if err := s.storage.WriteStream(ctx, storagePath, teeReader); err != nil {
		return errors.Wrap(err, "writing upload to storage")
	}

	checksum := hex.EncodeToString(hasher.Sum(nil))
	_ = checksum

	if local {
		if err := s.dispatcher.DispatchUploadTask(ctx, nodeID, transferID, relPath); err != nil {
			_ = s.storage.Delete(context.Background(), storagePath)

			return errors.WithMessage(err, "upload task")
		}

		return nil
	}

	if err := s.dispatcher.DispatchUploadTask(ctx, nodeID, transferID, relPath); err != nil {
		_ = s.storage.Delete(context.Background(), storagePath)

		return errors.WithMessage(err, "dispatched upload task")
	}

	return nil
}

func (s *FileService) MkDir(ctx context.Context, node *domain.Node, directory string) error {
	return s.doFileOperation(ctx, node, &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_MKDIR,
		Parameters: &proto.FileOperationRequest_MkdirParams{
			MkdirParams: &proto.MkdirParams{
				Path:      stripWorkPath(node.WorkPath, directory),
				Recursive: true,
			},
		},
	})
}

func (s *FileService) Copy(ctx context.Context, node *domain.Node, source, destination string) error {
	return s.doFileOperation(ctx, node, &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_COPY,
		Parameters: &proto.FileOperationRequest_CopyParams{
			CopyParams: &proto.CopyParams{
				Source:      stripWorkPath(node.WorkPath, source),
				Destination: stripWorkPath(node.WorkPath, destination),
				Recursive:   true,
			},
		},
	})
}

func (s *FileService) Move(ctx context.Context, node *domain.Node, source, destination string) error {
	return s.doFileOperation(ctx, node, &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_MOVE,
		Parameters: &proto.FileOperationRequest_MoveParams{
			MoveParams: &proto.MoveParams{
				Source:      stripWorkPath(node.WorkPath, source),
				Destination: stripWorkPath(node.WorkPath, destination),
			},
		},
	})
}

func (s *FileService) Remove(ctx context.Context, node *domain.Node, path string, recursive bool) error {
	return s.doFileOperation(ctx, node, &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_DELETE,
		Parameters: &proto.FileOperationRequest_DeleteParams{
			DeleteParams: &proto.DeleteParams{
				Path:      stripWorkPath(node.WorkPath, path),
				Recursive: recursive,
			},
		},
	})
}

func (s *FileService) GetFileInfo(
	ctx context.Context,
	node *domain.Node,
	path string,
) (*FileDetails, error) {
	nodeID := uint64(node.ID)
	relPath := stripWorkPath(node.WorkPath, path)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return nil, err
	}

	req := &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_STAT,
		Parameters: &proto.FileOperationRequest_StatParams{
			StatParams: &proto.StatParams{
				Path: relPath,
			},
		},
	}

	var resp *proto.FileOperationResponse
	if local {
		resp, err = s.gateway.RequestFileOperation(ctx, nodeID, req)
	} else {
		resp, err = s.dispatcher.DispatchFileOperation(ctx, nodeID, req)
	}
	if err != nil {
		return nil, errors.WithMessage(err, "stat request")
	}

	if !resp.Success {
		return nil, errors.Errorf("stat failed: %s", resp.Error)
	}

	statResult := resp.GetStatResult()
	if statResult == nil {
		return nil, errors.New("stat returned no result")
	}

	return protoFileStatToDetails(statResult.Stat), nil
}

func (s *FileService) Chmod(ctx context.Context, node *domain.Node, path string, perm uint32) error {
	return s.doFileOperation(ctx, node, &proto.FileOperationRequest{
		Operation: proto.FileOperationType_FILE_OPERATION_TYPE_CHMOD,
		Parameters: &proto.FileOperationRequest_ChmodParams{
			ChmodParams: &proto.ChmodParams{
				Path: stripWorkPath(node.WorkPath, path),
				Mode: int32(perm & 0x1FF),
			},
		},
	})
}

func (s *FileService) doFileOperation(
	ctx context.Context,
	node *domain.Node,
	req *proto.FileOperationRequest,
) error {
	nodeID := uint64(node.ID)

	local, err := s.resolveRoute(nodeID)
	if err != nil {
		return err
	}

	var resp *proto.FileOperationResponse
	if local {
		resp, err = s.gateway.RequestFileOperation(ctx, nodeID, req)
	} else {
		resp, err = s.dispatcher.DispatchFileOperation(ctx, nodeID, req)
	}
	if err != nil {
		return errors.WithMessage(err, "file operation")
	}

	if !resp.Success {
		return errors.Errorf("file operation failed: %s", resp.Error)
	}

	return nil
}

type cleanupReadCloser struct {
	io.ReadCloser

	storagePath string
	storage     files.StreamFileManager
}

func (c *cleanupReadCloser) Close() error {
	err := c.ReadCloser.Close()
	_ = c.storage.Delete(context.Background(), c.storagePath)

	return err
}

func protoFileStatToFileInfo(f *proto.FileStat) *FileInfo {
	if f == nil {
		return nil
	}

	var modTime uint64
	if f.ModifiedAt != nil {
		ts := f.ModifiedAt.AsTime().Unix()
		if ts > 0 {
			modTime = uint64(ts)
		}
	}

	return &FileInfo{
		Name:         f.Name,
		Size:         f.Size,
		TimeModified: modTime,
		Type:         protoFileTypeToFileType(f.Type),
		Perm:         f.Mode,
	}
}

func protoFileTypeToFileType(t proto.FileType) FileType {
	switch t {
	case proto.FileType_FILE_TYPE_DIRECTORY:
		return FileTypeDir
	case proto.FileType_FILE_TYPE_REGULAR:
		return FileTypeFile
	case proto.FileType_FILE_TYPE_SYMLINK:
		return FileTypeSymlink
	case proto.FileType_FILE_TYPE_SOCKET:
		return FileTypeSocket
	case proto.FileType_FILE_TYPE_FIFO:
		return FileTypeNamedPipe
	case proto.FileType_FILE_TYPE_BLOCK_DEVICE:
		return FileTypeBlockDevice
	case proto.FileType_FILE_TYPE_CHAR_DEVICE:
		return FileTypeDevice
	default:
		return FileTypeUnknown
	}
}

func protoFileStatToDetails(s *proto.FileStat) *FileDetails {
	if s == nil {
		return &FileDetails{}
	}

	var modTime, accessTime uint64
	if s.ModifiedAt != nil {
		ts := s.ModifiedAt.AsTime().Unix()
		if ts > 0 {
			modTime = uint64(ts)
		}
	}
	if s.AccessedAt != nil {
		ts := s.AccessedAt.AsTime().Unix()
		if ts > 0 {
			accessTime = uint64(ts)
		}
	}

	return &FileDetails{
		Name:             s.Name,
		Size:             s.Size,
		ModificationTime: modTime,
		AccessTime:       accessTime,
		Perm:             s.Mode,
		Type:             protoFileTypeToFileType(s.Type),
	}
}

func stripWorkPath(workPath, fullPath string) string {
	rel := strings.TrimPrefix(fullPath, workPath)
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		return "."
	}

	return rel
}

func generateTransferID() string {
	return generateRequestID()
}
