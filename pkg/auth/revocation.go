package auth

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"
)

// TokenRevocation lets the API revoke a previously-issued bearer token before
// its natural expiration (logout flow, security incident response). Lookups are
// performed by every authenticated request, so the implementation must be cheap.
//
// Token identifiers passed to Revoke / IsRevoked are caller-opaque strings;
// callers are expected to feed in a stable derivative of the bearer token
// (typically TokenIdentifier(rawBearer)) and never the raw bearer itself, so
// the revocation store never holds a usable credential.
type TokenRevocation interface {
	// Revoke marks the identifier as revoked for at most `ttl`. After the TTL
	// elapses the entry is reaped — by then the underlying token would have
	// expired naturally as well.
	Revoke(ctx context.Context, identifier string, ttl time.Duration) error

	// IsRevoked returns true iff Revoke was called for the identifier and the
	// entry has not yet expired.
	IsRevoked(ctx context.Context, identifier string) (bool, error)
}

// TokenIdentifier returns a stable, opaque identifier for a bearer token.
// It is intentionally a one-way function so the revocation store cannot be
// used as an oracle for valid credentials.
func TokenIdentifier(rawBearer string) string {
	sum := sha256.Sum256([]byte(rawBearer))

	return hex.EncodeToString(sum[:])
}

// NoopRevocation is used in contexts where revocation is not wired up yet
// (e.g. legacy unit tests). It never reports a token as revoked.
type NoopRevocation struct{}

func (NoopRevocation) Revoke(_ context.Context, _ string, _ time.Duration) error {
	return nil
}

func (NoopRevocation) IsRevoked(_ context.Context, _ string) (bool, error) {
	return false, nil
}
