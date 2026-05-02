package createsession

import (
	"github.com/gameap/gameap/internal/api/filemanager/filemanagerpath"
	"github.com/pkg/errors"
)

type Input struct {
	Path             string `json:"path"`
	Filename         string `json:"filename"`
	TotalSize        uint64 `json:"total_size"`
	ExpectedChecksum string `json:"expected_checksum"`
}

func (i *Input) Validate() error {
	if err := filemanagerpath.ValidatePath(i.Path); err != nil {
		return err
	}
	if err := filemanagerpath.ValidateFilename(i.Filename); err != nil {
		return err
	}
	if i.TotalSize == 0 {
		return errors.New("total_size must be positive")
	}
	if i.ExpectedChecksum == "" {
		return errors.New("expected_checksum is required")
	}

	return nil
}
