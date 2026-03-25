package daemon

import (
	"context"

	"github.com/gameap/gameap/pkg/proto"
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
	IsConnectedAnywhere(nodeID uint64) bool
}

type FileDispatcher interface {
	Start(ctx context.Context) error

	DispatchFileList(
		ctx context.Context, nodeID uint64, path string, recursive bool, pattern string,
	) (*proto.FileListResponse, error)
	DispatchFileOperation(
		ctx context.Context, nodeID uint64, req *proto.FileOperationRequest,
	) (*proto.FileOperationResponse, error)
	DispatchFileRead(
		ctx context.Context, nodeID uint64, path string, offset int64, length int64,
	) (*FileReadResult, error)
	DispatchFileWrite(
		ctx context.Context, nodeID uint64, path string, content []byte, mode int32, createDirs bool,
	) error
	DispatchUploadTask(ctx context.Context, nodeID uint64, transferID string, destPath string) error
	DispatchDownloadTask(ctx context.Context, nodeID uint64, transferID string, srcPath string) error
}

type FileReadResult struct {
	Content     []byte
	StoragePath string
}
