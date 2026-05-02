package filemanagerpath

import (
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

var (
	ErrPathContainsTraversal         = errors.New("path contains invalid directory traversal")
	ErrPathEscapesBaseDirectory      = errors.New("path attempts to escape base directory")
	ErrFilenameEmpty                 = errors.New("filename is empty")
	ErrFilenameContainsTraversal     = errors.New("filename contains invalid directory traversal")
	ErrFilenameContainsPathSeparator = errors.New("filename contains path separators")
)

func ValidatePath(path string) error {
	if strings.Contains(path, "..") {
		return ErrPathContainsTraversal
	}

	cleanPath := filepath.Clean(path)
	if strings.HasPrefix(cleanPath, "..") {
		return ErrPathEscapesBaseDirectory
	}

	return nil
}

func ValidateFilename(filename string) error {
	if filename == "" {
		return ErrFilenameEmpty
	}

	if strings.Contains(filename, "..") {
		return ErrFilenameContainsTraversal
	}

	if strings.ContainsAny(filename, "/\\") {
		return ErrFilenameContainsPathSeparator
	}

	return nil
}
