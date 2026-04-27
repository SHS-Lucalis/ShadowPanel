// API Security Tests for OWASP API Security Top 10:2023.
// This file contains shared helpers reused across all router_security_*_test.go files
// (auth / IDOR / escalation / daemon).
//
// Reference: https://owasp.org/API-Security/editions/2023/

package api_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/api"
	"github.com/gameap/gameap/internal/domain"
	pkgstrings "github.com/gameap/gameap/pkg/strings"
	"github.com/gameap/gameap/pkg/testcontainer"
	"github.com/stretchr/testify/require"
)

// securityTestEnv bundles everything a security test needs: container, fixtures, router.
type securityTestEnv struct {
	container *testcontainer.InmemoryContainer
	fixtures  *testcontainer.TestFixtures
	router    http.Handler
	ctx       context.Context
}

// setupSecurityTest creates a fresh in-memory container with seeded fixtures
// (admin/regular users, two servers, two nodes with daemon API tokens, enrollment setup key)
// and builds a real router. Each test gets isolated state.
func setupSecurityTest(tb testing.TB) *securityTestEnv {
	tb.Helper()

	c, err := testcontainer.LoadInmemoryContainer()
	require.NoError(tb, err)

	ctx := context.Background()

	fixtures, err := testcontainer.SetupFixtures(ctx, c)
	require.NoError(tb, err)

	return &securityTestEnv{
		container: c,
		fixtures:  fixtures,
		router:    api.CreateRouter(c),
		ctx:       ctx,
	}
}

// issuePASETOToken returns a valid user-auth token (PASETO/JWT, depending on auth.Service impl)
// with one-hour TTL. Use for happy-path authentication.
func issuePASETOToken(tb testing.TB, env *securityTestEnv, user *domain.User) string {
	tb.Helper()

	token, err := env.container.AuthService().GenerateTokenForUser(user, time.Hour)
	require.NoError(tb, err)

	return token
}

// issueExpiredPASETOToken returns a token that was already expired one hour ago.
// Used to assert API2 (Broken Authentication) — expired credentials must be rejected.
func issueExpiredPASETOToken(tb testing.TB, env *securityTestEnv, user *domain.User) string {
	tb.Helper()

	token, err := env.container.AuthService().GenerateTokenForUser(user, -time.Hour)
	require.NoError(tb, err)

	return token
}

// issuePAT creates and persists a Personal Access Token bound to the given user
// with the given abilities, returns the wire-format token string `<id>|<secret>`.
func issuePAT(
	tb testing.TB,
	env *securityTestEnv,
	user *domain.User,
	abilities []domain.PATAbility,
) string {
	tb.Helper()

	secret, err := pkgstrings.CryptoRandomString(40)
	require.NoError(tb, err)

	abilitiesCopy := abilities
	token := &domain.PersonalAccessToken{
		TokenableType: domain.EntityTypeUser,
		TokenableID:   user.ID,
		Name:          "security-test-token",
		Token:         pkgstrings.SHA256(secret),
		Abilities:     &abilitiesCopy,
	}

	require.NoError(tb, env.container.PersonalAccessTokenRepository().Save(env.ctx, token))

	return fmt.Sprintf("%d|%s", token.ID, secret)
}

// doRequest executes an HTTP request through the router with a Bearer token (if non-empty).
func doRequest(
	tb testing.TB,
	env *securityTestEnv,
	method, path, bearerToken string,
) *httptest.ResponseRecorder {
	tb.Helper()

	return doRequestWithBody(tb, env, method, path, bearerToken, nil)
}

// doRequestWithBody is doRequest with a request body (e.g. for POST/PUT cases).
func doRequestWithBody(
	tb testing.TB,
	env *securityTestEnv,
	method, path, bearerToken string,
	body []byte,
) *httptest.ResponseRecorder {
	tb.Helper()

	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, path, reader)
	if bearerToken != "" {
		req.Header.Set("Authorization", "Bearer "+bearerToken)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	return w
}

// doRequestRaw executes the request without setting Authorization or Content-Type —
// useful when the test wants full control over headers/cookies/query.
func doRequestRaw(
	tb testing.TB,
	env *securityTestEnv,
	req *http.Request,
) *httptest.ResponseRecorder {
	tb.Helper()

	w := httptest.NewRecorder()
	env.router.ServeHTTP(w, req)

	return w
}

// parseMethodPath splits "METHOD /path" notation used in table-driven tests.
func parseMethodPath(s string) (method, path string) {
	for i := 0; i < len(s); i++ {
		if s[i] == ' ' {
			return s[:i], s[i+1:]
		}
	}

	return s, ""
}
