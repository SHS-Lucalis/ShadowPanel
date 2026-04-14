package middlewares

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeConnectionChecker struct {
	connected    bool
	receivedNode uint64
}

func (f *fakeConnectionChecker) IsConnectedAnywhere(nodeID uint64) bool {
	f.receivedNode = nodeID

	return f.connected
}

func TestDaemonGRPCGuardMiddleware_Middleware(t *testing.T) {
	const testNodeID uint = 42

	tests := []struct {
		name           string
		connected      bool
		sessionInCtx   bool
		nilNode        bool
		expectedStatus int
		expectNext     bool
		wantError      string
	}{
		{
			name:           "not_connected_via_grpc_passes_through",
			connected:      false,
			sessionInCtx:   true,
			expectedStatus: http.StatusOK,
			expectNext:     true,
		},
		{
			name:           "connected_via_grpc_returns_conflict",
			connected:      true,
			sessionInCtx:   true,
			expectedStatus: http.StatusConflict,
			expectNext:     false,
			wantError:      "HTTP API is disabled",
		},
		{
			name:           "missing_daemon_session_in_context_returns_unauthorized",
			connected:      false,
			sessionInCtx:   false,
			expectedStatus: http.StatusUnauthorized,
			expectNext:     false,
			wantError:      "daemon session not found",
		},
		{
			name:           "nil_node_in_daemon_session_returns_unauthorized",
			connected:      false,
			sessionInCtx:   true,
			nilNode:        true,
			expectedStatus: http.StatusUnauthorized,
			expectNext:     false,
			wantError:      "daemon session not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			checker := &fakeConnectionChecker{connected: tt.connected}
			responder := api.NewResponder()
			middleware := NewDaemonGRPCGuardMiddleware(checker, responder)

			var nextCalled bool
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				nextCalled = true
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("ok"))
			})

			handler := middleware.Middleware(next)

			req := httptest.NewRequest(http.MethodGet, "/gdaemon_api/tasks", nil)

			if tt.sessionInCtx {
				session := &auth.DaemonSession{}
				if !tt.nilNode {
					session.Node = &domain.Node{ID: testNodeID}
				}
				ctx := auth.ContextWithDaemonSession(context.Background(), session)
				req = req.WithContext(ctx)
			}

			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			assert.Equal(t, tt.expectNext, nextCalled)

			if tt.wantError != "" {
				var response map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Equal(t, "error", response["status"])
				assert.Contains(t, response["error"], tt.wantError)
			}

			if tt.sessionInCtx && !tt.nilNode {
				assert.Equal(t, uint64(testNodeID), checker.receivedNode)
			} else {
				assert.Equal(t, uint64(0), checker.receivedNode)
			}
		})
	}
}

func TestDaemonGRPCGuardMiddleware_ForwardsNodeIDCorrectly(t *testing.T) {
	checker := &fakeConnectionChecker{connected: false}
	responder := api.NewResponder()
	middleware := NewDaemonGRPCGuardMiddleware(checker, responder)

	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	handler := middleware.Middleware(next)

	nodeIDs := []uint{1, 2, 999, 123456}
	for _, id := range nodeIDs {
		req := httptest.NewRequest(http.MethodGet, "/gdaemon_api/tasks", nil)
		ctx := auth.ContextWithDaemonSession(context.Background(), &auth.DaemonSession{
			Node: &domain.Node{ID: id},
		})
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		assert.Equal(t, uint64(id), checker.receivedNode)
	}
}
