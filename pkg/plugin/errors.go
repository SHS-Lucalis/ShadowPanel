package plugin

import "github.com/pkg/errors"

var (
	ErrManagerClosed        = errors.New("plugin manager is closed")
	ErrPluginAlreadyLoaded  = errors.New("plugin already loaded")
	ErrPluginNotFound       = errors.New("plugin not found")
	ErrAPIVersionMismatch   = errors.New("API version mismatch")
	ErrUnexpectedExitCode   = errors.New("unexpected exit code")
	ErrInitializationFailed = errors.New("plugin initialization failed")
	ErrExportNotFound       = errors.New("required export not found")
	ErrMemoryOutOfRange     = errors.New("memory operation out of range")
	ErrPluginReturnedError  = errors.New("plugin returned error")
)
