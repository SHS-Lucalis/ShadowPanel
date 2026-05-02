package getsession

import (
	"time"

	"github.com/gameap/gameap/internal/upload"
)

type Response struct {
	UploadID       string    `json:"upload_id"`
	TotalSize      uint64    `json:"total_size"`
	ChunkSize      uint64    `json:"chunk_size"`
	TotalChunks    uint      `json:"total_chunks"`
	ReceivedChunks []uint    `json:"received_chunks"`
	MissingChunks  []uint    `json:"missing_chunks"`
	UploadedBytes  uint64    `json:"uploaded_bytes"`
	Completed      bool      `json:"completed"`
	ExpiresAt      time.Time `json:"expires_at"`
}

func newResponse(status *upload.SessionStatus) *Response {
	received := status.ReceivedChunks
	if received == nil {
		received = []uint{}
	}
	missing := status.MissingChunks
	if missing == nil {
		missing = []uint{}
	}

	return &Response{
		UploadID:       status.Session.UploadID,
		TotalSize:      status.Session.TotalSize,
		ChunkSize:      status.Session.ChunkSize,
		TotalChunks:    status.Session.TotalChunks,
		ReceivedChunks: received,
		MissingChunks:  missing,
		UploadedBytes:  status.UploadedBytes,
		Completed:      status.Completed,
		ExpiresAt:      status.Session.ExpiresAt,
	}
}
