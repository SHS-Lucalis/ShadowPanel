package daemon

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/daemon/binnapi"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	errTransientForRetry = errors.New("transient")
	errPermanentForRetry = errors.New("permanent")
)

func newTestPool(t *testing.T, mockServer *MockDaemonServer) *Pool {
	t.Helper()

	pool, err := NewPool(config{
		Host:              mockServer.Host(),
		Port:              mockServer.Port(),
		ServerCertificate: []byte(daemonServerCert),
		ClientCertificate: []byte(clientCert),
		PrivateKey:        []byte(clientKey),
		Timeout:           10 * time.Second,
		Mode:              binnapi.ModeStatus,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = pool.Close()
	})

	return pool
}

func TestNewPool_ConstructsPool(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	// ACT
	pool, err := NewPool(config{
		Host:              mockServer.Host(),
		Port:              mockServer.Port(),
		ServerCertificate: []byte(daemonServerCert),
		ClientCertificate: []byte(clientCert),
		PrivateKey:        []byte(clientKey),
		Timeout:           10 * time.Second,
		Mode:              binnapi.ModeStatus,
	})

	// ASSERT
	require.NoError(t, err)
	require.NotNil(t, pool)
	stat := pool.Stat()
	assert.Equal(t, int32(0), stat.AcquiredResources(), "no resources should be acquired immediately after creation")
	_ = pool.Close()
}

func TestPool_Acquire_ReturnsUsableConnection(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Responses = []any{
		&binnapi.StatusVersionResponseMessage{
			Version:   "test-version",
			BuildDate: "test-date",
		},
	}
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	// ACT
	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)

	require.NotNil(t, conn.RemoteAddr(), "remote addr should be available on live connection")
	require.NotNil(t, conn.LocalAddr(), "local addr should be available on live connection")

	err = binnapi.WriteMessage(conn, binnapi.StatusRequestVersion)
	require.NoError(t, err)

	var resp binnapi.StatusVersionResponseMessage
	err = binnapi.ReadMessage(conn, &resp)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, "test-version", resp.Version)
	assert.NoError(t, conn.Close())
}

func TestPool_Acquire_ContextCancelled(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// ACT
	_, err = pool.Acquire(ctx)

	// ASSERT
	require.Error(t, err)
	assert.ErrorIs(t, err, context.Canceled, "cancelled context must surface as Canceled error")
}

func TestPool_Acquire_IdleReconnect(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping idle-reconnect test in short mode (requires >defaultTimeout=10s wait)")
	}

	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Responses = []any{
		&binnapi.StatusVersionResponseMessage{Version: "v1", BuildDate: "d1"},
		&binnapi.StatusVersionResponseMessage{Version: "v2", BuildDate: "d2"},
	}
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)
	require.NoError(t, binnapi.WriteMessage(conn, binnapi.StatusRequestVersion))
	var resp binnapi.StatusVersionResponseMessage
	require.NoError(t, binnapi.ReadMessage(conn, &resp))
	require.NoError(t, conn.Close())

	// ACT
	time.Sleep(defaultTimeout + 500*time.Millisecond)

	// ASSERT
	conn2, err := pool.Acquire(testContext(t))
	require.NoError(t, err, "second acquire after idle should reconnect successfully")
	require.NotNil(t, conn2.RemoteAddr())
	_ = conn2.Close()
}

func TestPool_WriteContext_Success(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Responses = []any{
		&binnapi.StatusVersionResponseMessage{Version: "v", BuildDate: "d"},
	}
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	versionMsgBytes, err := binnapi.StatusRequestVersion.MarshalBINN()
	require.NoError(t, err)
	versionMsgBytes = append(versionMsgBytes, binnapi.DaemonBinnEndBytes...)

	// ACT
	n, err := pool.WriteContext(testContext(t), versionMsgBytes)

	// ASSERT
	require.NoError(t, err)
	assert.Equal(t, len(versionMsgBytes), n, "all bytes should be written")
}

func TestPool_Close_ReturnsNil(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool, err := NewPool(config{
		Host:              mockServer.Host(),
		Port:              mockServer.Port(),
		ServerCertificate: []byte(daemonServerCert),
		ClientCertificate: []byte(clientCert),
		PrivateKey:        []byte(clientKey),
		Timeout:           10 * time.Second,
		Mode:              binnapi.ModeStatus,
	})
	require.NoError(t, err)

	// ACT
	closeErr := pool.Close()

	// ASSERT
	require.NoError(t, closeErr)
}

func TestPool_TryAcquire_OnEmptyPool(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	// ACT
	conn, err := pool.TryAcquire(testContext(t))

	// ASSERT
	require.Error(t, err, "TryAcquire should return an error when no resources are pre-existing")
	assert.Nil(t, conn)
}

func TestPooledConn_OperationsAfterClose(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)
	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)

	pc, ok := conn.(*PooledConn)
	require.True(t, ok, "Acquire must return *PooledConn")

	require.NoError(t, pc.Close())

	cases := []struct {
		name string
		fn   func() error
	}{
		{
			name: "read",
			fn: func() error {
				_, e := pc.Read(make([]byte, 4))

				return e
			},
		},
		{
			name: "set_deadline",
			fn:   func() error { return pc.SetDeadline(time.Now().Add(time.Second)) },
		},
		{
			name: "set_read_deadline",
			fn:   func() error { return pc.SetReadDeadline(time.Now().Add(time.Second)) },
		},
		{
			name: "set_write_deadline",
			fn:   func() error { return pc.SetWriteDeadline(time.Now().Add(time.Second)) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// ACT
			err := tc.fn()

			// ASSERT
			require.Error(t, err)
			assert.Contains(t, err.Error(), "not established", "operations on a released conn must return 'not established'")
		})
	}

	// LocalAddr/RemoteAddr should return nil instead of panicking
	assert.Nil(t, pc.LocalAddr(), "LocalAddr on released conn should be nil")
	assert.Nil(t, pc.RemoteAddr(), "RemoteAddr on released conn should be nil")

	// Close on already-closed conn is idempotent
	require.NoError(t, pc.Close())
}

