package getstatus

import (
	"github.com/gameap/gameap/internal/acme"
)

type ACMEService interface {
	Status() acme.Status
}

type Config interface {
	ACMEEnabled() bool
}
