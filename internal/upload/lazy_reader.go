package upload

import (
	"context"
	"io"

	"github.com/pkg/errors"
)

type lazyChunkReader struct {
	ctx     context.Context
	storage Storage
	path    string
	rc      io.ReadCloser
	done    bool
}

func newLazyChunkReader(ctx context.Context, storage Storage, path string) *lazyChunkReader {
	return &lazyChunkReader{
		ctx:     ctx,
		storage: storage,
		path:    path,
	}
}

func (l *lazyChunkReader) Read(p []byte) (int, error) {
	if l.done {
		return 0, io.EOF
	}
	if l.rc == nil {
		rc, err := l.storage.ReadStream(l.ctx, l.path)
		if err != nil {
			l.done = true

			return 0, errors.WithMessagef(err, "open chunk %s", l.path)
		}
		l.rc = rc
	}
	n, err := l.rc.Read(p)
	if err != nil {
		_ = l.rc.Close()
		l.rc = nil
		l.done = true
	}

	return n, err
}

func (l *lazyChunkReader) Close() error {
	l.done = true
	if l.rc != nil {
		err := l.rc.Close()
		l.rc = nil

		return err
	}

	return nil
}
