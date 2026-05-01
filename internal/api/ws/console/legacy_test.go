package console

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/ws"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialedConsoleClient is a real ws.Client whose underlying socket is the
// server side of a WebSocket connection that the test holds the client side
// of. Modeled on metricsbase/pump_test.go's dialedClient.
type dialedConsoleClient struct {
	srvClient *ws.Client
	cliConn   *websocket.Conn
	httpSrv   *httptest.Server
	hub       *ws.Hub
}

// dialConsoleClient stands up an httptest server that accepts a WebSocket,
// wraps the server side in a ws.Client, dials it from the client side, and
// runs the client read/write pumps in the background. Cleanup tears
// everything down via t.Cleanup.
func dialConsoleClient(t *testing.T) *dialedConsoleClient {
	t.Helper()

	hub := ws.NewHub(silentLogger())

	type accepted struct {
		client *ws.Client
		err    error
	}
	acceptedCh := make(chan accepted, 1)

	httpSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := ws.Accept(w, r, nil)
		if err != nil {
			acceptedCh <- accepted{nil, err}

			return
		}
		client := ws.NewClient(r.Context(), conn, hub, nil, silentLogger())
		hub.Register(client)
		acceptedCh <- accepted{client, nil}
		client.Run()
	}))

	dialCtx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	wsURL := "ws" + strings.TrimPrefix(httpSrv.URL, "http")
	cliConn, resp, err := websocket.Dial(dialCtx, wsURL, nil)
	require.NoError(t, err)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}

	got := <-acceptedCh
	require.NoError(t, got.err, "server-side accept must succeed")
	require.NotNil(t, got.client)

	d := &dialedConsoleClient{
		srvClient: got.client,
		cliConn:   cliConn,
		httpSrv:   httpSrv,
		hub:       hub,
	}
	t.Cleanup(func() {
		_ = cliConn.Close(websocket.StatusNormalClosure, "")
		got.client.Close()
		httpSrv.Close()
	})

	return d
}

// consoleFrame is the JSON envelope read off the wire.
type consoleFrame struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
	Error   string          `json:"error,omitempty"`
	Ts      int64           `json:"ts"`
}

func readConsoleFrame(t *testing.T, c *websocket.Conn, timeout time.Duration) (consoleFrame, bool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	_, data, err := c.Read(ctx)
	if err != nil {
		return consoleFrame{}, false
	}

	var f consoleFrame
	require.NoError(t, json.Unmarshal(data, &f))

	return f, true
}

// expectNoConsoleFrame asserts no frame arrives within timeout.
func expectNoConsoleFrame(t *testing.T, c *websocket.Conn, timeout time.Duration) {
	t.Helper()

	if extra, ok := readConsoleFrame(t, c, timeout); ok {
		t.Fatalf("expected no frame, got type=%q payload=%q", extra.Type, string(extra.Payload))
	}
}

// callMessageHandler invokes the raw inbound dispatch path so the handler
// closure runs as if a frame arrived. Tests use it to drive both legacy and
// gRPC paths without bouncing through the network read pump.
func callMessageHandler(t *testing.T, handler ws.MessageHandler, msgType string, payload any) {
	t.Helper()

	raw, err := json.Marshal(payload)
	require.NoError(t, err, "test payload must be JSON-marshallable")
	handler(context.Background(), &ws.InboundMessage{
		Type:    msgType,
		Payload: raw,
	})
}

// ---------- newLegacyMessageHandler ----------

