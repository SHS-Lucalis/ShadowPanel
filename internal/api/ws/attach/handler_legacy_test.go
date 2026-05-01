package attach

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/daemon"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/ws"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// errDiskFull is a static sentinel used by the upload-failure test so the
// err113 linter does not complain about a one-off dynamic errors.New.
var errDiskFull = errors.New("disk full")

// TestRunLegacyMode_sendsAttachStartedAndDownloadsHistory verifies that on
// connect the legacy path emits an attach.started frame and downloads the
// console history file via the file service.
func TestRunLegacyMode_sendsAttachStartedAndDownloadsHistory(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	env.files.downloadResp = []byte("previous output")

	// ACT
	conn := env.dial(t)
	require.NotNil(t, conn)

	started, ok := readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)
	require.True(t, ok, "client must receive attach.started frame on legacy connect")

	var sp attachStartedPayload
	require.NoError(t, json.Unmarshal(started.Payload, &sp))

	// ASSERT
	assert.NotEmpty(t, sp.SessionID, "session id must be generated")
	assert.Equal(t, uint64(env.server.ID), sp.ServerID, "server id must be carried")

	output, ok := readFrameOfType(t, conn, typeAttachOutput, 2*time.Second)
	require.True(t, ok, "history download must surface as an attach.output frame")

	var op attachOutputPayload
	require.NoError(t, json.Unmarshal(output.Payload, &op))
	assert.Equal(t, []byte("previous output"), op.Data, "history bytes must be relayed unchanged")

	require.Eventually(t, func() bool {
		for _, c := range env.files.downloadList() {
			if strings.HasSuffix(c.Path, "/output.txt") {
				return true
			}
		}

		return false
	}, time.Second, 10*time.Millisecond, "history must be fetched via Download(output.txt)")
}

// TestRunLegacyMode_inputUploadedToInputFile verifies that an attach.input
// frame is uploaded to <serverDir>/input.txt via the file service when the
// node has no script_send_command configured.
func TestRunLegacyMode_inputUploadedToInputFile(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	conn := env.dial(t)

	// Drain the initial frames so the next read is the one we care about.
	_, _ = readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)

	// ACT
	writeJSONFrame(t, conn, &ws.InboundMessage{
		Type:    typeAttachInput,
		Payload: rawPayload(t, attachInputPayload{Data: []byte("status\n")}),
	})

	// ASSERT
	require.Eventually(t, func() bool {
		for _, c := range env.files.uploadList() {
			if strings.HasSuffix(c.Path, "/input.txt") && string(c.Data) == "status\n" {
				return true
			}
		}

		return false
	}, 2*time.Second, 10*time.Millisecond, "input must be uploaded to input.txt with the same bytes")
}

// TestRunLegacyMode_inputViaScriptSendCommand verifies that when the node
// has script_send_command set, the daemon command service is used instead
// of the file service.
func TestRunLegacyMode_inputViaScriptSendCommand(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	cmd := "tmux send-keys '{command}'"
	env.setNodeScriptSendCommand(t, cmd)
	env.files.downloadResp = nil

	conn := env.dial(t)
	_, _ = readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)

	// ACT
	writeJSONFrame(t, conn, &ws.InboundMessage{
		Type:    typeAttachInput,
		Payload: rawPayload(t, attachInputPayload{Data: []byte("hello")}),
	})

	// ASSERT — daemonCommands.ExecuteCommand must be invoked with substituted command
	require.Eventually(t, func() bool {
		for _, c := range env.dCmds.executedCommands() {
			if strings.Contains(c.Command, "hello") && c.NodeID == env.node.ID {
				return true
			}
		}

		return false
	}, 2*time.Second, 10*time.Millisecond, "send-command script must run with substituted input")

	assert.Empty(t, env.files.uploadList(),
		"file service Upload must not be called when script_send_command is set")
}

// TestRunLegacyMode_inputDeniedWhenCanSendFalse verifies that input is
// rejected with an error frame when the per-server send permission is not
// granted, and no daemon-command or upload side effects occur.
func TestRunLegacyMode_inputDeniedWhenCanSendFalse(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	env.rbac.isAdmin = false
	env.rbac.allowAbility(domain.AbilityNameGameServerConsoleView)

	conn := env.dial(t)
	_, _ = readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)

	// ACT
	writeJSONFrame(t, conn, &ws.InboundMessage{
		Type:    typeAttachInput,
		Payload: rawPayload(t, attachInputPayload{Data: []byte("rm -rf /")}),
	})

	// ASSERT — error frame is returned
	frame, ok := readFrameOfType(t, conn, ws.TypeError, 2*time.Second)
	require.True(t, ok, "client must receive an error frame for forbidden input")

	var ep ws.ErrorPayload
	require.NoError(t, json.Unmarshal(frame.Payload, &ep))
	assert.Contains(t, ep.Message, "permission denied")

	// Give the legacy handler a moment in case it (incorrectly) acted on the input.
	time.Sleep(50 * time.Millisecond)

	assert.Empty(t, env.files.uploadList(),
		"no upload must happen when the user lacks send permission")
	assert.Empty(t, env.dCmds.executedCommands(),
		"no command must be executed when the user lacks send permission")
}

