package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gameap/gameap/internal/cache"
)

const revocationKeyPrefix = "auth:revoked:"

// CacheRevocation backs TokenRevocation with the project's cache.Cache
// abstraction. Behind a Redis cache it spreads the denylist across the cluster;
// behind the in-memory cache it scopes the denylist to the local process
// (acceptable for single-instance deployments and for tests).
type CacheRevocation struct {
	cache cache.Cache
}

func NewCacheRevocation(c cache.Cache) *CacheRevocation {
	return &CacheRevocation{cache: c}
}

func (r *CacheRevocation) Revoke(ctx context.Context, identifier string, ttl time.Duration) error {
	if ttl <= 0 {
		// The token is already past its natural lifetime; nothing to do.
		return nil
	}

	return r.cache.Set(ctx, revocationKeyPrefix+identifier, true, cache.WithExpiration(ttl))
}

func (r *CacheRevocation) IsRevoked(ctx context.Context, identifier string) (bool, error) {
	_, err := r.cache.Get(ctx, revocationKeyPrefix+identifier)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, cache.ErrNotFound) {
		return false, nil
	}

	return false, err
}