func TestHandler_NewLegacyMessageHandler(t *testing.T) {
	tests := []struct {
		name string
		// canSend controls whether the user is allowed to send commands.
		canSend bool
		// rbac is the RBAC backend used to (re)check the ability inside the
		// handler before the command is dispatched. allowAllRBAC{} = success,
		// denyAllRBAC{} = failure.
		rbac base.RBAC
		// msgType is the inbound message type sent to the handler.
		msgType string
		// payloadJSON is the raw inbound payload (any so we can pass invalid types).
		payload any
		// nodeScript is the script_send_command value of the node, or nil to
		// fall back to the input.txt upload path.
		nodeScript *string
		// daemonErr is the error returned from daemonCommands.ExecuteCommand.
		daemonErr error
		// uploadErr is the error returned from fileService.Upload.
		uploadErr error

		// Expected observable outcome:
		wantWireFrameType  string // empty = no frame expected
		wantWireFrameError string // expected payload.message or top-level error
		wantDaemonCalls    int32
		wantUploadCalls    int32
	}{
		{
			name:    "non_command_message_type_is_ignored",
			canSend: true,
			rbac:    allowAllRBAC{},
			msgType: "console.history",
			payload: consoleCommandPayload{Command: "say hello"},
			// no daemon call, no frame
		},
		{
			name:               "user_without_send_permission_gets_error_frame_and_no_dispatch",
			canSend:            false,
			rbac:               allowAllRBAC{}, // unused: outer guard fires first
			msgType:            typeConsoleCommand,
			payload:            consoleCommandPayload{Command: "say denied"},
			wantWireFrameType:  ws.TypeError,
			wantWireFrameError: "permission denied: cannot send commands",
		},
		{
			name:    "malformed_payload_is_silently_dropped",
			canSend: true,
			rbac:    allowAllRBAC{},
			msgType: typeConsoleCommand,
			// Strings cannot decode into consoleCommandPayload (struct), so
			// json.Unmarshal returns an error and the handler returns silently.
			payload: "not-an-object",
		},
		{
			name:    "empty_command_string_is_silently_dropped",
			canSend: true,
			rbac:    allowAllRBAC{},
			msgType: typeConsoleCommand,
			payload: consoleCommandPayload{Command: ""},
		},
		{
			name:               "rbac_recheck_failure_returns_error_frame_and_no_dispatch",
			canSend:            true,
			rbac:               denyAllRBAC{},
			msgType:            typeConsoleCommand,
			payload:            consoleCommandPayload{Command: "say re-denied"},
			wantWireFrameType:  ws.TypeError,
			wantWireFrameError: "permission denied: cannot send commands",
		},
		{
			name:            "script_path_executes_command_with_replaced_shortcodes",
			canSend:         true,
			rbac:            allowAllRBAC{},
			msgType:         typeConsoleCommand,
			payload:         consoleCommandPayload{Command: "say hi"},
			nodeScript:      new("echo {command} > {dir}/input.txt"),
			wantDaemonCalls: 1,
		},
		{
			name:               "script_failure_emits_error_frame",
			canSend:            true,
			rbac:               allowAllRBAC{},
			msgType:            typeConsoleCommand,
			payload:            consoleCommandPayload{Command: "say boom"},
			nodeScript:         new("./send.sh '{command}'"),
			daemonErr:          errors.New("daemon offline"),
			wantDaemonCalls:    1,
			wantWireFrameType:  ws.TypeError,
			wantWireFrameError: "failed to send command",
		},
		{
			name:            "no_script_uploads_command_to_input_txt",
			canSend:         true,
			rbac:            allowAllRBAC{},
			msgType:         typeConsoleCommand,
			payload:         consoleCommandPayload{Command: "say upload"},
			wantUploadCalls: 1,
		},
		{
			name:               "upload_failure_emits_error_frame",
			canSend:            true,
			rbac:               allowAllRBAC{},
			msgType:            typeConsoleCommand,
			payload:            consoleCommandPayload{Command: "say up-fail"},
			uploadErr:          errors.New("permission denied"),
			wantUploadCalls:    1,
			wantWireFrameType:  ws.TypeError,
			wantWireFrameError: "failed to send command",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			d := dialConsoleClient(t)

			dc := &fakeDaemonCommands{
				result: &daemon.CommandResult{Output: ""},
				err:    tt.daemonErr,
			}
			fs := &fakeFileService{uploadErr: tt.uploadErr}

			h := &Handler{
				abilityChecker: newAbilityCheckerWithRBAC(tt.rbac),
				daemonCommands: dc,
				fileService:    fs,
				logger:         silentLogger(),
			}

			server := newTestServer()
			node := newTestNode(nil)
			node.ScriptSendCommand = tt.nodeScript
			user := &domain.User{ID: 1}

			handler := h.newLegacyMessageHandler(
				context.Background(), d.srvClient, server, node, user, tt.canSend,
			)

			// ACT
			callMessageHandler(t, handler, tt.msgType, tt.payload)

			// ASSERT — wire side first, since failures there are the most useful signal.
			if tt.wantWireFrameType != "" {
				frame, ok := readConsoleFrame(t, d.cliConn, time.Second)
				require.True(t, ok, "expected a frame, got read timeout")
				assert.Equal(t, tt.wantWireFrameType, frame.Type, "frame type mismatch")

				if tt.wantWireFrameError != "" {
					var payload struct {
						Message string `json:"message"`
					}
					require.NoError(t, json.Unmarshal(frame.Payload, &payload))
					assert.Equal(t, tt.wantWireFrameError, payload.Message,
						"error message mismatch")
				}
			} else {
				expectNoConsoleFrame(t, d.cliConn, 100*time.Millisecond)
			}

			assert.Equal(t, tt.wantDaemonCalls, dc.calls.Load(),
				"daemonCommands.ExecuteCommand call count")
			assert.Equal(t, tt.wantUploadCalls, fs.uploadCalls.Load(),
				"fileService.Upload call count")

			if tt.wantUploadCalls > 0 {
				assert.Equal(t, "/srv/gs/test/input.txt", fs.uploadPath,
					"input.txt path must be serverDir/input.txt")
				if cmd, ok := tt.payload.(consoleCommandPayload); ok {
					assert.Equal(t, []byte(cmd.Command), fs.uploadContent,
						"uploaded content must match the requested command")
				}
			}
		})
	}
}

