package files

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
)

type CachingFileManager struct {
	inner FileManager
	cache FileManager
}

func NewCachingFileManager(inner, cache FileManager) *CachingFileManager {
	return &CachingFileManager{
		inner: inner,
		cache: cache,
	}
}

func (c *CachingFileManager) Read(ctx context.Context, path string) ([]byte, error) {
	cachePath := c.cachePath(path)

	if data, err := c.cache.Read(ctx, cachePath); err == nil {
		return data, nil
	}

	data, err := c.inner.Read(ctx, path)
	if err != nil {
		return nil, err
	}

	c.writeToCache(ctx, cachePath, data)

	return data, nil
}

func (c *CachingFileManager) Write(ctx context.Context, path string, data []byte) error {
	err := c.inner.Write(ctx, path, data)
	if err != nil {
		return err
	}

	cachePath := c.cachePath(path)
	_ = c.cache.Delete(ctx, cachePath)
	c.writeToCache(ctx, cachePath, data)

	return nil
}

func (c *CachingFileManager) Delete(ctx context.Context, path string) error {
	err := c.inner.Delete(ctx, path)
	if err != nil {
		return err
	}

	_ = c.cache.Delete(ctx, c.cachePath(path))

	return nil
}

func (c *CachingFileManager) Exists(ctx context.Context, path string) bool {
	return c.inner.Exists(ctx, path)
}

func (c *CachingFileManager) List(ctx context.Context, dir string) ([]string, error) {
	return c.inner.List(ctx, dir)
}

func (c *CachingFileManager) cachePath(path string) string {
	hash := sha256.Sum256([]byte(path))

	return hex.EncodeToString(hash[:])
}

func (c *CachingFileManager) writeToCache(ctx context.Context, cachePath string, data []byte) {
	if err := c.cache.Write(ctx, cachePath, data); err != nil {
		slog.Warn("failed to write cache", slog.String("error", err.Error()))
	}
}