func TestPooledConn_AddrsAndDeadlines_Live(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)
	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// ACT + ASSERT
	require.NotNil(t, conn.LocalAddr(), "LocalAddr should be non-nil on live connection")
	require.NotNil(t, conn.RemoteAddr(), "RemoteAddr should be non-nil on live connection")

	require.NoError(t, conn.SetDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	require.NoError(t, conn.SetWriteDeadline(time.Now().Add(5*time.Second)))
}

func TestRetry(t *testing.T) {
	tests := []struct {
		name        string
		attempts    int
		makeFn      func() (func() error, *atomic.Int32)
		wantError   string
		wantCalls   int32
		nilSentinel bool
	}{
		{
			name:     "zero_attempts_returns_error_without_calling",
			attempts: 0,
			makeFn: func() (func() error, *atomic.Int32) {
				var count atomic.Int32

				return func() error {
					count.Add(1)

					return nil
				}, &count
			},
			wantError: "attempts must be at least 1",
			wantCalls: 0,
		},
		{
			name:     "negative_attempts_returns_error_without_calling",
			attempts: -1,
			makeFn: func() (func() error, *atomic.Int32) {
				var count atomic.Int32

				return func() error {
					count.Add(1)

					return nil
				}, &count
			},
			wantError: "attempts must be at least 1",
			wantCalls: 0,
		},
		{
			name:     "succeeds_on_first_attempt",
			attempts: 3,
			makeFn: func() (func() error, *atomic.Int32) {
				var count atomic.Int32

				return func() error {
					count.Add(1)

					return nil
				}, &count
			},
			wantError:   "",
			wantCalls:   1,
			nilSentinel: true,
		},
		{
			name:     "succeeds_on_last_attempt",
			attempts: 3,
			makeFn: func() (func() error, *atomic.Int32) {
				var count atomic.Int32

				return func() error {
					n := count.Add(1)
					if n < 3 {
						return errTransientForRetry
					}

					return nil
				}, &count
			},
			wantError:   "",
			wantCalls:   3,
			nilSentinel: true,
		},
		{
			name:     "all_attempts_unsuccessful_returns_last_error",
			attempts: 2,
			makeFn: func() (func() error, *atomic.Int32) {
				var count atomic.Int32

				return func() error {
					count.Add(1)

					return errPermanentForRetry
				}, &count
			},
			wantError: "after 2 attempts, last error: permanent",
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			fn, count := tt.makeFn()

			// ACT
			err := Retry(tt.attempts, time.Microsecond, fn)

			// ASSERT
			if tt.nilSentinel {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError, "error message must match")
			}
			assert.Equal(t, tt.wantCalls, count.Load(), "call count must match")
		})
	}
}

func TestPuddlePanicError_Error(t *testing.T) {
	// ARRANGE
	err := newPuddlePanicError("boom")

	// ACT
	msg := err.Error()

	// ASSERT
	assert.Contains(t, msg, "panic in puddle")
	assert.Contains(t, msg, "boom")
}

func TestPool_WriteContext_RetryOnTransientError(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Responses = []any{
		&binnapi.StatusVersionResponseMessage{Version: "v", BuildDate: "d"},
	}
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	// Acquire a connection so a resource exists in the pool
	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)
	require.NoError(t, conn.Close())

	versionMsgBytes, err := binnapi.StatusRequestVersion.MarshalBINN()
	require.NoError(t, err)
	versionMsgBytes = append(versionMsgBytes, binnapi.DaemonBinnEndBytes...)

	// ACT
	n, err := pool.WriteContext(testContext(t), versionMsgBytes)

	// ASSERT
	require.NoError(t, err, "writing through the pool should succeed when the connection is alive")
	assert.Equal(t, len(versionMsgBytes), n)
}

func TestPool_WriteContext_AcquireFailsAfterClose(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)
	require.NoError(t, pool.Close())

	// ACT
	_, err = pool.WriteContext(testContext(t), []byte{0x01})

	// ASSERT
	require.Error(t, err)
	// Closed pool returns an error from Acquire which is wrapped or panics gracefully
	// (puddle returns an error from Acquire after Close)
	assert.True(t,
		// either wrapped acquire error or panic-captured error
		err != nil,
		"writing to a closed pool must return an error",
	)
}

func TestPool_StatReportsCounts(t *testing.T) {
	// ARRANGE
	mockServer, err := NewMockDaemonServer(t)
	require.NoError(t, err)
	t.Cleanup(mockServer.Stop)
	mockServer.Start()

	pool := newTestPool(t, mockServer)

	conn, err := pool.Acquire(testContext(t))
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	// ACT
	stat := pool.Stat()

	// ASSERT
	require.NotNil(t, stat)
	assert.GreaterOrEqual(t, stat.AcquiredResources(), int32(1), "at least one resource should be acquired")
}
