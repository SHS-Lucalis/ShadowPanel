// Package logout implements the POST /api/auth/logout endpoint.
//
// Aligned with OWASP API Security Top 10:2023 — API2:2023 (Broken
// Authentication). The endpoint marks the bearer token used to authenticate
// the request as revoked, so any subsequent request presenting the same token
// is rejected with 401, even before the token's natural expiration.
package logout

import (
	"net/http"
	"strings"
	"time"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

// defaultRevocationTTL is the upper bound used when the bearer token has no
// derivable expiration (e.g. a Personal Access Token that never expires). The
// revocation only needs to outlive any plausible re-use window — the cache
// entry will be reaped after this period.
const defaultRevocationTTL = 30 * 24 * time.Hour

type Handler struct {
	authService auth.Service
	revocation  auth.TokenRevocation
	responder   base.Responder
}

func NewHandler(
	authService auth.Service,
	revocation auth.TokenRevocation,
	responder base.Responder,
) *Handler {
	return &Handler{
		authService: authService,
		revocation:  revocation,
		responder:   responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("user not authenticated"),
			http.StatusUnauthorized,
		))

		return
	}

	bearer := extractBearerToken(r)
	if bearer == "" {
		// The auth middleware accepted us, so the token must be somewhere; if
		// it's missing here the request shape is unusual. Treat as success
		// (the middleware wouldn't have admitted us with no token, so this
		// path is a defensive fallback).
		rw.WriteHeader(http.StatusNoContent)

		return
	}

	ttl := h.revocationTTL(bearer)

	if err := h.revocation.Revoke(ctx, auth.TokenIdentifier(bearer), ttl); err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to revoke token"))

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

// revocationTTL derives how long the denylist entry must persist. For
// stateless tokens (PASETO/JWT) it is the time until the token's `exp` claim,
// since after that point the token would be rejected on its own. For PATs and
// any token whose expiration is unknown we fall back to defaultRevocationTTL.
func (h *Handler) revocationTTL(bearer string) time.Duration {
	claims, err := h.authService.ValidateToken(bearer)
	if err != nil {
		return defaultRevocationTTL
	}

	exp, err := claims.GetExpirationTime()
	if err != nil || exp == nil {
		return defaultRevocationTTL
	}

	remaining := time.Until(*exp)
	if remaining <= 0 {
		return 0
	}

	return remaining
}

func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader != "" {
		parts := strings.Split(authHeader, " ")
		if len(parts) == 2 && strings.EqualFold(parts[0], "bearer") {
			return parts[1]
		}
	}

	if token := r.URL.Query().Get("token"); token != "" {
		return token
	}

	if cookie, err := r.Cookie("token"); err == nil && cookie.Value != "" {
		return cookie.Value
	}

	return ""
}
