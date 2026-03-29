package transfers

import "fmt"

const (
	InitialPartSize = 1 * 1024 * 1024
	MaxPartSize     = 8 * 1024 * 1024

	transferPrefix = "transfers/"
)

type DoneInfo struct {
	Success    bool   `json:"success"`
	Checksum   string `json:"checksum,omitempty"`
	TotalParts int    `json:"total_parts"`
	Error      string `json:"error,omitempty"`
}

func TransferPartPath(transferID string, partNum int) string {
	return transferPrefix + transferID + "/parts/" + fmt.Sprintf("%06d", partNum)
}

func TransferPartsPrefix(transferID string) string {
	return transferPrefix + transferID + "/parts/"
}

func TransferDonePath(transferID string) string {
	return transferPrefix + transferID + "/done"
}

func TransferPrefix(transferID string) string {
	return transferPrefix + transferID + "/"
}

func TransferDataPath(transferID string) string {
	return transferPrefix + transferID + "/data"
}

func PartSizeForNum(partNum int) int {
	size := InitialPartSize
	for i := 0; i < partNum && size < MaxPartSize; i++ {
		size *= 2
	}

	return size
}
