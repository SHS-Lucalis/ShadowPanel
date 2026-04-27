// API Security Tests for OWASP API Security Top 10:2023.
// Categories covered:
//   - API2:2023 — Broken Authentication (X-Auth-Token contract for daemon endpoints)
//   - API8:2023 — Security Misconfiguration (enrollment / setup-key surface)
//
// References:
//   - https://owasp.org/API-Security/editions/2023/en/0xa2-broken-authentication/
//   - https://owasp.org/API-Security/editions/2023/en/0xa8-security-misconfiguration/
//
// The /gdaemon_api/* family authenticates daemons (not users) via the X-Auth-Token
// header, looking up the calling node by its `gdaemon_api_token`. A separate
// downstream middleware (DaemonGRPCGuard) ensures the daemon isn't already
// connected via gRPC bidi stream — but that requires a SessionRegistry, which
// the in-memory test container intentionally leaves nil. As a result, these
// tests focus on what the *DaemonAuthMiddleware* must enforce in isolation
// (the security-critical first layer); positive happy-paths that exercise the
// full pipeline are out of scope here.

package api_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/pkg/testcontainer"
	"github.com/stretchr/testify/assert"
)

// TestRouterSecurity_API2_DaemonAPIAuth verifies the X-Auth-Token contract:
// missing/empty/invalid/cross-typed tokens must result in 401 Unauthorized
// before the request reaches any business logic.
func TestRouterSecurity_API2_DaemonAPIAuth(t *testing.T) {
	type tokenSource int
	const (
		tokenAbsent tokenSource = iota
		tokenEmpty
		tokenBogus
		tokenUserPASETO
		tokenViaAuthorizationHeader
	)

	tests := []struct {
		name           string
		method         string
		path           string
		source         tokenSource
		wantStatusCode int
	}{
		{"no_x_auth_token_on_get_servers_returns_401",
			http.MethodGet, "/gdaemon_api/servers", tokenAbsent, http.StatusUnauthorized},
		{"empty_x_auth_token_on_get_servers_returns_401",
			http.MethodGet, "/gdaemon_api/servers", tokenEmpty, http.StatusUnauthorized},
		{"bogus_x_auth_token_on_get_servers_returns_401",
			http.MethodGet, "/gdaemon_api/servers", tokenBogus, http.StatusUnauthorized},
		{"user_paseto_in_x_auth_token_returns_401",
			http.MethodGet, "/gdaemon_api/servers", tokenUserPASETO, http.StatusUnauthorized},
		{"daemon_endpoint_does_not_accept_authorization_header",
			http.MethodGet, "/gdaemon_api/servers", tokenViaAuthorizationHeader, http.StatusUnauthorized},

		{"no_x_auth_token_on_get_server_returns_401",
			http.MethodGet, "/gdaemon_api/servers/1", tokenAbsent, http.StatusUnauthorized},
		{"bogus_x_auth_token_on_put_server_returns_401",
			http.MethodPut, "/gdaemon_api/servers/1", tokenBogus, http.StatusUnauthorized},
		{"no_x_auth_token_on_put_task_returns_401",
			http.MethodPut, "/gdaemon_api/tasks/1", tokenAbsent, http.StatusUnauthorized},
		{"bogus_x_auth_token_on_put_task_returns_401",
			http.MethodPut, "/gdaemon_api/tasks/1", tokenBogus, http.StatusUnauthorized},
		{"bogus_x_auth_token_on_put_task_output_returns_401",
			http.MethodPut, "/gdaemon_api/tasks/1/output", tokenBogus, http.StatusUnauthorized},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := setupSecurityTest(t)
			req := httptest.NewRequest(tt.method, tt.path, nil)

			switch tt.source {
			case tokenAbsent:
				// no header at all
			case tokenEmpty:
				req.Header.Set("X-Auth-Token", "")
			case tokenBogus:
				req.Header.Set("X-Auth-Token", "this-is-not-a-real-node-token")
			case tokenUserPASETO:
				req.Header.Set("X-Auth-Token", issuePASETOToken(t, env, env.fixtures.AdminUser))
			case tokenViaAuthorizationHeader:
				// Sending a real node token via Authorization (Bearer) must NOT authenticate
				// for /gdaemon_api/* — that family ignores Authorization and only honours X-Auth-Token.
				req.Header.Set("Authorization", "Bearer "+testcontainer.Node1GDaemonAPIToken)
			}

			w := doRequestRaw(t, env, req)
			assert.Equalf(t,
				tt.wantStatusCode,
				w.Code,
				"daemon endpoint must enforce X-Auth-Token; body=%s", w.Body.String(),
			)
		})
	}
}

// TestRouterSecurity_API2_DaemonGetTokenIsPublic verifies that /gdaemon_api/get_token
// is intentionally reachable without auth (it's the bootstrap endpoint).
// We pin this so a future change does not silently lock it down or expose more.
func TestRouterSecurity_API2_DaemonGetTokenIsPublic(t *testing.T) {
	env := setupSecurityTest(t)
	req := httptest.NewRequest(http.MethodGet, "/gdaemon_api/get_token", nil)

	w := doRequestRaw(t, env, req)

	// We do NOT assert a specific status — depending on configuration it may be
	// 200, 400, or 401 from the handler itself — but it must not be 404 (route
	// vanished) and must not be 500 (panic).
	assert.NotEqualf(t, http.StatusNotFound, w.Code,
		"/gdaemon_api/get_token route must remain registered; body=%s", w.Body.String())
	assert.NotEqualf(t, http.StatusInternalServerError, w.Code,
		"/gdaemon_api/get_token must not panic; body=%s", w.Body.String())
}

