package hostlibrary

import (
	"context"
	"errors"
	"time"

	intcache "github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/pkg/plugin/sdk/cache"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type CacheServiceImpl struct {
	cache     intcache.Cache
	keyPrefix string
}

func NewCacheService(c intcache.Cache, keyPrefix string) *CacheServiceImpl {
	return &CacheServiceImpl{
		cache:     c,
		keyPrefix: keyPrefix,
	}
}

func (s *CacheServiceImpl) prefixedKey(key string) string {
	return s.keyPrefix + key
}

func (s *CacheServiceImpl) Get(
	ctx context.Context,
	req *cache.CacheGetRequest,
) (*cache.CacheGetResponse, error) {
	value, err := s.cache.Get(ctx, s.prefixedKey(req.Key))
	if err != nil {
		if errors.Is(err, intcache.ErrNotFound) {
			return &cache.CacheGetResponse{
				Found: false,
			}, nil
		}

		return nil, err
	}

	bytes, ok := value.([]byte)
	if !ok {
		return &cache.CacheGetResponse{
			Found: false,
		}, nil
	}

	return &cache.CacheGetResponse{
		Value: bytes,
		Found: true,
	}, nil
}

func (s *CacheServiceImpl) Set(
	ctx context.Context,
	req *cache.CacheSetRequest,
) (*cache.CacheSetResponse, error) {
	var opts []intcache.Option
	if req.TtlSeconds > 0 {
		opts = append(opts, intcache.WithExpiration(time.Duration(req.TtlSeconds)*time.Second))
	}

	err := s.cache.Set(ctx, s.prefixedKey(req.Key), req.Value, opts...)
	if err != nil {
		return &cache.CacheSetResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &cache.CacheSetResponse{
		Success: true,
	}, nil
}

func (s *CacheServiceImpl) Delete(
	ctx context.Context,
	req *cache.CacheDeleteRequest,
) (*cache.CacheDeleteResponse, error) {
	err := s.cache.Delete(ctx, s.prefixedKey(req.Key))
	if err != nil && !errors.Is(err, intcache.ErrNotFound) {
		return &cache.CacheDeleteResponse{
			Success: false,
		}, nil
	}

	return &cache.CacheDeleteResponse{
		Success: true,
	}, nil
}

type CacheHostLibrary struct {
	impl *CacheServiceImpl
}

func NewCacheHostLibrary(c intcache.Cache, keyPrefix string) *CacheHostLibrary {
	return &CacheHostLibrary{
		impl: NewCacheService(c, keyPrefix),
	}
}

func (l *CacheHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return cache.Instantiate(ctx, r, l.impl)
}
