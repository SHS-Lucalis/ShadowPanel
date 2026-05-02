package upload

import (
	"net/http"

	"github.com/gameap/gameap/pkg/api"
)

var (
	ErrSessionNotFound = api.NewError(
		http.StatusNotFound, "upload session not found",
	)
	ErrSessionForbidden = api.NewError(
		http.StatusForbidden, "upload session belongs to another user",
	)
	ErrSessionExpired = api.NewError(
		http.StatusGone, "upload session expired",
	)
	ErrInvalidIndex = api.NewError(
		http.StatusBadRequest, "chunk index out of range",
	)
	ErrChunkSizeMismatch = api.NewError(
		http.StatusRequestEntityTooLarge, "chunk size does not match expected size for this index",
	)
	ErrIncompleteUpload = api.NewError(
		http.StatusConflict, "not all chunks have been uploaded",
	)
	ErrChecksumMismatch = api.NewError(
		http.StatusUnprocessableEntity, "assembled file checksum does not match expected checksum",
	)
	ErrInvalidChecksum = api.NewError(
		http.StatusBadRequest, "expected_checksum must be 64 lowercase hex characters",
	)
	ErrInvalidTotalSize = api.NewError(
		http.StatusBadRequest, "total_size must be positive",
	)
	ErrTooManyChunks = api.NewError(
		http.StatusBadRequest, "requested file would require too many chunks",
	)
	ErrSessionAlreadyDone = api.NewError(
		http.StatusConflict, "upload session is already completed",
	)
	ErrNodeMismatch = api.NewError(
		http.StatusForbidden, "node does not match upload session",
	)
)
