package gateway

import (
	"context"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestService_validateAuth(t *testing.T) {
	tests := []struct {
		name      string
		setupNode func(*testing.T, *serviceDeps)
		setupAuth func(*serviceDeps)
		req       *proto.RegisterRequest
		wantCode  codes.Code
		wantError string
	}{
		{
			name: "valid_apikey_with_active_node_returns_nil",
			setupNode: func(t *testing.T, d *serviceDeps) {
				t.Helper()
				require.NoError(t, d.nodeRepo.Save(context.Background(), &domain.Node{
					Enabled:       true,
					Name:          "node1",
					GdaemonAPIKey: "secret-key",
				}))
			},
			setupAuth: func(d *serviceDeps) {
				d.apiKeyVerifier.valid = map[string]uint64{"secret-key": 1}
			},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "secret-key",
			},
		},
		{
			name: "node_not_found_returns_not_found",
			setupNode: func(_ *testing.T, _ *serviceDeps) {
				// no nodes saved
			},
			setupAuth: func(d *serviceDeps) {
				d.apiKeyVerifier.valid = map[string]uint64{"key": 1}
			},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "key",
			},
			wantCode:  codes.NotFound,
			wantError: "node not found",
		},
		{
			name: "disabled_node_returns_permission_denied",
			setupNode: func(t *testing.T, d *serviceDeps) {
				t.Helper()
				require.NoError(t, d.nodeRepo.Save(context.Background(), &domain.Node{
					Enabled:       false,
					Name:          "disabled",
					GdaemonAPIKey: "key",
				}))
			},
			setupAuth: func(d *serviceDeps) {
				d.apiKeyVerifier.valid = map[string]uint64{"key": 1}
			},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "key",
			},
			wantCode:  codes.PermissionDenied,
			wantError: "node is disabled",
		},
		{
			name: "missing_apikey_returns_invalid_argument",
			setupNode: func(t *testing.T, d *serviceDeps) {
				t.Helper()
				require.NoError(t, d.nodeRepo.Save(context.Background(), &domain.Node{
					Enabled:       true,
					Name:          "n",
					GdaemonAPIKey: "key",
				}))
			},
			setupAuth: func(_ *serviceDeps) {},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "",
			},
			wantCode:  codes.InvalidArgument,
			wantError: "API key is required",
		},
		{
			name: "wrong_apikey_returns_unauthenticated",
			setupNode: func(t *testing.T, d *serviceDeps) {
				t.Helper()
				require.NoError(t, d.nodeRepo.Save(context.Background(), &domain.Node{
					Enabled:       true,
					Name:          "n",
					GdaemonAPIKey: "real-key",
				}))
			},
			setupAuth: func(d *serviceDeps) {
				d.apiKeyVerifier.valid = map[string]uint64{"real-key": 1}
			},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "wrong-key",
			},
			wantCode:  codes.Unauthenticated,
			wantError: "invalid API key",
		},
		{
			name: "verifier_error_returns_internal",
			setupNode: func(t *testing.T, d *serviceDeps) {
				t.Helper()
				require.NoError(t, d.nodeRepo.Save(context.Background(), &domain.Node{
					Enabled:       true,
					Name:          "n",
					GdaemonAPIKey: "key",
				}))
			},
			setupAuth: func(d *serviceDeps) {
				d.apiKeyVerifier.err = errSentinel
			},
			req: &proto.RegisterRequest{
				NodeId: 1,
				ApiKey: "key",
			},
			wantCode:  codes.Internal,
			wantError: "failed to verify API key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			svc, deps := newServiceWithDeps(t)
			tt.setupNode(t, deps)
			tt.setupAuth(deps)

			// ACT
			err := svc.validateAuth(context.Background(), tt.req)

			// ASSERT
			if tt.wantError == "" {
				require.NoError(t, err, "validateAuth must succeed")

				return
			}

			require.Error(t, err)
			st, ok := status.FromError(err)
			require.True(t, ok, "error must be a gRPC status")
			assert.Equal(t, tt.wantCode, st.Code(), "gRPC code mismatch")
			assert.Contains(t, st.Message(), tt.wantError, "error message must contain expected substring")
		})
	}
}

func TestService_validateAuth_nilVerifierFallsBackToNodeAPIKey(t *testing.T) {
	t.Run("matching_apikey_succeeds_when_no_verifier_configured", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		require.NoError(t, deps.nodeRepo.Save(context.Background(), &domain.Node{
			Enabled:       true,
			GdaemonAPIKey: "stored-key",
		}))
		// drop verifier to exercise the fallback branch
		svc.apiKeyVerifier = nil

		// ACT
		err := svc.validateAuth(context.Background(), &proto.RegisterRequest{
			NodeId: 1,
			ApiKey: "stored-key",
		})

		// ASSERT
		require.NoError(t, err)
	})

	t.Run("mismatched_apikey_returns_unauthenticated_when_no_verifier_configured", func(t *testing.T) {
		// ARRANGE
		svc, deps := newServiceWithDeps(t)
		require.NoError(t, deps.nodeRepo.Save(context.Background(), &domain.Node{
			Enabled:       true,
			GdaemonAPIKey: "stored-key",
		}))
		svc.apiKeyVerifier = nil

		// ACT
		err := svc.validateAuth(context.Background(), &proto.RegisterRequest{
			NodeId: 1,
			ApiKey: "different-key",
		})

		// ASSERT
		require.Error(t, err)
		st, ok := status.FromError(err)
		require.True(t, ok)
		assert.Equal(t, codes.Unauthenticated, st.Code())
		assert.Contains(t, st.Message(), "invalid API key")
	})
}