// TestRouterSecurity_API2_DaemonAPI_DeletedNodeToken verifies what happens when a
// node is removed from the system but its X-Auth-Token is still in flight.
//
// In the current implementation, DaemonAuthMiddleware queries the node repository
// with `WithDeleted: true`, which means a soft-deleted node *still authenticates*.
// This is a documented behavioural gotcha; the test pins it so anyone tightening
// the middleware later must update the contract intentionally.
func TestRouterSecurity_API2_DaemonAPI_DeletedNodeToken(t *testing.T) {
	env := setupSecurityTest(t)

	// Hard-delete Node1 from the in-memory repo.
	err := env.container.NodeRepository().Delete(env.ctx, env.fixtures.Node1.ID)
	if err != nil {
		t.Fatalf("delete node: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/gdaemon_api/servers", nil)
	req.Header.Set("X-Auth-Token", testcontainer.Node1GDaemonAPIToken)

	w := doRequestRaw(t, env, req)

	// Hard-deleted node: token is no longer valid → 401.
	// (If the repo treats deletion as soft, the middleware's WithDeleted=true
	//  would still let the call through; in that case the test will surface the
	//  behavioural divergence as a non-401 status.)
	assert.Equalf(t, http.StatusUnauthorized, w.Code,
		"hard-deleted node's token must not authenticate; body=%s", w.Body.String())
}

// TestRouterSecurity_API8_EnrollmentSetupKeyValidation verifies the public
// enrollment surface at /nodes/setup/{key}. With EnrollmentService unavailable
// (the test container intentionally returns nil from EnrollmentServiceOrNil),
// the endpoint must respond with 503 Service Unavailable and never leak.
func TestRouterSecurity_API8_EnrollmentSetupKeyValidation(t *testing.T) {
	env := setupSecurityTest(t)

	tests := []struct {
		name string
		path string
	}{
		{"random_setup_key", "/nodes/setup/totally-random-key"},
		{"empty_setup_key", "/nodes/setup/"},
		{"path_traversal_in_setup_key", "/nodes/setup/..%2Fadmin"},
		{"sql_injection_attempt_in_setup_key", "/nodes/setup/%27%20OR%201=1--"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			w := doRequestRaw(t, env, req)

			assert.NotEqualf(t, http.StatusOK, w.Code,
				"enrollment must not succeed without a valid configured key; body=%s", w.Body.String())
			assert.NotEqualf(t, http.StatusInternalServerError, w.Code,
				"enrollment endpoint must not panic on malformed input; body=%s", w.Body.String())
		})
	}
}

// TestRouterSecurity_API8_DaemonSetupTokenValidation verifies the legacy
// /gdaemon/setup/{token} surface. Without a configured DAEMON_SETUP_TOKEN
// or matching cache entry, every request must be rejected with 4xx.
func TestRouterSecurity_API8_DaemonSetupTokenValidation(t *testing.T) {
	env := setupSecurityTest(t)

	tests := []string{
		"/gdaemon/setup/wrong-token",
		"/gdaemon/setup/empty-token",
		"/gdaemon/setup/%27%20OR%201=1--",
	}

	for _, path := range tests {
		t.Run("rejects_"+sanitize(path), func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, path, nil)
			w := doRequestRaw(t, env, req)

			assert.NotEqualf(t, http.StatusOK, w.Code,
				"daemon setup must not succeed with bogus token; body=%s", w.Body.String())
			assert.GreaterOrEqualf(t, w.Code, 400,
				"daemon setup with bogus token must return 4xx; body=%s", w.Body.String())
		})
	}
}

// TestRouterSecurity_API8_NodeCreateRejectsBogusToken verifies POST /gdaemon/create/{token}.
// Without a valid setup token, a node-creation request must be rejected.
// Otherwise an attacker could enroll a malicious node and receive valid certificates.
func TestRouterSecurity_API8_NodeCreateRejectsBogusToken(t *testing.T) {
	env := setupSecurityTest(t)

	req := httptest.NewRequest(http.MethodPost, "/gdaemon/create/bogus-token", nil)
	w := doRequestRaw(t, env, req)

	assert.NotEqualf(t, http.StatusOK, w.Code,
		"node creation must not succeed with a bogus setup token; body=%s", w.Body.String())
	assert.NotEqualf(t, http.StatusInternalServerError, w.Code,
		"node creation must not panic on bogus input; body=%s", w.Body.String())
}

// sanitize converts a URL path into a stable test name fragment.
func sanitize(s string) string {
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch {
		case ch >= 'a' && ch <= 'z',
			ch >= 'A' && ch <= 'Z',
			ch >= '0' && ch <= '9':
			out = append(out, ch)
		default:
			out = append(out, '_')
		}
	}

	return string(out)
}
