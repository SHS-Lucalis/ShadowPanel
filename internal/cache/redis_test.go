// Redis error-injection tests are not implemented because miniredis is not in go.mod; adding it would require a new dependency.

package cache_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/cache"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

func TestRedisCache(t *testing.T) {
	testRedisAddr := os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		t.Skip("Skipping Redis cache tests because TEST_REDIS_ADDR is not set")
	}

	testRedisPassword := os.Getenv("TEST_REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr,
		Password: testRedisPassword,
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping Redis cache tests because Redis is not available: %v", err)
	}

	suite.Run(t, cache.NewCacheSuite(
		func(_ *testing.T) cache.Cache {
			c := cache.NewRedisFromClient(client)
			_ = c.Clear(context.Background())

			return c
		},
	))
}

func TestRedisCacheSpecificFeatures(t *testing.T) {
	testRedisAddr := os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		t.Skip("Skipping Redis cache tests because TEST_REDIS_ADDR is not set")
	}

	testRedisPassword := os.Getenv("TEST_REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr,
		Password: testRedisPassword,
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		t.Skipf("Skipping Redis cache tests because Redis is not available: %v", err)
	}

	c := cache.NewRedisFromClient(client)
	ctx = context.Background()

	_ = c.Clear(ctx)

	t.Run("delete_pattern", func(t *testing.T) {
		err := c.Set(ctx, "pattern:key1", "value1")
		require.NoError(t, err)

		err = c.Set(ctx, "pattern:key2", "value2")
		require.NoError(t, err)

		err = c.Set(ctx, "pattern:key3", "value3")
		require.NoError(t, err)

		err = c.Set(ctx, "other_key", "other_value")
		require.NoError(t, err)

		err = c.DeletePattern(ctx, "pattern:*")
		require.NoError(t, err)

		_, err = c.Get(ctx, "pattern:key1")
		require.ErrorIs(t, err, cache.ErrNotFound)

		_, err = c.Get(ctx, "pattern:key2")
		require.ErrorIs(t, err, cache.ErrNotFound)

		_, err = c.Get(ctx, "pattern:key3")
		require.ErrorIs(t, err, cache.ErrNotFound)

		value, err := c.Get(ctx, "other_key")
		require.NoError(t, err)
		require.Equal(t, "other_value", value)
	})

	t.Run("get_typed", func(t *testing.T) {
		type testStruct struct {
			Name  string
			Count int
		}

		original := testStruct{
			Name:  "test",
			Count: 42,
		}

		err := c.Set(ctx, "typed_key", original)
		require.NoError(t, err)

		retrieved, err := cache.GetTyped[testStruct](ctx, c, "typed_key")
		require.NoError(t, err)
		require.Equal(t, original.Name, retrieved.Name)
		require.Equal(t, original.Count, retrieved.Count)
	})

	t.Run("set_with_ttl", func(t *testing.T) {
		ttl := 100 * time.Millisecond
		err := cache.SetWithTTL(ctx, c, "ttl_key", "ttl_value", ttl)
		require.NoError(t, err)

		value, err := c.Get(ctx, "ttl_key")
		require.NoError(t, err)
		require.Equal(t, "ttl_value", value)

		time.Sleep(ttl + 50*time.Millisecond)

		value, err = c.Get(ctx, "ttl_key")
		require.ErrorIs(t, err, cache.ErrNotFound)
		require.Nil(t, value)
	})

	_ = c.Clear(ctx)
}

func BenchmarkRedisCache_Set(b *testing.B) {
	testRedisAddr := os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		b.Skip("Skipping Redis cache benchmarks because TEST_REDIS_ADDR is not set")
	}

	testRedisPassword := os.Getenv("TEST_REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr,
		Password: testRedisPassword,
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		b.Skipf("Skipping Redis cache benchmarks because Redis is not available: %v", err)
	}

	c := cache.NewRedisFromClient(client)
	_ = c.Clear(context.Background())

	ctx = context.Background()

	b.ResetTimer()
	for range b.N {
		key := "bench_key_" + string(rune(b.N%1000))
		_ = c.Set(ctx, key, "benchmark_value")
	}
}

func BenchmarkRedisCache_Get(b *testing.B) {
	testRedisAddr := os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		b.Skip("Skipping Redis cache benchmarks because TEST_REDIS_ADDR is not set")
	}

	testRedisPassword := os.Getenv("TEST_REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr,
		Password: testRedisPassword,
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		b.Skipf("Skipping Redis cache benchmarks because Redis is not available: %v", err)
	}

	c := cache.NewRedisFromClient(client)
	_ = c.Clear(context.Background())

	ctx = context.Background()

	for i := range 1000 {
		key := "bench_key_" + string(rune(i))
		_ = c.Set(ctx, key, "benchmark_value")
	}

	b.ResetTimer()
	for range b.N {
		key := "bench_key_" + string(rune(b.N%1000))
		_, _ = c.Get(ctx, key)
	}
}

func BenchmarkRedisCache_Delete(b *testing.B) {
	testRedisAddr := os.Getenv("TEST_REDIS_ADDR")
	if testRedisAddr == "" {
		b.Skip("Skipping Redis cache benchmarks because TEST_REDIS_ADDR is not set")
	}

	testRedisPassword := os.Getenv("TEST_REDIS_PASSWORD")

	client := redis.NewClient(&redis.Options{
		Addr:     testRedisAddr,
		Password: testRedisPassword,
		DB:       0,
	})
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		b.Skipf("Skipping Redis cache benchmarks because Redis is not available: %v", err)
	}

	c := cache.NewRedisFromClient(client)
	_ = c.Clear(context.Background())

	ctx = context.Background()

	b.ResetTimer()
	for i := range b.N {
		key := "bench_key_" + string(rune(i))
		_ = c.Set(ctx, key, "benchmark_value")
		_ = c.Delete(ctx, key)
	}
}
