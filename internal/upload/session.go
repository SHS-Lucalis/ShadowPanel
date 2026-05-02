package upload

import (
	"fmt"
	"strings"
	"time"
)

const (
	transferPrefix    = "transfers/"
	metadataFileName  = "upload.json"
	dataFileName      = "data"
	doneFileName      = "done"
	chunksDirName     = "chunks"
	chunkIndexPattern = "%06d"
)

type Session struct {
	UploadID         string    `json:"upload_id"`
	ServerID         uint      `json:"server_id"`
	NodeID           uint      `json:"node_id"`
	UserID           uint      `json:"user_id"`
	FullPath         string    `json:"full_path"`
	TotalSize        uint64    `json:"total_size"`
	ChunkSize        uint64    `json:"chunk_size"`
	TotalChunks      uint      `json:"total_chunks"`
	ExpectedChecksum string    `json:"expected_checksum"`
	CreatedAt        time.Time `json:"created_at"`
	ExpiresAt        time.Time `json:"expires_at"`
}

func (s *Session) ChunkSizeFor(index uint) uint64 {
	if index+1 == s.TotalChunks {
		size := s.TotalSize - s.ChunkSize*uint64(s.TotalChunks-1)

		return size
	}

	return s.ChunkSize
}

type DoneInfo struct {
	Success  bool   `json:"success"`
	Checksum string `json:"checksum,omitempty"`
}

func transferRoot(uploadID string) string {
	return transferPrefix + uploadID + "/"
}

func metadataPath(uploadID string) string {
	return transferRoot(uploadID) + metadataFileName
}

func dataPath(uploadID string) string {
	return transferRoot(uploadID) + dataFileName
}

func donePath(uploadID string) string {
	return transferRoot(uploadID) + doneFileName
}

func chunksPrefix(uploadID string) string {
	return transferRoot(uploadID) + chunksDirName + "/"
}

func chunkPath(uploadID string, index uint) string {
	return chunksPrefix(uploadID) + fmt.Sprintf(chunkIndexPattern, index)
}

func indexFromChunkPath(uploadID, path string) (uint, bool) {
	prefix := chunksPrefix(uploadID)
	if !strings.HasPrefix(path, prefix) {
		return 0, false
	}
	tail := strings.TrimPrefix(path, prefix)
	if tail == "" || strings.Contains(tail, "/") {
		return 0, false
	}
	var index uint
	if _, err := fmt.Sscanf(tail, chunkIndexPattern, &index); err != nil {
		return 0, false
	}

	return index, true
}
