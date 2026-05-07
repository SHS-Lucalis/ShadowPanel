package acme

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-acme/lego/v4/certificate"
	"github.com/go-acme/lego/v4/challenge"
)

type State string

const (
	StateDisabled State = "disabled"
	StatePending  State = "pending"
	StateActive   State = "active"
	StateRenewing State = "renewing"
	StateFailed   State = "failed"
)

type Status struct {
	Enabled            bool
	State              State
	ChallengeType      string
	Domains            []string
	DNSProvider        string
	NotBefore          time.Time
	NotAfter           time.Time
	LastRenewalAt      time.Time
	NextRenewalCheckAt time.Time
	LastError          string
}

type Account struct {
	Email        string          `json:"email"`
	PrivateKey   []byte          `json:"private_key"`
	Registration json.RawMessage `json:"registration"`
}

type Storage interface {
	SaveAccount(ctx context.Context, account *Account) error
	LoadAccount(ctx context.Context, email string) (*Account, error)
	HasAccount(ctx context.Context, email string) (bool, error)

	SaveResource(ctx context.Context, key string, resource *certificate.Resource) error
	LoadResource(ctx context.Context, key string) (*certificate.Resource, error)
	HasResource(ctx context.Context, key string) (bool, error)
	DeleteResource(ctx context.Context, key string) error
}

type Locker interface {
	Acquire(ctx context.Context, key string, ttl time.Duration) (Lock, error)
}

type Lock interface {
	Release(ctx context.Context) error
	Refresh(ctx context.Context, ttl time.Duration) error
}

// DNSProviderRegistry resolves DNS provider identifiers to lego challenge.Provider
// implementations. Identifier format: "<plugin-id>:<provider-name>" for plugin-backed
// providers, or a bare provider name for built-in registry entries.
type DNSProviderRegistry interface {
	Resolve(ctx context.Context, identifier string) (challenge.Provider, error)
}
