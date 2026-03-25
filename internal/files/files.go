package files

import (
	"context"
	"io"
)

type FileManager interface {
	Read(ctx context.Context, path string) ([]byte, error)
	Write(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
	Exists(ctx context.Context, path string) bool
	List(ctx context.Context, dir string) ([]string, error)
}

type StreamFileManager interface {
	FileManager

	ReadStream(ctx context.Context, path string) (io.ReadCloser, error)
	WriteStream(ctx context.Context, path string, data io.Reader) error
}
