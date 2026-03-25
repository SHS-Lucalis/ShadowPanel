package daemon

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"math"
	"net"
	"os"
	"sync"
	"time"

	"github.com/gameap/gameap/internal/daemon/binnapi"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/pkg/errors"
)

const (
	filesRetryCount = 2
	filesRetryDelay = 10 * time.Millisecond
)

// Deprecated: Use FileService instead. FileBINNService uses the legacy BinAPI protocol.
type FileBINNService struct {
	configMaker *configMaker

	mu    sync.RWMutex
	pools map[uint]*Pool
}

// Deprecated: Use NewFileService instead.
func NewFileBINNService(
	certRepo repositories.ClientCertificateRepository,
	fileManager files.FileManager,
) *FileBINNService {
	return &FileBINNService{
		configMaker: newConfigMaker(certRepo, fileManager),
		pools:       make(map[uint]*Pool),
	}
}

// ReadDir reads the contents of a directory.
func (s *FileBINNService) ReadDir(
	ctx context.Context,
	node *domain.Node,
	directory string,
) ([]*FileInfo, error) {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.ReadDirRequestMessage{
			Directory:   directory,
			DetailsMode: false,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write read dir request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read read dir response")
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithMessagef(
			err,
			"failed to read directory after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return nil, errors.Errorf("read dir failed with status code %d: %s", resp.Code, resp.Info)
	}

	fileList, ok := resp.Data.([]any)
	if !ok {
		return nil, errors.New("invalid response data format")
	}

	var resultList = make([]*FileInfo, 0, len(fileList))
	for _, item := range fileList {
		fileData, ok := item.([]any)
		if !ok {
			return nil, errors.New("invalid file info format")
		}

		binnapiFileInfo, err := binnapi.CreateFileInfoResponseMessage(fileData)
		if err != nil {
			return nil, errors.WithMessage(err, "failed to parse file info")
		}

		resultList = append(resultList, &FileInfo{
			Name:         binnapiFileInfo.Name,
			Size:         binnapiFileInfo.Size,
			TimeModified: binnapiFileInfo.TimeModified,
			Type:         FileType(binnapiFileInfo.Type),
			Perm:         binnapiFileInfo.Perm,
		})
	}

	return resultList, nil
}

// MkDir creates a directory.
func (s *FileBINNService) MkDir(ctx context.Context, node *domain.Node, directory string) error {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.MkDirRequestMessage{
			Directory: directory,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write mkdir request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read mkdir response")
		}

		return nil
	})
	if err != nil {
		return errors.WithMessagef(
			err,
			"failed to create directory after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return errors.Errorf("mkdir failed with status code %d: %s", resp.Code, resp.Info)
	}

	return nil
}

func (s *FileBINNService) Copy(ctx context.Context, node *domain.Node, source, destination string) error {
	return s.move(ctx, node, source, destination, true)
}

func (s *FileBINNService) Move(ctx context.Context, node *domain.Node, source, destination string) error {
	return s.move(ctx, node, source, destination, false)
}

// Move moves or copies a file.
func (s *FileBINNService) move(ctx context.Context, node *domain.Node, source, destination string, cp bool) error {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.MoveRequestMessage{
			Source:      source,
			Destination: destination,
			Copy:        cp,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write move request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read move response")
		}

		return nil
	})
	if err != nil {
		return errors.WithMessagef(
			err,
			"failed to move/copy file after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return errors.Errorf("move failed with status code %d: %s", resp.Code, resp.Info)
	}

	return nil
}

// Download downloads a file from the daemon.
func (s *FileBINNService) Download(ctx context.Context, node *domain.Node, filePath string) ([]byte, error) {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage
	var file []byte

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.DownloadRequestMessage{
			FilePath: filePath,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write download request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read download response")
		}

		if resp.Code != binnapi.StatusCodeReadyToTransfer {
			return errors.Errorf("download failed with status code %d: %s", resp.Code, resp.Info)
		}

		fileSize, err := binnapi.CreateFileSize(resp.Data)
		if err != nil {
			return errors.WithMessage(err, "failed to get file size")
		}

		if fileSize == 0 {
			file = []byte{}

			return nil
		}

		file = make([]byte, fileSize)
		_, err = io.ReadFull(conn, file)
		if err != nil {
			return errors.WithMessage(err, "failed to read file content")
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithMessagef(
			err,
			"failed to download file after %d attempts",
			filesRetryCount,
		)
	}

	return file, nil
}

