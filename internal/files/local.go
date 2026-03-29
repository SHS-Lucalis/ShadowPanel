package files

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

const (
	defaultLocalFilePerm = 0644
	defaultLocalDirPerm  = 0755
)

type LocalFileManager struct {
	root *os.Root
}

func NewLocalFileManager(basePath string) *LocalFileManager {
	root, err := os.OpenRoot(basePath)
	if err != nil {
		panic(fmt.Sprintf("failed to open root directory: %v", err))
	}

	return &LocalFileManager{
		root: root,
	}
}

func (fm *LocalFileManager) Read(_ context.Context, path string) ([]byte, error) {
	b, err := fm.root.ReadFile(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read file")
	}

	return b, nil
}

func (fm *LocalFileManager) Write(ctx context.Context, path string, data []byte) error {
	if !fm.Exists(ctx, path) {
		err := fm.mkdirAll(filepath.Dir(path))
		if err != nil {
			return errors.Wrapf(err, "failed to create directories: %s", filepath.Dir(path))
		}
	}

	err := fm.root.WriteFile(path, data, defaultLocalFilePerm)
	if err != nil {
		return errors.Wrap(err, "failed to write file")
	}

	return nil
}

func (fm *LocalFileManager) mkdirAll(path string) error {
	if path == "" || path == "." {
		return nil
	}

	_, err := fm.root.Stat(path)
	if err == nil {
		return nil
	}

	parent := filepath.Dir(path)
	if parent != path && parent != "." {
		if err := fm.mkdirAll(parent); err != nil {
			return errors.Wrapf(err, "failed to create directory: %s", parent)
		}
	}

	err = fm.root.Mkdir(path, defaultLocalDirPerm)
	if err != nil && !os.IsExist(err) {
		return errors.Wrapf(err, "failed to create directory: %s", path)
	}

	return nil
}

func (fm *LocalFileManager) Delete(_ context.Context, path string) error {
	err := fm.root.Remove(path)
	if err != nil {
		return errors.Wrap(err, "failed to delete file")
	}

	return nil
}

func (fm *LocalFileManager) Exists(_ context.Context, path string) bool {
	_, err := fm.root.Stat(path)

	return err == nil
}

func (fm *LocalFileManager) List(_ context.Context, dir string) ([]string, error) {
	dirFile, err := fm.root.Open(dir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open directory")
	}
	defer func(dirFile *os.File) {
		err := dirFile.Close()
		if err != nil {
			slog.Warn(fmt.Sprintf("failed to close directory file: %v", err))
		}
	}(dirFile)

	entries, err := dirFile.ReadDir(-1)
	if err != nil {
		return nil, errors.Wrap(err, "failed to read directory")
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		files = append(files, entry.Name())
	}

	return files, nil
}

func (fm *LocalFileManager) ReadStream(_ context.Context, path string) (io.ReadCloser, error) {
	file, err := fm.root.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file for reading")
	}

	return file, nil
}

func (fm *LocalFileManager) ReadStreamAt(_ context.Context, path string, offset int64) (io.ReadCloser, error) {
	file, err := fm.root.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open file for reading at offset")
	}

	if _, err := file.Seek(offset, io.SeekStart); err != nil {
		_ = file.Close()

		return nil, errors.Wrap(err, "failed to seek to offset")
	}

	return file, nil
}

func (fm *LocalFileManager) WriteStream(ctx context.Context, path string, data io.Reader) error {
	if !fm.Exists(ctx, path) {
		if err := fm.mkdirAll(filepath.Dir(path)); err != nil {
			return errors.Wrapf(err, "failed to create directories: %s", filepath.Dir(path))
		}
	}

	file, err := fm.root.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, defaultLocalFilePerm)
	if err != nil {
		return errors.Wrap(err, "failed to open file for writing")
	}

	if _, err := io.Copy(file, data); err != nil {
		_ = file.Close()

		return errors.Wrap(err, "failed to write stream data")
	}

	return file.Close()
}

func (fm *LocalFileManager) DeleteByPrefix(_ context.Context, prefix string) error {
	dirFile, err := fm.root.Open(filepath.Dir(prefix))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return errors.Wrap(err, "opening prefix directory")
	}

	entries, readErr := dirFile.ReadDir(-1)
	_ = dirFile.Close()

	if readErr != nil {
		return errors.Wrap(readErr, "reading prefix directory")
	}

	base := filepath.Base(prefix)
	dir := filepath.Dir(prefix)

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(dir, name)

		if !strings.HasPrefix(name, base) && name != base {
			continue
		}

		if entry.IsDir() {
			if err := fm.removeDirRecursive(fullPath); err != nil {
				return errors.Wrapf(err, "removing directory %s", fullPath)
			}
		} else {
			if err := fm.root.Remove(fullPath); err != nil && !os.IsNotExist(err) {
				return errors.Wrapf(err, "removing file %s", fullPath)
			}
		}
	}

	return nil
}

func (fm *LocalFileManager) removeDirRecursive(path string) error {
	dirFile, err := fm.root.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}

		return err
	}

	entries, readErr := dirFile.ReadDir(-1)
	_ = dirFile.Close()

	if readErr != nil {
		return readErr
	}

	for _, entry := range entries {
		childPath := filepath.Join(path, entry.Name())
		if entry.IsDir() {
			if err := fm.removeDirRecursive(childPath); err != nil {
				return err
			}
		} else {
			if err := fm.root.Remove(childPath); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}

	return fm.root.Remove(path)
}

var _ StreamFileManager = (*LocalFileManager)(nil)
