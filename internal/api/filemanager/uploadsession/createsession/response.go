package createsession

import (
	"time"

	"github.com/gameap/gameap/internal/upload"
)

type Response struct {
	UploadID    string    `json:"upload_id"`
	ChunkSize   uint64    `json:"chunk_size"`
	TotalChunks uint      `json:"total_chunks"`
	TotalSize   uint64    `json:"total_size"`
	ExpiresAt   time.Time `json:"expires_at"`
}

func newResponse(sess *upload.Session) *Response {
	return &Response{
		UploadID:    sess.UploadID,
		ChunkSize:   sess.ChunkSize,
		TotalChunks: sess.TotalChunks,
		TotalSize:   sess.TotalSize,
		ExpiresAt:   sess.ExpiresAt,
	}
}