// DownloadStream downloads a file from the daemon as a stream.
// The caller is responsible for closing the returned ReadCloser.
func (s *FileBINNService) DownloadStream(
	ctx context.Context,
	node *domain.Node,
	filePath string,
) (io.ReadCloser, error) {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pool")
	}

	var conn net.Conn
	var resp binnapi.BaseResponseMessage
	var fileSize uint64

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		var err error
		conn, err = pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}

		err = binnapi.WriteMessage(conn, &binnapi.DownloadRequestMessage{
			FilePath: filePath,
		})
		if err != nil {
			connCloseErr := conn.Close()
			if connCloseErr != nil {
				slog.Warn("failed to close connection", "error", connCloseErr)
			}

			return errors.WithMessage(err, "failed to write download request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			connCloseErr := conn.Close()
			if connCloseErr != nil {
				slog.Warn("failed to close connection", "error", connCloseErr)
			}

			return errors.WithMessage(err, "failed to read download response")
		}

		if resp.Code != binnapi.StatusCodeReadyToTransfer {
			connCloseErr := conn.Close()
			if connCloseErr != nil {
				slog.Warn("failed to close connection", "error", connCloseErr)
			}

			return errors.Errorf("download failed with status code %d: %s", resp.Code, resp.Info)
		}

		fileSizeValue, err := binnapi.CreateFileSize(resp.Data)
		if err != nil {
			connCloseErr := conn.Close()
			if connCloseErr != nil {
				slog.Warn("failed to close connection", "error", connCloseErr)
			}

			return errors.WithMessage(err, "failed to get file size")
		}
		fileSize = uint64(fileSizeValue)

		return nil
	})
	if err != nil {
		return nil, errors.WithMessagef(
			err,
			"failed to initialize download stream after %d attempts",
			filesRetryCount,
		)
	}

	if fileSize > uint64(math.MaxInt64) {
		connCloseErr := conn.Close()
		if connCloseErr != nil {
			slog.Warn("failed to close connection", "error", connCloseErr)
		}

		return nil, errors.New("file size exceeds maximum supported size")
	}

	return &fileStreamReader{
		reader: io.LimitReader(conn, int64(fileSize)),
		closer: conn,
	}, nil
}

// fileStreamReader wraps an io.Reader and io.Closer to provide io.ReadCloser functionality.
type fileStreamReader struct {
	reader io.Reader
	closer io.Closer
}

func (f *fileStreamReader) Read(p []byte) (n int, err error) {
	return f.reader.Read(p)
}

func (f *fileStreamReader) Close() error {
	return f.closer.Close()
}

// Upload uploads a file to the daemon.
func (s *FileBINNService) Upload(
	ctx context.Context,
	node *domain.Node,
	filePath string,
	content []byte,
	perms os.FileMode,
) error {
	return s.UploadStream(
		ctx,
		node,
		filePath,
		bytes.NewReader(content),
		uint64(len(content)),
		perms,
	)
}

func (s *FileBINNService) UploadStream(
	ctx context.Context,
	node *domain.Node,
	filePath string,
	r io.Reader,
	size uint64,
	perms os.FileMode,
) error {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.UploadRequestMessage{
			FilePath: filePath,
			FileSize: size,
			MakeDirs: true,
			Perms:    perms,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write upload request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read upload response")
		}

		if resp.Code != binnapi.StatusCodeReadyToTransfer {
			return errors.Errorf("upload failed with status code %d: %s", resp.Code, resp.Info)
		}

		_, err = io.Copy(conn, r)
		if err != nil {
			return errors.WithMessage(err, "failed to stream file content")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read upload response")
		}

		if resp.Code != binnapi.StatusCodeOK {
			return errors.Errorf("upload failed with status code %d: %s", resp.Code, resp.Info)
		}

		return nil
	})
	if err != nil {
		return errors.WithMessagef(
			err,
			"failed to upload stream after %d attempts",
			filesRetryCount,
		)
	}

	return nil
}

