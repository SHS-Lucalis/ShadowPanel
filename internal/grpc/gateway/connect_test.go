package gateway

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// setupAuthorizedNode saves a node with id derived from in-memory repo (1)
// and registers a matching apiKey on the fake verifier.
func setupAuthorizedNode(t *testing.T, deps *serviceDeps, apiKey string) {
	t.Helper()
	require.NoError(t, deps.nodeRepo.Save(context.Background(), &domain.Node{
		Enabled:       true,
		Name:          "n",
		GdaemonAPIKey: apiKey,
	}))
	deps.apiKeyVerifier.valid[apiKey] = 1
}

func TestService_Connect_authenticatesAndSendsRegisterAck(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	setupAuthorizedNode(t, deps, "secret")

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	stream := newStubConnectServer(ctx)

	// First incoming: a valid Register request.
	stream.PushMessage(&proto.DaemonMessage{
		RequestId: "register-1",
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{
				NodeId:       1,
				ApiKey:       "secret",
				Version:      "v1.0.0",
				Capabilities: []string{"http_proxy"},
			},
		},
	})

	// Wait for the ack to be sent then close stream so Connect returns.
	connectErrCh := make(chan error, 1)
	go func() {
		connectErrCh <- svc.Connect(stream)
	}()

	// Wait until the ack appears in stream.sent, then EOF the receive loop.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(stream.Sent()) > 0 {
			break
		}
		time.Sleep(2 * time.Millisecond)
	}

	require.Eventually(t, func() bool {
		return len(stream.Sent()) > 0
	}, time.Second, 5*time.Millisecond, "RegisterAck must be sent")

	// Close incoming so handleMessages returns.
	stream.CloseRecv()

	// ACT: wait for Connect to finish.
	var err error
	select {
	case err = <-connectErrCh:
	case <-time.After(2 * time.Second):
		t.Fatal("Connect did not return in time")
	}

	// ASSERT
	require.NoError(t, err)

	sent := stream.Sent()
	require.Len(t, sent, 1, "exactly one outbound message expected (RegisterAck)")
	assert.Equal(t, "register-1", sent[0].RequestId, "RequestId echoed from register")
	require.NotNil(t, sent[0].GetRegisterAck(), "payload must be RegisterAck")
	assert.True(t, sent[0].GetRegisterAck().Success)

	// Session must have been unregistered on Connect return.
	assert.False(t, deps.registry.IsConnected(1), "session must be unregistered on Connect return")
}

func TestService_Connect_invalidFirstMessageReturnsError(t *testing.T) {
	t.Run("recv_error_returns_invalid_argument", func(t *testing.T) {
		// ARRANGE
		svc, _ := newServiceWithDeps(t)
		stream := newStubConnectServer(context.Background())
		stream.CloseRecv() // immediate EOF on Recv

		// ACT
		err := svc.Connect(stream)

		// ASSERT
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "failed to receive registration message")
	})

	t.Run("non_register_first_message_returns_invalid_argument", func(t *testing.T) {
		// ARRANGE
		svc, _ := newServiceWithDeps(t)
		stream := newStubConnectServer(context.Background())
		stream.PushMessage(&proto.DaemonMessage{
			Payload: &proto.DaemonMessage_Heartbeat{Heartbeat: &proto.Heartbeat{}},
		})

		// ACT
		err := svc.Connect(stream)

		// ASSERT
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.InvalidArgument, st.Code())
		assert.Contains(t, st.Message(), "first message must be RegisterRequest")
	})
}

func TestService_Connect_invalidAuthRejectsConnection(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	require.NoError(t, deps.nodeRepo.Save(context.Background(), &domain.Node{
		Enabled:       true,
		Name:          "n",
		GdaemonAPIKey: "real",
	}))
	deps.apiKeyVerifier.valid["real"] = 1

	stream := newStubConnectServer(context.Background())
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{NodeId: 1, ApiKey: "wrong"},
		},
	})

	// ACT
	err := svc.Connect(stream)

	// ASSERT
	require.Error(t, err)
	st, ok := status.FromError(err)
	require.True(t, ok)
	assert.Equal(t, codes.Unauthenticated, st.Code())

	assert.Empty(t, stream.Sent(), "no ack must be sent on auth failure")
	assert.False(t, deps.registry.IsConnected(1), "session must not be registered when auth fails")
}

