package files

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
)

// InMemoryFileManager is an in-memory implementation of FileManager for testing purposes.
type InMemoryFileManager struct {
	mu    sync.RWMutex
	files map[string][]byte
}

// NewInMemoryFileManager creates a new InMemoryFileManager instance.
func NewInMemoryFileManager() *InMemoryFileManager {
	return &InMemoryFileManager{
		files: make(map[string][]byte),
	}
}

func (fm *InMemoryFileManager) Read(_ context.Context, path string) ([]byte, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	data, exists := fm.files[path]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", path) //nolint:err113
	}

	return data, nil
}

func (fm *InMemoryFileManager) Write(_ context.Context, path string, data []byte) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.files[path] = data

	return nil
}

func (fm *InMemoryFileManager) Delete(_ context.Context, path string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	delete(fm.files, path)

	return nil
}

func (fm *InMemoryFileManager) Exists(_ context.Context, path string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	_, exists := fm.files[path]

	return exists
}

func (fm *InMemoryFileManager) List(_ context.Context, dir string) ([]string, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	var result []string
	for path := range fm.files {
		if strings.HasPrefix(path, dir) {
			result = append(result, path)
		}
	}

	return result, nil
}

func (fm *InMemoryFileManager) ReadStream(_ context.Context, path string) (io.ReadCloser, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	data, exists := fm.files[path]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", path) //nolint:err113
	}

	cp := make([]byte, len(data))
	copy(cp, data)

	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (fm *InMemoryFileManager) ReadStreamAt(_ context.Context, path string, offset int64) (io.ReadCloser, error) {
	fm.mu.RLock()
	defer fm.mu.RUnlock()

	data, exists := fm.files[path]
	if !exists {
		return nil, fmt.Errorf("file not found: %s", path) //nolint:err113
	}

	if offset >= int64(len(data)) {
		return io.NopCloser(bytes.NewReader(nil)), nil
	}

	cp := make([]byte, int64(len(data))-offset)
	copy(cp, data[offset:])

	return io.NopCloser(bytes.NewReader(cp)), nil
}

func (fm *InMemoryFileManager) WriteStream(_ context.Context, path string, data io.Reader) error {
	content, err := io.ReadAll(data)
	if err != nil {
		return fmt.Errorf("failed to read stream data: %w", err)
	}

	fm.mu.Lock()
	defer fm.mu.Unlock()

	fm.files[path] = content

	return nil
}

func (fm *InMemoryFileManager) DeleteByPrefix(_ context.Context, prefix string) error {
	fm.mu.Lock()
	defer fm.mu.Unlock()

	for path := range fm.files {
		if strings.HasPrefix(path, prefix) {
			delete(fm.files, path)
		}
	}

	return nil
}

var _ StreamFileManager = (*InMemoryFileManager)(nil)
