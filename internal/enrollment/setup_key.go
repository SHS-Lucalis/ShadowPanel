package enrollment

import (
	"context"
	"crypto/subtle"
	"time"

	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/pkg/strings"
	"github.com/pkg/errors"
)

const (
	SetupKeyCacheKey = "daemon:setup_key"
	setupKeyLength   = 32
	setupKeyTTL      = 1 * time.Hour
)

var (
	ErrSetupKeyNotConfigured = errors.New("daemon setup key is not configured")
	ErrInvalidSetupKey       = errors.New("invalid setup key")
)

type SetupKeyManager struct {
	cache    cache.Cache
	envValue string
}

func NewSetupKeyManager(cache cache.Cache, envValue string) *SetupKeyManager {
	return &SetupKeyManager{
		cache:    cache,
		envValue: envValue,
	}
}

func (m *SetupKeyManager) Validate(ctx context.Context, key string) error {
	storedKey, err := m.Get(ctx)
	if err != nil {
		return err
	}

	if subtle.ConstantTimeCompare([]byte(key), []byte(storedKey)) != 1 {
		return ErrInvalidSetupKey
	}

	return nil
}

func (m *SetupKeyManager) Get(ctx context.Context) (string, error) {
	val, err := m.cache.Get(ctx, SetupKeyCacheKey)
	if err == nil {
		key, ok := val.(string)
		if ok {
			if key == "" {
				return "", ErrSetupKeyNotConfigured
			}

			return key, nil
		}
	}

	if err != nil && !errors.Is(err, cache.ErrNotFound) {
		return "", errors.WithMessage(err, "failed to get setup key from cache")
	}

	if m.envValue != "" {
		return m.envValue, nil
	}

	return "", ErrSetupKeyNotConfigured
}

func (m *SetupKeyManager) Set(ctx context.Context, key string, opts ...cache.Option) error {
	return m.cache.Set(ctx, SetupKeyCacheKey, key, opts...)
}

func (m *SetupKeyManager) Generate(ctx context.Context) (string, error) {
	key, err := strings.CryptoRandomString(setupKeyLength)
	if err != nil {
		return "", errors.WithMessage(err, "failed to generate setup key")
	}

	if err := m.Set(ctx, key, cache.WithExpiration(setupKeyTTL)); err != nil {
		return "", errors.WithMessage(err, "failed to store setup key")
	}

	return key, nil
}

func (m *SetupKeyManager) Invalidate(ctx context.Context) error {
	if m.envValue != "" {
		return m.cache.Set(ctx, SetupKeyCacheKey, "")
	}

	return m.cache.Delete(ctx, SetupKeyCacheKey)
}

func (m *SetupKeyManager) Delete(ctx context.Context) error {
	return m.cache.Delete(ctx, SetupKeyCacheKey)
}
