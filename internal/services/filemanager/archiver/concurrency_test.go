package archiver_test

import (
	"context"
	"sync"
	"testing"

	"github.com/gameap/gameap/internal/services/filemanager/archiver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryConcurrencyGuard(t *testing.T) {
	t.Parallel()

	t.Run("acquire_within_limit", func(t *testing.T) {
		t.Parallel()

		guard := archiver.NewInMemoryConcurrencyGuard(2)

		release1, err := guard.Acquire(context.Background(), 1)
		require.NoError(t, err)
		release2, err := guard.Acquire(context.Background(), 1)
		require.NoError(t, err)

		release1()
		release2()
	})

	t.Run("rejects_above_limit", func(t *testing.T) {
		t.Parallel()

		guard := archiver.NewInMemoryConcurrencyGuard(1)

		release, err := guard.Acquire(context.Background(), 7)
		require.NoError(t, err)

		_, err = guard.Acquire(context.Background(), 7)
		require.Error(t, err)
		assert.ErrorIs(t, err, archiver.ErrTooManyConcurrent)

		release()

		_, err = guard.Acquire(context.Background(), 7)
		require.NoError(t, err)
	})

	t.Run("limits_per_server_independently", func(t *testing.T) {
		t.Parallel()

		guard := archiver.NewInMemoryConcurrencyGuard(1)

		_, err := guard.Acquire(context.Background(), 1)
		require.NoError(t, err)
		_, err = guard.Acquire(context.Background(), 2)
		require.NoError(t, err)
	})

	t.Run("concurrent_acquire_is_safe", func(t *testing.T) {
		t.Parallel()

		guard := archiver.NewInMemoryConcurrencyGuard(5)

		var wg sync.WaitGroup
		var success, rejected int
		var mu sync.Mutex

		for range 50 {
			wg.Go(func() {
				rel, err := guard.Acquire(context.Background(), 99)
				mu.Lock()
				defer mu.Unlock()
				if err != nil {
					rejected++

					return
				}
				success++
				rel()
			})
		}

		wg.Wait()

		assert.GreaterOrEqual(t, success, 5, "at least limit-many should succeed eventually")
		assert.Equal(t, 50, success+rejected)
	})

	t.Run("default_limit_when_zero", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		guard := archiver.NewInMemoryConcurrencyGuard(0)

		// ACT
		release, err := guard.Acquire(context.Background(), 1)
		require.NoError(t, err)
		_, err2 := guard.Acquire(context.Background(), 1)

		// ASSERT
		assert.ErrorIs(t, err2, archiver.ErrTooManyConcurrent, "zero must default to limit=1")
		release()
	})

	t.Run("release_called_twice_is_safe", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		guard := archiver.NewInMemoryConcurrencyGuard(1)
		release, err := guard.Acquire(context.Background(), 5)
		require.NoError(t, err)

		// ACT
		release()
		assert.NotPanics(t, release, "second release must be a no-op")

		// ASSERT
		next, err := guard.Acquire(context.Background(), 5)
		require.NoError(t, err, "counter must not have gone negative — fresh acquire still works")
		next()
	})

	t.Run("repeated_acquire_release_does_not_leak", func(t *testing.T) {
		t.Parallel()

		// ARRANGE
		guard := archiver.NewInMemoryConcurrencyGuard(1)

		// ACT + ASSERT: 1000 cycles must each succeed (counter must reset each time)
		for range 1000 {
			release, err := guard.Acquire(context.Background(), 42)
			require.NoError(t, err)
			release()
		}
	})
}
