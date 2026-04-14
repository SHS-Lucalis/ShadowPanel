package middlewares

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type DaemonGRPCGuardMiddleware struct {
	connChecker DaemonConnectionChecker
	responder   base.Responder
}

func NewDaemonGRPCGuardMiddleware(
	connChecker DaemonConnectionChecker,
	responder base.Responder,
) *DaemonGRPCGuardMiddleware {
	return &DaemonGRPCGuardMiddleware{
		connChecker: connChecker,
		responder:   responder,
	}
}

func (m *DaemonGRPCGuardMiddleware) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		daemonSession := auth.DaemonSessionFromContext(r.Context())
		if daemonSession == nil || daemonSession.Node == nil {
			m.responder.WriteError(r.Context(), w, api.WrapHTTPError(
				errors.New("daemon session not found"),
				http.StatusUnauthorized,
			))

			return
		}

		if m.connChecker.IsConnectedAnywhere(uint64(daemonSession.Node.ID)) {
			m.responder.WriteError(r.Context(), w, api.WrapHTTPError(
				errors.New("daemon is connected via gRPC bidi stream, HTTP API is disabled for this node"),
				http.StatusConflict,
			))

			return
		}

		next.ServeHTTP(w, r)
	})
}
