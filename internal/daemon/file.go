package daemon

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

type FileGateway interface {
	RequestFileList(
		ctx context.Context, nodeID uint64, path string, recursive bool, pattern string,
	) (*proto.FileListResponse, error)
	RequestFileRead(
		ctx context.Context, nodeID uint64, path string, offset int64, length int64,
	) (*proto.FileReadResponse, error)
	RequestFileWrite(
		ctx context.Context, nodeID uint64, path string, content []byte, mode int32, createDirs bool,
	) error
	RequestFileOperation(
		ctx context.Context, nodeID uint64, req *proto.FileOperationRequest,
	) (*proto.FileOperationResponse, error)
}

type ConnectionChecker interface {
	IsConnected(nodeID uint64) bool
}

type FileService struct {
	gateway  FileGateway
	registry ConnectionChecker
	legacy   *FileBINNService
}

func NewFileService(
	gateway FileGateway,
	registry ConnectionChecker,
	legacy *FileBINNService,
) *FileService {
	return &FileService{
		gateway:  gateway,
		registry: registry,
		legacy:   legacy,
	}
}

func (s *FileService) ReadDir(
	ctx context.Context,
	node *domain.Node,
	directory string,
) ([]*FileInfo, error) {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		relDir := stripWorkPath(node.WorkPath, directory)
		resp, err := s.gateway.RequestFileList(ctx, nodeID, relDir, false, "")
		if err != nil {
			return nil, errors.WithMessage(err, "gRPC file list request failed")
		}

		if !resp.Success {
			return nil, errors.Errorf("gRPC file list failed: %s", resp.Error)
		}

		result := make([]*FileInfo, 0, len(resp.Files))
		for _, f := range resp.Files {
			result = append(result, protoFileInfoToDaemon(f))
		}

		return result, nil
	}

	return s.legacy.ReadDir(ctx, node, directory)
}

func (s *FileService) Download(ctx context.Context, node *domain.Node, filePath string) ([]byte, error) {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		relPath := stripWorkPath(node.WorkPath, filePath)
		resp, err := s.gateway.RequestFileRead(ctx, nodeID, relPath, 0, 0)
		if err != nil {
			return nil, errors.WithMessage(err, "gRPC file read request failed")
		}

		if !resp.Success {
			return nil, errors.Errorf("gRPC file read failed: %s", resp.Error)
		}

		return resp.Content, nil
	}

	return s.legacy.Download(ctx, node, filePath)
}

func (s *FileService) DownloadStream(
	ctx context.Context,
	node *domain.Node,
	filePath string,
) (io.ReadCloser, error) {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		relPath := stripWorkPath(node.WorkPath, filePath)
		resp, err := s.gateway.RequestFileRead(ctx, nodeID, relPath, 0, 0)
		if err != nil {
			return nil, errors.WithMessage(err, "gRPC file read request failed")
		}

		if !resp.Success {
			return nil, errors.Errorf("gRPC file read failed: %s", resp.Error)
		}

		return io.NopCloser(bytes.NewReader(resp.Content)), nil
	}

	return s.legacy.DownloadStream(ctx, node, filePath)
}

func (s *FileService) Upload(
	ctx context.Context,
	node *domain.Node,
	filePath string,
	content []byte,
	perms os.FileMode,
) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		relPath := stripWorkPath(node.WorkPath, filePath)
		mode := int32(perms & 0x1FF)

		return s.gateway.RequestFileWrite(ctx, nodeID, relPath, content, mode, true)
	}

	return s.legacy.Upload(ctx, node, filePath, content, perms)
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

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		content, err := io.ReadAll(r)
		if err != nil {
			return errors.WithMessage(err, "failed to read upload content")
		}

		relPath := stripWorkPath(node.WorkPath, filePath)
		mode := int32(perms & 0x1FF)

		return s.gateway.RequestFileWrite(ctx, nodeID, relPath, content, mode, true)
	}

	return s.legacy.UploadStream(ctx, node, filePath, r, size, perms)
}

func (s *FileService) MkDir(ctx context.Context, node *domain.Node, directory string) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		return s.doFileOperation(ctx, nodeID, &proto.FileOperationRequest{
			Operation: proto.FileOperationType_FILE_OPERATION_TYPE_MKDIR,
			Parameters: &proto.FileOperationRequest_MkdirParams{
				MkdirParams: &proto.MkdirParams{
					Path:      stripWorkPath(node.WorkPath, directory),
					Recursive: true,
				},
			},
		})
	}

	return s.legacy.MkDir(ctx, node, directory)
}

func (s *FileService) Copy(ctx context.Context, node *domain.Node, source, destination string) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		return s.doFileOperation(ctx, nodeID, &proto.FileOperationRequest{
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

	return s.legacy.Copy(ctx, node, source, destination)
}

func (s *FileService) Move(ctx context.Context, node *domain.Node, source, destination string) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		return s.doFileOperation(ctx, nodeID, &proto.FileOperationRequest{
			Operation: proto.FileOperationType_FILE_OPERATION_TYPE_MOVE,
			Parameters: &proto.FileOperationRequest_MoveParams{
				MoveParams: &proto.MoveParams{
					Source:      stripWorkPath(node.WorkPath, source),
					Destination: stripWorkPath(node.WorkPath, destination),
				},
			},
		})
	}

	return s.legacy.Move(ctx, node, source, destination)
}