// Remove removes a file or directory.
func (s *FileBINNService) Remove(ctx context.Context, node *domain.Node, path string, recursive bool) error {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.RemoveRequestMessage{
			Path:      path,
			Recursive: recursive,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write remove request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read remove response")
		}

		return nil
	})
	if err != nil {
		return errors.WithMessagef(
			err,
			"failed to remove file after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return errors.Errorf("remove failed with status code %d: %s", resp.Code, resp.Info)
	}

	return nil
}

// GetFileInfo gets detailed information about a file.
func (s *FileBINNService) GetFileInfo(ctx context.Context, node *domain.Node, path string) (*FileDetails, error) {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.FileInfoRequestMessage{
			Path: path,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write file info request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read file info response")
		}

		return nil
	})
	if err != nil {
		return nil, errors.WithMessagef(
			err,
			"failed to get file info after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return nil, errors.Errorf("file info failed with status code %d: %s", resp.Code, resp.Info)
	}

	binnapiFileDetails, err := binnapi.CreateFileDetailsResponseMessage(resp.Data)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to parse file details")
	}

	return &FileDetails{
		Name:             binnapiFileDetails.Name,
		Mime:             binnapiFileDetails.Mime,
		Size:             binnapiFileDetails.Size,
		ModificationTime: binnapiFileDetails.ModificationTime,
		AccessTime:       binnapiFileDetails.AccessTime,
		CreateTime:       binnapiFileDetails.CreateTime,
		Perm:             binnapiFileDetails.Perm,
		Type:             FileType(binnapiFileDetails.Type),
	}, nil
}

// Chmod changes file permissions.
func (s *FileBINNService) Chmod(ctx context.Context, node *domain.Node, path string, perm uint32) error {
	cfg, err := s.configMaker.MakeWithMode(ctx, node, binnapi.ModeFiles)
	if err != nil {
		return errors.WithMessage(err, "failed to make config")
	}

	pool, err := s.getPool(node.ID, cfg)
	if err != nil {
		return errors.WithMessage(err, "failed to get pool")
	}

	var resp binnapi.BaseResponseMessage

	err = Retry(filesRetryCount, filesRetryDelay, func() error {
		conn, err := pool.Acquire(ctx)
		if err != nil {
			return errors.WithMessage(err, "failed to acquire connection from pool")
		}
		defer func() {
			err = conn.Close()
			if err != nil {
				slog.Warn("failed to close connection", "error", err)
			}
		}()

		err = binnapi.WriteMessage(conn, &binnapi.ChmodMessage{
			Path: path,
			Perm: perm,
		})
		if err != nil {
			return errors.WithMessage(err, "failed to write chmod request")
		}

		err = binnapi.ReadMessage(conn, &resp)
		if err != nil {
			return errors.WithMessage(err, "failed to read chmod response")
		}

		return nil
	})
	if err != nil {
		return errors.WithMessagef(
			err,
			"failed to change file permissions after %d attempts",
			filesRetryCount,
		)
	}

	if resp.Code != binnapi.StatusCodeOK {
		return errors.Errorf("chmod failed with status code %d: %s", resp.Code, resp.Info)
	}

	return nil
}

func (s *FileBINNService) getPool(nodeID uint, cfg config) (*Pool, error) {
	s.mu.RLock()
	pool, exists := s.pools[nodeID]
	s.mu.RUnlock()

	if exists {
		return pool, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Double-check existence to avoid race condition
	pool, exists = s.pools[nodeID]
	if exists {
		return pool, nil
	}

	pool, err := NewPool(cfg)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to create pool")
	}

	s.pools[nodeID] = pool

	return pool, nil
}