// ---------- legacyPoller ----------

func TestLegacyPoller_Poll_emitsDiffOnAppend(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadResult: []byte("first\n")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	// Seed the lastContent so the next poll observes an append, mirroring the
	// poller's first run priming behavior.
	poller.poll(context.Background())
	first, ok := readConsoleFrame(t, d.cliConn, time.Second)
	require.True(t, ok, "expected the initial seed to be sent as a console.output frame")
	assert.Equal(t, typeConsoleOutput, first.Type)

	// ACT — append more bytes. Poller should send only the new tail.
	fs.downloadResult = []byte("first\nsecond\n")
	poller.poll(context.Background())

	// ASSERT
	frame, ok := readConsoleFrame(t, d.cliConn, time.Second)
	require.True(t, ok, "expected an append frame after content grew")
	assert.Equal(t, typeConsoleOutput, frame.Type)

	var payload struct {
		Chunk string `json:"chunk"`
	}
	require.NoError(t, json.Unmarshal(frame.Payload, &payload))
	assert.Equal(t, "second\n", payload.Chunk, "only the appended suffix must be emitted")
}

func TestLegacyPoller_Poll_emitsFullContentOnReplace(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadResult: []byte("aaa\nbbb\n")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	poller.poll(context.Background())
	first, ok := readConsoleFrame(t, d.cliConn, time.Second)
	require.True(t, ok)
	assert.Equal(t, typeConsoleOutput, first.Type)

	// ACT — replace content with something that does NOT have lastContent as
	// a prefix; the poller must emit the entire new content as the chunk.
	fs.downloadResult = []byte("xxx\n")
	poller.poll(context.Background())

	// ASSERT
	frame, ok := readConsoleFrame(t, d.cliConn, time.Second)
	require.True(t, ok)

	var payload struct {
		Chunk string `json:"chunk"`
	}
	require.NoError(t, json.Unmarshal(frame.Payload, &payload))
	assert.Equal(t, "xxx\n", payload.Chunk,
		"non-prefix change must emit the full new content as the chunk")
}

func TestLegacyPoller_Poll_noChange_emitsNothing(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadResult: []byte("constant content\n")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	poller.poll(context.Background())
	first, ok := readConsoleFrame(t, d.cliConn, time.Second)
	require.True(t, ok, "first poll should emit the initial content")
	assert.Equal(t, typeConsoleOutput, first.Type)

	// ACT — same content, second poll should be a no-op.
	poller.poll(context.Background())

	// ASSERT
	expectNoConsoleFrame(t, d.cliConn, 150*time.Millisecond)
}

func TestLegacyPoller_Poll_downloadError_isSwallowed(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadErr: errors.New("io error")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	// ACT
	poller.poll(context.Background())

	// ASSERT — no frame emitted, no panic, lastContent unchanged.
	expectNoConsoleFrame(t, d.cliConn, 150*time.Millisecond)
	assert.Empty(t, poller.lastContent)
}

func TestLegacyPoller_Run_exitsOnContextCancel(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadResult: []byte("payload\n")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})

	// ACT
	go func() {
		poller.run(ctx)
		close(done)
	}()

	cancel()

	// ASSERT — run must return promptly after ctx cancel.
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("legacyPoller.run did not return after context cancel")
	}
}

func TestLegacyPoller_Run_exitsOnClientDone(t *testing.T) {
	// ARRANGE
	d := dialConsoleClient(t)

	fs := &fakeFileService{downloadResult: []byte("payload\n")}
	node := newTestNode(nil)
	poller := newLegacyPoller(d.srvClient, fs, node, "/srv/gs/test", silentLogger())

	done := make(chan struct{})

	// ACT
	go func() {
		poller.run(context.Background())
		close(done)
	}()

	d.srvClient.Close() // signals client.Done()

	// ASSERT
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("legacyPoller.run did not return after client close")
	}
}
