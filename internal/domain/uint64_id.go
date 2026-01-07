package domain

import (
	"database/sql/driver"
	"strconv"

	"github.com/pkg/errors"
)

type Uint64ID uint64

func (id Uint64ID) Value() (driver.Value, error) {
	return int64(id), nil //nolint:gosec
}

func (id *Uint64ID) Scan(src any) error {
	switch v := src.(type) {
	case int64:
		*id = Uint64ID(v) //nolint:gosec

		return nil
	case uint64:
		*id = Uint64ID(v)

		return nil
	case int32:
		*id = Uint64ID(v) //nolint:gosec

		return nil
	case int:
		*id = Uint64ID(v) //nolint:gosec

		return nil
	case string:
		parsed, err := strconv.ParseUint(v, 10, 64)
		if err != nil {
			return errors.Wrap(err, "failed to parse string to uint64")
		}

		*id = Uint64ID(parsed)

		return nil
	case nil:
		return nil
	default:
		return errors.Errorf("cannot scan %T into Uint64ID", src)
	}
}
