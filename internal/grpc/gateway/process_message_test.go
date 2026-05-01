package gateway

import (
	"context"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/grpc/session"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestService_processMessage(t *testing.T) {
	t.Run("heartbeat_updates_last_ping_and_returns_nil", func(t *testing.T) {
		// ARRANGE
		svc, _ := newServiceWithDeps(t)
		stream := newStubStream(context.Background())
		sess := session.NewSession(1, stream, "v", nil, func() {})
		before := sess.LastPing()
		time.Sleep(2 * time.Millisecond)

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_Heartbeat{Heartbeat: &proto.Heartbeat{}},
		})

		// ASSERT
		require.NoError(t, err)
		assert.True(t, sess.LastPing().After(before), "lastPing must advance on Heartbeat")
	})

	t.Run("task_status_routed_to_task_handler", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(7, newStubStream(context.Background()), "v", nil, func() {})
		update := &proto.TaskStatusUpdate{TaskId: 99, Status: proto.DaemonTaskStatus_DAEMON_TASK_STATUS_SUCCESS}

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_TaskStatus{TaskStatus: update},
		})

		// ASSERT
		require.NoError(t, err)
		got := deps.taskHandler.StatusUpdates()
		require.Len(t, got, 1)
		assert.Equal(t, uint64(99), got[0].TaskId)
	})

	t.Run("nil_task_handler_swallowed_for_task_status", func(t *testing.T) {
		// ARRANGE
		svc, _ := newServiceWithDeps(t)
		svc.taskHandler = nil
		sess := session.NewSession(7, newStubStream(context.Background()), "v", nil, func() {})

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_TaskStatus{TaskStatus: &proto.TaskStatusUpdate{TaskId: 1}},
		})

		// ASSERT
		require.NoError(t, err, "nil handler must be safely ignored")
	})

	t.Run("task_output_routed_to_task_handler", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_TaskOutput{TaskOutput: &proto.TaskOutput{TaskId: 5, OutputChunk: []byte("o")}},
		})

		// ASSERT
		require.NoError(t, err)
		out := deps.taskHandler.Outputs()
		require.Len(t, out, 1)
		assert.Equal(t, uint64(5), out[0].TaskId)
	})

	t.Run("command_output_routed_to_command_handler", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_CommandOutput{
				CommandOutput: &proto.CommandOutput{CommandId: "c-1", OutputChunk: []byte("hi")},
			},
		})

		// ASSERT
		require.NoError(t, err)
		out := deps.commandHandler.Outputs()
		require.Len(t, out, 1)
		assert.Equal(t, "c-1", out[0].CommandId)
	})

	t.Run("command_result_resolves_pending_request_and_calls_handler", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("req-1")

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_CommandResult{
				CommandResult: &proto.CommandResult{RequestId: "req-1", ExitCode: 0},
			},
		})

		// ASSERT
		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg, "pending request must receive resolution")
			require.NotNil(t, msg.GetCommandResult())
			assert.Equal(t, int32(0), msg.GetCommandResult().ExitCode)
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for pending request resolution")
		}

		results := deps.commandHandler.Results()
		require.Len(t, results, 1, "handler must still be invoked even after resolution")
	})

	t.Run("server_statuses_routed_to_handler", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_ServerStatuses{
				ServerStatuses: &proto.ServerStatusBatch{},
			},
		})

		// ASSERT
		require.NoError(t, err)
		batches := deps.serverHandler.Batches()
		require.Len(t, batches, 1)
	})

	t.Run("file_read_response_resolves_pending_request", func(t *testing.T) {
		// ARRANGE
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("file-read-1")

		// ACT
		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_FileReadResponse{
				FileReadResponse: &proto.FileReadResponse{RequestId: "file-read-1", Success: true},
			},
		})

		// ASSERT
		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg)
			require.NotNil(t, msg.GetFileReadResponse())
			assert.True(t, msg.GetFileReadResponse().Success)
		case <-time.After(time.Second):
			t.Fatal("file_read pending request not resolved")
		}
	})

	t.Run("file_write_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("fw-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_FileWriteResponse{
				FileWriteResponse: &proto.FileWriteResponse{RequestId: "fw-1", Success: true},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetFileWriteResponse())
		case <-time.After(time.Second):
			t.Fatal("file_write not resolved")
		}
	})

	t.Run("file_list_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("fl-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_FileListResponse{
				FileListResponse: &proto.FileListResponse{RequestId: "fl-1"},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetFileListResponse())
		case <-time.After(time.Second):
			t.Fatal("file_list not resolved")
		}
	})

	t.Run("file_operation_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("fo-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_FileOperationResponse{
				FileOperationResponse: &proto.FileOperationResponse{RequestId: "fo-1", Success: true},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetFileOperationResponse())
		case <-time.After(time.Second):
			t.Fatal("file_operation not resolved")
		}
	})

	t.Run("status_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("st-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_StatusResponse{
				StatusResponse: &proto.StatusResponse{RequestId: "st-1"},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetStatusResponse())
		case <-time.After(time.Second):
			t.Fatal("status not resolved")
		}
	})

	t.Run("console_log_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("cl-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_ConsoleLogResponse{
				ConsoleLogResponse: &proto.ConsoleLogResponse{RequestId: "cl-1"},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetConsoleLogResponse())
		case <-time.After(time.Second):
			t.Fatal("console_log not resolved")
		}
	})

	t.Run("http_proxy_response_resolves_pending_request", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
		ch := sess.RegisterPendingRequest("hp-1")

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_HttpProxyResponse{
				HttpProxyResponse: &proto.HTTPProxyResponse{RequestId: "hp-1"},
			},
		})

		require.NoError(t, err)
		select {
		case msg := <-ch:
			require.NotNil(t, msg.GetHttpProxyResponse())
		case <-time.After(time.Second):
			t.Fatal("http_proxy not resolved")
		}
	})

	t.Run("attach_started_routed_to_attach_handler", func(t *testing.T) {
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_AttachStarted{AttachStarted: &proto.AttachStarted{}},
		})

		require.NoError(t, err)
		deps.attachHandler.mu.Lock()
		defer deps.attachHandler.mu.Unlock()
		require.Len(t, deps.attachHandler.started, 1)
	})

	t.Run("attach_output_routed_to_attach_handler", func(t *testing.T) {
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_AttachOutput{AttachOutput: &proto.AttachOutput{}},
		})

		require.NoError(t, err)
		deps.attachHandler.mu.Lock()
		defer deps.attachHandler.mu.Unlock()
		require.Len(t, deps.attachHandler.outputs, 1)
	})

	t.Run("attach_closed_routed_to_attach_handler", func(t *testing.T) {
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			Payload: &proto.DaemonMessage_AttachClosed{AttachClosed: &proto.AttachClosed{}},
		})

		require.NoError(t, err)
		deps.attachHandler.mu.Lock()
		defer deps.attachHandler.mu.Unlock()
		require.Len(t, deps.attachHandler.closed, 1)
	})

	t.Run("metrics_response_routed_to_metrics_handler", func(t *testing.T) {
		svc, deps := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			RequestId: "mr-1",
			Payload:   &proto.DaemonMessage_MetricsResponse{MetricsResponse: &proto.MetricsResponse{}},
		})

		require.NoError(t, err)
		got := deps.metricsHandler.Responses()
		require.Len(t, got, 1)
	})

	t.Run("unknown_payload_returns_nil_without_panic", func(t *testing.T) {
		svc, _ := newServiceWithDeps(t)
		sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})

		err := svc.processMessage(context.Background(), sess, &proto.DaemonMessage{
			RequestId: "x",
			Payload:   nil,
		})

		require.NoError(t, err, "nil/unknown payload must be a no-op")
	})
}

func TestResolveResponse_alwaysReturnsNilAndDispatchesToSession(t *testing.T) {
	// ARRANGE
	sess := session.NewSession(1, newStubStream(context.Background()), "v", nil, func() {})
	ch := sess.RegisterPendingRequest("any-id")
	msg := &proto.DaemonMessage{RequestId: "any-id"}

	// ACT
	err := resolveResponse(sess, "any-id", msg)

	// ASSERT
	require.NoError(t, err)
	select {
	case got := <-ch:
		assert.Same(t, msg, got)
	case <-time.After(time.Second):
		t.Fatal("resolveResponse must deliver to channel")
	}
}
