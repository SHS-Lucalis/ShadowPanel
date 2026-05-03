package archiver

import (
	"context"
	"io"

	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
)

type FileLister interface {
	ReadDirRecursive(ctx context.Context, node *domain.Node, directory string) ([]*daemon.FileInfo, error)
	GetFileInfo(ctx context.Context, node *domain.Node, path string) (*daemon.FileDetails, error)
}

type FileStreamer interface {
	DownloadStream(ctx context.Context, node *domain.Node, filePath string) (io.ReadCloser, error)
}

type ConcurrencyGuard interface {
	Acquire(ctx context.Context, serverID uint) (release func(), err error)
}
