package middlewares

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// loginAttemptHandler is a stub for the wrapped login handler. It echoes the
// configured outcome without inspecting the request body.
type loginAttemptHandler struct {
	outcome int
}

func (h *loginAttemptHandler) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(h.outcome)
}

func newLoginRequest(t *testing.T, login, password, remoteAddr string) *http.Request {
	t.Helper()

	body, err := json.Marshal(map[string]string{
		"login":    login,
		"password": password,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = remoteAddr
	req.Header.Set("Content-Type", "application/json")

	return req
}

func TestLoginRateLimitMiddleware_BlocksAfterMaxFailuresPerUsername(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerUsername(3),
		WithLoginRateLimitPerIP(100),
		WithLoginRateLimitWindow(time.Minute),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	for i := range 3 {
		req := newLoginRequest(t, "alice", "wrong", "10.0.0.1:1234")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equalf(t, http.StatusUnauthorized, w.Code,
			"attempt %d should reach the inner handler; body=%s", i, w.Body.String())
	}

	req := newLoginRequest(t, "alice", "wrong", "10.0.0.1:1234")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "fourth attempt must be 429")
	assert.NotEmpty(t, w.Header().Get("Retry-After"), "429 must set Retry-After")
}

func TestLoginRateLimitMiddleware_BlocksAfterMaxFailuresPerIP(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerIP(2),
		WithLoginRateLimitPerUsername(100),
		WithLoginRateLimitWindow(time.Minute),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	// Attempts vary the username so the per-username counter never trips —
	// the per-IP counter is what blocks them.
	for i, login := range []string{"alice", "bob"} {
		req := newLoginRequest(t, login, "wrong", "10.0.0.2:1234")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		assert.Equalf(t, http.StatusUnauthorized, w.Code, "attempt %d (login=%s) reached inner", i, login)
	}

	req := newLoginRequest(t, "carol", "wrong", "10.0.0.2:1234")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "third IP attempt must be 429 even with a fresh username")
}

func TestLoginRateLimitMiddleware_SeparateIPBuckets(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerIP(1),
		WithLoginRateLimitPerUsername(100),
		WithLoginRateLimitWindow(time.Minute),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	// First IP exhausts its limit.
	req := newLoginRequest(t, "alice", "wrong", "10.0.0.10:1234")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// Second IP must not inherit the first IP's failure count.
	req = newLoginRequest(t, "alice", "wrong", "10.0.0.11:1234")
	w = httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, "different IP must have independent counter")
}

func TestLoginRateLimitMiddleware_SuccessResetsUsernameCounter(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerUsername(2),
		WithLoginRateLimitPerIP(100),
		WithLoginRateLimitWindow(time.Minute),
	)
	failHandler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})
	okHandler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusOK})

	// Burn one failure for alice.
	req := newLoginRequest(t, "alice", "wrong", "10.0.0.20:1")
	w := httptest.NewRecorder()
	failHandler.ServeHTTP(w, req)
	require.Equal(t, http.StatusUnauthorized, w.Code)

	// A successful login resets her counter.
	req = newLoginRequest(t, "alice", "right", "10.0.0.20:2")
	w = httptest.NewRecorder()
	okHandler.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	// She should be allowed two more failures before the per-username limit
	// trips, proving the previous one was reset.
	for i := range 2 {
		req = newLoginRequest(t, "alice", "wrong", "10.0.0.20:3")
		w = httptest.NewRecorder()
		failHandler.ServeHTTP(w, req)
		assert.Equalf(t, http.StatusUnauthorized, w.Code, "post-reset attempt %d should reach inner", i)
	}
}

func TestLoginRateLimitMiddleware_BodyForwardedToNextHandler(t *testing.T) {
	// The middleware reads the request body to peek at the username; it must
	// then restore the body so the wrapped login handler can decode it again.
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerIP(100),
		WithLoginRateLimitPerUsername(100),
	)

	var seen []byte
	wrapped := mw.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))

	req := newLoginRequest(t, "alice", "secret", "10.0.0.30:1")
	wrapped.ServeHTTP(httptest.NewRecorder(), req)

	assert.Contains(t, string(seen), `"login":"alice"`,
		"inner handler must still see the JSON body verbatim")
}

func TestLoginRateLimitMiddleware_EmailFieldUsedAsUsername(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerUsername(2),
		WithLoginRateLimitPerIP(100),
		WithLoginRateLimitWindow(time.Minute),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	for i := range 2 {
		body, err := json.Marshal(map[string]string{
			"email":    "Alice@Example.COM",
			"password": "wrong",
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
		req.RemoteAddr = "10.0.0.40:1"
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equalf(t, http.StatusUnauthorized, w.Code, "attempt %d", i)
	}

	// Third attempt must be 429 — and case must not matter.
	body, err := json.Marshal(map[string]string{
		"email":    "ALICE@example.com",
		"password": "wrong",
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.RemoteAddr = "10.0.0.40:2"
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "different-case email must hit the same bucket")
}

func TestLoginRateLimitMiddleware_HonoursClientIPHeader(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerIP(1),
		WithLoginRateLimitPerUsername(100),
		WithLoginRateLimitClientIPHeader("X-Real-IP"),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	// Same RemoteAddr but different X-Real-IP — second request must pass.
	for _, ip := range []string{"203.0.113.1", "203.0.113.2"} {
		req := newLoginRequest(t, "bob", "wrong", "10.0.0.50:1234")
		req.Header.Set("X-Real-IP", ip)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		require.Equalf(t, http.StatusUnauthorized, w.Code, "X-Real-IP=%s reached inner", ip)
	}

	// A third request from the first IP would now exceed its quota.
	req := newLoginRequest(t, "bob", "wrong", "10.0.0.50:1234")
	req.Header.Set("X-Real-IP", "203.0.113.1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusTooManyRequests, w.Code, "second hit on same X-Real-IP must 429")
}

func TestLoginRateLimitMiddleware_NoBodyDoesNotPanic(t *testing.T) {
	c := cache.NewInMemory()
	mw := NewLoginRateLimitMiddleware(c, api.NewResponder())
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", http.NoBody)
	req.RemoteAddr = "10.0.0.60:1"

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"empty body must not block the inner handler")
}

// Sanity: the limiter relies on the cache returning ErrNotFound for missing
// keys; if a buggy cache returned a non-numeric value, readCount must treat it
// as zero rather than panic or count nonsense.
func TestLoginRateLimitMiddleware_NonNumericCacheValueTreatedAsZero(t *testing.T) {
	c := cache.NewInMemory()
	require.NoError(t, c.Set(context.Background(), "auth:login-fail:ip:10.0.0.70", "garbage"))

	mw := NewLoginRateLimitMiddleware(c, api.NewResponder(),
		WithLoginRateLimitPerIP(2),
		WithLoginRateLimitPerUsername(100),
	)
	handler := mw.Middleware(&loginAttemptHandler{outcome: http.StatusUnauthorized})

	req := newLoginRequest(t, "x", "wrong", "10.0.0.70:1")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code,
		"corrupt cache value must not lock the user out before any failures are counted")
}
