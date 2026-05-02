// Package uploadsessiontest provides shared test helpers for upload session HTTP handlers.
package uploadsessiontest

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/api/filemanager/uploadsession"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/rbac"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	UserID    = uint(1)
	ServerID  = uint(1)
	NodeID    = uint(1)
	WorkPath  = "/srv/gameap"
	ServerDir = "servers/test1"
)

func NewUser() *domain.User {
	return &domain.User{ID: UserID, Login: "tester", Email: "tester@example.com"}
}

func NewNode() *domain.Node {
	return &domain.Node{
		ID:                  NodeID,
		Enabled:             true,
		Name:                "test-node",
		OS:                  "linux",
		WorkPath:            WorkPath,
		GdaemonHost:         "127.0.0.1",
		GdaemonPort:         31717,
		GdaemonAPIKey:       "test-key",
		GdaemonServerCert:   "cert",
		ClientCertificateID: 1,
	}
}

func NewServer() *domain.Server {
	now := time.Now()

	return &domain.Server{
		ID:        ServerID,
		UID:       uuid.MustParse("11111111-1111-1111-1111-111111111111"),
		UUIDShort: "short1",
		Enabled:   true,
		Installed: 1,
		Name:      "Test Server",
		GameID:    "cs",
		DSID:      NodeID,
		GameModID: 1,
		ServerIP:  "127.0.0.1",
		Dir:       ServerDir,
		CreatedAt: &now,
		UpdatedAt: &now,
	}
}

// NewResolver wires real in-memory repositories with the production resolver
// constructor, so tests exercise the full path-resolution + RBAC flow.
//
// Pass withAccess=true to grant the test user the GameServerFiles ability on
// the seeded server. The returned Resolver is the same value handlers see in
// production.
func NewResolver(t *testing.T, withAccess bool) *uploadsession.Resolver {
	t.Helper()

	servers := inmemory.NewServerRepository()
	nodes := inmemory.NewNodeRepository()
	rbacRepo := inmemory.NewRBACRepository()
	rbacService := rbac.NewRBAC(services.NewNilTransactionManager(), rbacRepo, 0)

	require.NoError(t, servers.Save(context.Background(), NewServer()))
	servers.AddUserServer(UserID, ServerID)
	require.NoError(t, nodes.Save(context.Background(), NewNode()))

	if withAccess {
		grantFilesAbility(t, rbacRepo, UserID, ServerID)
	}

	return uploadsession.NewResolver(servers, nodes, rbacService)
}

// NewRequest builds an HTTP request configured the way every upload-session
// handler test needs it: optional auth context, mux URL vars and a body
// reader. Method is left to the caller because handlers vary
// (POST/PUT/GET/DELETE).
func NewRequest(
	t *testing.T,
	method string,
	body []byte,
	vars map[string]string,
	withAuth bool,
) *http.Request {
	t.Helper()

	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req := httptest.NewRequest(method, "/", bodyReader)
	ctx := context.Background()
	if withAuth {
		ctx = auth.ContextWithSession(ctx, &auth.Session{
			Login: "tester",
			Email: "tester@example.com",
			User:  NewUser(),
		})
	}
	req = req.WithContext(ctx)
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}

	return req
}

// AssertErrorContains decodes a JSON error body produced by api.Responder and
// asserts that the "error" field contains the expected substring. Use it after
// asserting the HTTP status code so that the failure message is precise about
// which layer is wrong.
func AssertErrorContains(t *testing.T, body []byte, want string) {
	t.Helper()

	var resp map[string]any
	require.NoError(t, json.Unmarshal(body, &resp), "error body must be valid JSON")
	errMsg, _ := resp["error"].(string)
	assert.Contains(t, errMsg, want, "error body should contain expected substring")
}

func grantFilesAbility(t *testing.T, repo *inmemory.RBACRepository, userID, serverID uint) {
	t.Helper()
	ability := &domain.Ability{
		Name:       domain.AbilityNameGameServerFiles,
		EntityType: lo.ToPtr(domain.EntityTypeServer),
		EntityID:   new(serverID),
	}
	require.NoError(t, repo.SaveAbility(context.Background(), ability))
	require.NoError(t, repo.SavePermission(context.Background(), &domain.Permission{
		AbilityID:  ability.ID,
		EntityID:   new(userID),
		EntityType: lo.ToPtr(domain.EntityTypeUser),
		Forbidden:  false,
	}))
}
