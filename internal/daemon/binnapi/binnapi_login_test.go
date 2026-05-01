package binnapi

import (
	"context"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/et-nik/binngo/decode"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginPipe creates a pair of in-memory net.Conn connections for client/server
// roleplay during Login tests. Cleanup closes both ends.
func loginPipe(t *testing.T) (clientConn, serverConn net.Conn) {
	t.Helper()

	clientConn, serverConn = net.Pipe()

	t.Cleanup(func() {
		_ = clientConn.Close()
		_ = serverConn.Close()
	})

	return clientConn, serverConn
}

// readLoginRequestOnServer fully consumes the login request body and its
// 4-byte sentinel from the server side of a net.Pipe.
func readLoginRequestOnServer(t *testing.T, serverConn net.Conn) []any {
	t.Helper()

	var raw []any
	require.NoError(t, decode.NewDecoder(serverConn).Decode(&raw))

	require.NoError(t, ReadEndBytes(context.Background(), serverConn))

	return raw
}

func TestLogin_success(t *testing.T) {
	// ARRANGE
	clientConn, serverConn := loginPipe(t)

	serverErr := make(chan error, 1)
	serverGot := make(chan []any, 1)

	go func() {
		raw := readLoginRequestOnServer(t, serverConn)
		serverGot <- raw

		resp := &BaseResponseMessage{Code: StatusCodeOK, Info: "ok"}
		serverErr <- WriteMessage(serverConn, resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	// ACT
	err := Login(ctx, clientConn, ModeCMD, "user", "pass")

	// ASSERT
	require.NoError(t, err)
	require.NoError(t, <-serverErr, "server-side write must succeed")

	got := <-serverGot
	require.GreaterOrEqual(t, len(got), 4, "login request must contain mode, login, password, sub-mode")
	assert.Equal(t, "user", got[1], "login field must be encoded in slot 1")
	assert.Equal(t, "pass", got[2], "password field must be encoded in slot 2")
}

func TestLogin_serverError(t *testing.T) {
	// ARRANGE
	clientConn, serverConn := loginPipe(t)

	serverErr := make(chan error, 1)
	go func() {
		_ = readLoginRequestOnServer(t, serverConn)

		resp := &BaseResponseMessage{Code: StatusCodeError, Info: "bad credentials"}
		serverErr <- WriteMessage(serverConn, resp)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	// ACT
	err := Login(ctx, clientConn, ModeCMD, "user", "wrong")

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "login failed", "client must surface the login-failure prefix")
	assert.Contains(t, err.Error(), "bad credentials", "client must include server-provided info string")

	require.NoError(t, <-serverErr)
}

func TestLogin_writeError_returnsError(t *testing.T) {
	// ARRANGE
	clientConn, serverConn := loginPipe(t)

	require.NoError(t, clientConn.Close(), "closing client must succeed before Login attempt")
	_ = serverConn.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// ACT
	err := Login(ctx, clientConn, ModeCMD, "user", "pass")

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to write login message", "write failure must be wrapped with the documented prefix")
}

func TestLogin_decodeError_returnsError(t *testing.T) {
	// ARRANGE
	clientConn, serverConn := loginPipe(t)

	serverDone := make(chan struct{})
	go func() {
		defer close(serverDone)

		_ = readLoginRequestOnServer(t, serverConn)

		// 0x10 is not a known BINN storage type — the decoder rejects the
		// container immediately with ErrUnknownType (no further reads). This
		// is a single byte so net.Pipe's synchronous Write does not deadlock.
		_, _ = serverConn.Write([]byte{0x10})
		_ = serverConn.Close()
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	t.Cleanup(cancel)

	// ACT
	err := Login(ctx, clientConn, ModeCMD, "user", "pass")

	// ASSERT
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to decode login response", "decode failure must be wrapped with the documented prefix")

	<-serverDone
}

func TestLogin_contextCanceled_duringEndBytesRead(t *testing.T) {
	// ARRANGE
	clientConn, serverConn := loginPipe(t)

	releaseServer := make(chan struct{})
	serverDone := make(chan struct{})

	go func() {
		defer close(serverDone)

		_ = readLoginRequestOnServer(t, serverConn)

		// Write a valid response, but never the trailing sentinel; ReadEndBytes
		// will block until the context expires.
		resp := &BaseResponseMessage{Code: StatusCodeOK, Info: "ok"}
		encoded, err := resp.MarshalBINN()
		if err != nil {
			return
		}
		_, _ = serverConn.Write(encoded)
		// Block holding the connection open without flushing the sentinel.
		<-releaseServer
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	t.Cleanup(cancel)

	// ACT
	err := Login(ctx, clientConn, ModeCMD, "user", "pass")

	// ASSERT
	require.Error(t, err)
	combined := err.Error()
	assert.True(t,
		strings.Contains(combined, "failed to read end bytes") ||
			strings.Contains(combined, "deadline exceeded"),
		"error must mention the end-bytes wrapper or the context cancellation: got %q", combined,
	)

	close(releaseServer)
	<-serverDone
}
