package idgen

import "github.com/rs/xid"

// New generates a globally unique, time-sortable, filesystem-safe ID.
func New() string {
	return xid.New().String()
}