// TestRunLegacyMode_uploadFailureSurfacesAsError verifies that when the
// file service fails to upload input, the client receives an error frame.
func TestRunLegacyMode_uploadFailureSurfacesAsError(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	env.files.uploadErr = errDiskFull

	conn := env.dial(t)
	_, _ = readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)

	// ACT
	writeJSONFrame(t, conn, &ws.InboundMessage{
		Type:    typeAttachInput,
		Payload: rawPayload(t, attachInputPayload{Data: []byte("ping")}),
	})

	// ASSERT
	frame, ok := readFrameOfType(t, conn, ws.TypeError, 2*time.Second)
	require.True(t, ok, "client must receive an error frame on upload failure")

	var ep ws.ErrorPayload
	require.NoError(t, json.Unmarshal(frame.Payload, &ep))
	assert.Contains(t, ep.Message, "failed to send input",
		"error must mention the user-facing failure")
}

// TestRunLegacyMode_historyViaScriptGetConsole verifies that when the node
// has script_get_console set, the daemon command service is invoked and its
// output is sent as an attach.output frame.
func TestRunLegacyMode_historyViaScriptGetConsole(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)
	cmd := "cat {dir}/server.log"
	env.setNodeScriptGetConsole(t, cmd)
	env.dCmds.result = &daemon.CommandResult{Output: "scripted history"}

	// ACT
	conn := env.dial(t)

	// ASSERT
	output, ok := readFrameOfType(t, conn, typeAttachOutput, 2*time.Second)
	require.True(t, ok, "history must be returned as attach.output")

	var op attachOutputPayload
	require.NoError(t, json.Unmarshal(output.Payload, &op))
	assert.Equal(t, []byte("scripted history"), op.Data,
		"history bytes from the script must be relayed unchanged")

	assert.Empty(t, env.files.downloadList(),
		"Download must not be called when script_get_console returns content")
}

// TestRunLegacyMode_pollerForwardsAppendedDiff verifies that the legacy
// poller picks up new content appended to output.txt and emits an
// attach.output frame containing only the appended bytes.
func TestRunLegacyMode_pollerForwardsAppendedDiff(t *testing.T) {
	// ARRANGE
	env := newAttachEnv(t, false)

	// First Download (history) returns "hello", subsequent polls return
	// "hello" + appended bytes so the poller computes a non-empty diff.
	calls := 0
	var mu = &sync.Mutex{}
	env.files.downloadHook = func(_ int) ([]byte, error) {
		mu.Lock()
		defer mu.Unlock()
		calls++
		switch calls {
		case 1:
			return []byte("hello"), nil
		case 2:
			// Same content as last seen → no diff emitted.
			return []byte("hello"), nil
		default:
			return []byte("hello world"), nil
		}
	}

	conn := env.dial(t)
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	// Drain the initial frames.
	_, _ = readFrameOfType(t, conn, typeAttachStarted, 2*time.Second)
	hist, ok := readFrameOfType(t, conn, typeAttachOutput, 2*time.Second)
	require.True(t, ok)

	var histPayload attachOutputPayload
	require.NoError(t, json.Unmarshal(hist.Payload, &histPayload))
	require.Equal(t, []byte("hello"), histPayload.Data,
		"history frame must contain initial download bytes")

	// ACT — wait for the poller to advance through at least 3 ticks.
	// (legacyPollInterval = 500ms.)
	frame, ok := readFrameOfType(t, conn, typeAttachOutput, 3*time.Second)
	require.True(t, ok, "poller must publish a diff frame after content grows")

	// ASSERT — only the appended diff is forwarded
	var op attachOutputPayload
	require.NoError(t, json.Unmarshal(frame.Payload, &op))
	assert.Equal(t, []byte(" world"), op.Data, "diff bytes must be the appended suffix")
}

// TestRunLegacyMode_historyTrimmedToMaxBytes verifies that when the
// downloaded history exceeds the 64KiB cap, only the trailing window is
// forwarded.
func TestRunLegacyMode_historyTrimmedToMaxBytes(t *testing.T) {
	// ARRANGE
	const maxBytes = 65536
	big := make([]byte, maxBytes+1024)
	for i := range big {
		big[i] = byte('A' + (i % 26))
	}

	env := newAttachEnv(t, false)
	env.files.downloadResp = big

	// ACT
	conn := env.dial(t)

	// ASSERT
	output, ok := readFrameOfType(t, conn, typeAttachOutput, 2*time.Second)
	require.True(t, ok)

	var op attachOutputPayload
	require.NoError(t, json.Unmarshal(output.Payload, &op))
	require.Len(t, op.Data, maxBytes, "history must be trimmed to the trailing 64KiB window")
	// Trailing chunk: the last byte of the input is at index len(big)-1, and
	// the trimmed slice keeps indices [len(big)-maxBytes : len(big)).
	assert.Equal(t, big[len(big)-maxBytes:], op.Data, "trimmed bytes must match trailing window")
}
