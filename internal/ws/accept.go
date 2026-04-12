package ws

import (
	"net/http"
	"time"

	"github.com/coder/websocket"
)

// Accept upgrades the HTTP connection to a WebSocket and clears any read/write
// deadlines that net/http set on the underlying connection based on
// http.Server.ReadTimeout / WriteTimeout.
//
// Without this, long-lived WebSocket sessions are silently killed once the
// HTTP server's per-request write deadline expires (the writePump fails with
// i/o timeout, the connection is closed and updates stop arriving until the
// client reconnects).
func Accept(rw http.ResponseWriter, r *http.Request, opts *websocket.AcceptOptions) (*websocket.Conn, error) {
	rc := http.NewResponseController(rw)
	_ = rc.SetWriteDeadline(time.Time{})
	_ = rc.SetReadDeadline(time.Time{})

	return websocket.Accept(rw, r, opts)
}
