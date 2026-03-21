package gamesimport

import (
	"github.com/gameap/gameap/pkg/strings"
	"github.com/pkg/errors"
)

const (
	minCodeLength = 2
	maxCodeLength = 16
	minNameLength = 2
	maxNameLength = 128
)

type Options struct {
	Name *string
	Code *string
}

func (o *Options) Validate() error {
	if o == nil {
		return nil
	}

	if o.Code != nil {
		if len(*o.Code) < minCodeLength || len(*o.Code) > maxCodeLength {
			return errors.Errorf("code must be between %d and %d characters", minCodeLength, maxCodeLength)
		}

		if !strings.IsSlug(*o.Code) {
			return errors.New("code must match pattern: ^[a-z0-9_-]+$")
		}
	}

	if o.Name != nil {
		if len(*o.Name) < minNameLength || len(*o.Name) > maxNameLength {
			return errors.Errorf("name must be between %d and %d characters", minNameLength, maxNameLength)
		}
	}

	return nil
}

func (o *Options) IsEmpty() bool {
	return o == nil || (o.Name == nil && o.Code == nil)
}
