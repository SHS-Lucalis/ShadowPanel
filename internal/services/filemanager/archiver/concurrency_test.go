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
}
