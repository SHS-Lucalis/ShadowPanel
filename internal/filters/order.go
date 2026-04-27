package filters

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

type SortDirection int

const (
	SortDirectionAsc SortDirection = iota
	SortDirectionDesc
)

func (sd SortDirection) String() string {
	switch sd {
	case SortDirectionAsc:
		return "asc"
	case SortDirectionDesc:
		return "desc"
	default:
		return ""
	}
}

type Sorting struct {
	Field     string
	Direction SortDirection
}

func NewSorting(field string, direction SortDirection) *Sorting {
	return &Sorting{
		Field:     field,
		Direction: direction,
	}
}

func (p *Sorting) String() string {
	return fmt.Sprintf("%s %s", p.Field, p.Direction.String())
}

// ErrInvalidSortField is returned by ParseUserSort when the requested field is
// not in the caller-supplied allow-list.
var ErrInvalidSortField = errors.New("invalid sort field")

// ParseUserSort safely converts an untrusted "sort" query parameter into a
// Sorting value. It exists to prevent SQL injection through ORDER BY: the
// Sorting.String() output is interpolated directly into queries by repository
// implementations, so the field portion MUST be allow-listed by every caller
// that accepts a user-supplied value.
//
// Input format (matches the existing handler convention):
//
//	"name"   → ASC by `name`
//	"-name"  → DESC by `name`
//	""       → returns nil, nil (caller falls back to its default)
//
// allowed maps the public field name (what the API consumer sends) to the
// physical column name written to ORDER BY. This lets handlers decouple the
// API surface from the schema and gives a clean place to reject anything that
// isn't on the list.
//
// Returns ErrInvalidSortField when the requested field is not allowed.
func ParseUserSort(raw string, allowed map[string]string) (*Sorting, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}

	direction := SortDirectionAsc
	field := raw
	if strings.HasPrefix(raw, "-") {
		direction = SortDirectionDesc
		field = strings.TrimPrefix(raw, "-")
	}

	column, ok := allowed[field]
	if !ok || column == "" {
		return nil, errors.Wrapf(ErrInvalidSortField, "sort field %q is not allowed", field)
	}

	return &Sorting{Field: column, Direction: direction}, nil
}