func TestService_Connect_handleMessagesProcessesIncomingThenReturnsOnEOF(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	setupAuthorizedNode(t, deps, "k")

	stream := newStubConnectServer(context.Background())
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{NodeId: 1, ApiKey: "k"},
		},
	})
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_TaskStatus{
			TaskStatus: &proto.TaskStatusUpdate{TaskId: 42, Status: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS},
		},
	})

	connectErrCh := make(chan error, 1)
	go func() {
		connectErrCh <- svc.Connect(stream)
	}()

	// Wait until the ack is sent and the task status processed.
	require.Eventually(t, func() bool {
		return len(stream.Sent()) > 0 && len(deps.taskHandler.StatusUpdates()) > 0
	}, 2*time.Second, 5*time.Millisecond, "ack and task status update must be observed")

	// Trigger Connect return.
	stream.CloseRecv()

	// ACT/ASSERT
	select {
	case err := <-connectErrCh:
		require.NoError(t, err, "Connect must return cleanly when EOF reached")
	case <-time.After(2 * time.Second):
		t.Fatal("Connect did not return after EOF")
	}

	updates := deps.taskHandler.StatusUpdates()
	require.Len(t, updates, 1)
	assert.Equal(t, uint64(42), updates[0].TaskId)
}

func TestService_Connect_recvErrorPropagatesNonEOF(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	setupAuthorizedNode(t, deps, "k")

	stream := newStubConnectServer(context.Background())
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{NodeId: 1, ApiKey: "k"},
		},
	})
	stream.PushError(io.ErrClosedPipe)

	// ACT
	err := svc.Connect(stream)

	// ASSERT
	require.Error(t, err, "non-EOF stream errors must surface")
	assert.ErrorIs(t, err, io.ErrClosedPipe)
}

func TestService_Connect_eofAfterRegisterClosesGracefully(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	setupAuthorizedNode(t, deps, "k")

	stream := newStubConnectServer(context.Background())
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{NodeId: 1, ApiKey: "k"},
		},
	})
	stream.CloseRecv()

	// ACT
	err := svc.Connect(stream)

	// ASSERT
	require.NoError(t, err, "EOF after register must be a clean shutdown")
}

func TestService_Connect_shutdownContextCancelsSession(t *testing.T) {
	// ARRANGE
	svc, deps := newServiceWithDeps(t)
	setupAuthorizedNode(t, deps, "k")

	shutdownCtx, shutdownCancel := context.WithCancel(context.Background())
	svc.SetShutdownContext(shutdownCtx)

	stream := newStubConnectServer(context.Background())
	stream.PushMessage(&proto.DaemonMessage{
		Payload: &proto.DaemonMessage_Register{
			Register: &proto.RegisterRequest{NodeId: 1, ApiKey: "k"},
		},
	})

	connectErrCh := make(chan error, 1)
	go func() {
		connectErrCh <- svc.Connect(stream)
	}()

	require.Eventually(t, func() bool {
		return len(stream.Sent()) > 0
	}, 2*time.Second, 5*time.Millisecond, "ack must be sent before shutdown")

	// ACT: trigger application-wide shutdown.
	shutdownCancel()

	// ASSERT: Connect returns cleanly.
	select {
	case err := <-connectErrCh:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		t.Fatal("Connect did not honour shutdownCtx cancellation")
	}
}

func TestService_SetShutdownContext_nilDefaultsToBackground(t *testing.T) {
	// ARRANGE
	svc, _ := newServiceWithDeps(t)
	var nilCtx context.Context

	// ACT
	svc.SetShutdownContext(nilCtx)

	// ASSERT
	require.NotNil(t, svc.shutdownCtx, "nil ctx must be replaced with background")
	assert.NoError(t, svc.shutdownCtx.Err(), "background context must not have an error")
}