func (s *FileService) Remove(ctx context.Context, node *domain.Node, path string, recursive bool) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		return s.doFileOperation(ctx, nodeID, &proto.FileOperationRequest{
			Operation: proto.FileOperationType_FILE_OPERATION_TYPE_DELETE,
			Parameters: &proto.FileOperationRequest_DeleteParams{
				DeleteParams: &proto.DeleteParams{
					Path:      stripWorkPath(node.WorkPath, path),
					Recursive: recursive,
				},
			},
		})
	}

	return s.legacy.Remove(ctx, node, path, recursive)
}

func (s *FileService) GetFileInfo(
	ctx context.Context,
	node *domain.Node,
	path string,
) (*FileDetails, error) {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		resp, err := s.gateway.RequestFileOperation(ctx, nodeID, &proto.FileOperationRequest{
			Operation: proto.FileOperationType_FILE_OPERATION_TYPE_STAT,
			Parameters: &proto.FileOperationRequest_StatParams{
				StatParams: &proto.StatParams{
					Path: stripWorkPath(node.WorkPath, path),
				},
			},
		})
		if err != nil {
			return nil, errors.WithMessage(err, "gRPC stat request failed")
		}

		if !resp.Success {
			return nil, errors.Errorf("gRPC stat failed: %s", resp.Error)
		}

		statResult := resp.GetStatResult()
		if statResult == nil {
			return nil, errors.New("gRPC stat returned no result")
		}

		return protoFileStatToDetails(statResult.Stat), nil
	}

	return s.legacy.GetFileInfo(ctx, node, path)
}

func (s *FileService) Chmod(ctx context.Context, node *domain.Node, path string, perm uint32) error {
	nodeID := uint64(node.ID)

	if s.gateway != nil && s.registry.IsConnected(nodeID) {
		return s.doFileOperation(ctx, nodeID, &proto.FileOperationRequest{
			Operation: proto.FileOperationType_FILE_OPERATION_TYPE_CHMOD,
			Parameters: &proto.FileOperationRequest_ChmodParams{
				ChmodParams: &proto.ChmodParams{
					Path: stripWorkPath(node.WorkPath, path),
					Mode: int32(perm & 0x1FF),
				},
			},
		})
	}

	return s.legacy.Chmod(ctx, node, path, perm)
}

func (s *FileService) doFileOperation(
	ctx context.Context,
	nodeID uint64,
	req *proto.FileOperationRequest,
) error {
	resp, err := s.gateway.RequestFileOperation(ctx, nodeID, req)
	if err != nil {
		return errors.WithMessage(err, "gRPC file operation failed")
	}

	if !resp.Success {
		return errors.Errorf("gRPC file operation failed: %s", resp.Error)
	}

	return nil
}

func protoFileInfoToDaemon(f *proto.FileInfo) *FileInfo {
	if f == nil {
		return nil
	}

	fileType := FileTypeFile
	if f.IsDir {
		fileType = FileTypeDir
	}

	var modTime uint64
	if f.ModifiedAt != nil {
		ts := f.ModifiedAt.AsTime().Unix()
		if ts > 0 {
			modTime = uint64(ts)
		}
	}

	var size uint64
	if f.Size > 0 {
		size = uint64(f.Size)
	}

	var perm uint32
	if f.Mode >= 0 {
		perm = uint32(f.Mode)
	}

	return &FileInfo{
		Name:         f.Name,
		Size:         size,
		TimeModified: modTime,
		Type:         fileType,
		Perm:         perm,
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

	fileType := FileTypeFile
	switch s.Type {
	case proto.FileType_FILE_TYPE_DIRECTORY:
		fileType = FileTypeDir
	case proto.FileType_FILE_TYPE_SYMLINK:
		fileType = FileTypeSymlink
	case proto.FileType_FILE_TYPE_SOCKET:
		fileType = FileTypeSocket
	case proto.FileType_FILE_TYPE_FIFO:
		fileType = FileTypeNamedPipe
	case proto.FileType_FILE_TYPE_BLOCK_DEVICE:
		fileType = FileTypeBlockDevice
	case proto.FileType_FILE_TYPE_CHAR_DEVICE:
		fileType = FileTypeDevice
	}

	return &FileDetails{
		Name:             s.Name,
		Size:             s.Size,
		ModificationTime: modTime,
		AccessTime:       accessTime,
		Perm:             s.Mode,
		Type:             fileType,
	}
}

// stripWorkPath removes the node's WorkPath prefix from an absolute path,
// producing a relative path for the daemon's gRPC file operations.
// The daemon prepends its own basePath, so sending absolute paths causes doubling.
func stripWorkPath(workPath, fullPath string) string {
	rel := strings.TrimPrefix(fullPath, workPath)
	rel = strings.TrimPrefix(rel, "/")

	if rel == "" {
		return "."
	}

	return rel
}
