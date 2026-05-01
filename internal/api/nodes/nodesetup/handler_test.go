package nodesetup

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	daemonbase "github.com/gameap/gameap/internal/api/daemon/base"
	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/internal/certificates"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/enrollment"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeCache struct {
	delegate cache.Cache
	setErr   error
}

func newFakeCache(setErr error) *fakeCache {
	return &fakeCache{
		delegate: cache.NewInMemory(),
		setErr:   setErr,
	}
}

func (c *fakeCache) Get(ctx context.Context, key string) (any, error) {
	return c.delegate.Get(ctx, key)
}

func (c *fakeCache) Set(ctx context.Context, key string, value any, options ...cache.Option) error {
	if c.setErr != nil {
		return c.setErr
	}

	return c.delegate.Set(ctx, key, value, options...)
}

func (c *fakeCache) Delete(ctx context.Context, key string) error {
	return c.delegate.Delete(ctx, key)
}

func (c *fakeCache) Clear(ctx context.Context) error {
	return c.delegate.Clear(ctx)
}

func newAuthCtx() context.Context {
	session := &auth.Session{
		Login: "admin",
		Email: "admin@example.com",
		User:  &testUser1,
	}

	return auth.ContextWithSession(context.Background(), session)
}

func newEnrollmentService(t *testing.T, c cache.Cache) *enrollment.Service {
	t.Helper()

	keyManager := enrollment.NewSetupKeyManager(c, "")
	certsSvc := certificates.NewService(files.NewInMemoryFileManager())
	nodesRepo := inmemory.NewNodeRepository()
	clientCertsRepo := inmemory.NewClientCertificateRepository()

	return enrollment.NewService(keyManager, nodesRepo, clientCertsRepo, certsSvc)
}

var testUser1 = domain.User{
	ID:    1,
	Login: "admin",
	Email: "admin@example.com",
}

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		setupAuth      func() context.Context
		setupEnv       func(t *testing.T)
		setupCache     func(cache.Cache)
		panelHost      string
		expectedStatus int
		wantError      string
		validateResp   func(*testing.T, setupResponse)
	}{
		{
			name: "successful setup without env token",
			setupAuth: func() context.Context {
				session := &auth.Session{
					Login: "admin",
					Email: "admin@example.com",
					User:  &testUser1,
				}

				return auth.ContextWithSession(context.Background(), session)
			},
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.NotEmpty(t, resp.Token)
				assert.NotEmpty(t, resp.Link)
				assert.Equal(t, "http://panel.example.com", resp.Host)
				assert.Contains(t, resp.Link, "http://panel.example.com/gdaemon/setup/")
			},
		},
		{
			name: "successful setup with env token",
			setupAuth: func() context.Context {
				session := &auth.Session{
					Login: "admin",
					Email: "admin@example.com",
					User:  &testUser1,
				}

				return auth.ContextWithSession(context.Background(), session)
			},
			setupEnv: func(t *testing.T) {
				t.Helper()

				t.Setenv("DAEMON_SETUP_TOKEN", "test-env-token")
			},
			panelHost:      "https://panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.Equal(t, "test-env-token", resp.Token)
				assert.NotEmpty(t, resp.Link)
				assert.Equal(t, "http://panel.example.com", resp.Host)
				assert.Contains(t, resp.Link, "http://panel.example.com/gdaemon/setup/test-env-token")
			},
		},
		{
			name:           "user not authenticated",
			panelHost:      "https://panel.example.com",
			expectedStatus: http.StatusUnauthorized,
			wantError:      "user not authenticated",
		},
		{
			name: "cache clears old setup token",
			setupAuth: func() context.Context {
				session := &auth.Session{
					Login: "admin",
					Email: "admin@example.com",
					User:  &testUser1,
				}

				return auth.ContextWithSession(context.Background(), session)
			},
			setupCache: func(c cache.Cache) {
				err := c.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "old-token", cache.WithExpiration(300*time.Second))
				require.NoError(t, err)
			},
			panelHost:      "https://panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.NotEmpty(t, resp.Token)
				assert.NotEqual(t, "old-token", resp.Token)
			},
		},
		{
			name: "creates and stores create token in cache",
			setupAuth: func() context.Context {
				session := &auth.Session{
					Login: "admin",
					Email: "admin@example.com",
					User:  &testUser1,
				}

				return auth.ContextWithSession(context.Background(), session)
			},
			panelHost:      "https://panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.NotEmpty(t, resp.Token)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, tt.panelHost, nil, 0, "", 0)

			if tt.setupCache != nil {
				tt.setupCache(cacheInstance)
			}

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			ctx := context.Background()
			if tt.setupAuth != nil {
				ctx = tt.setupAuth()
			}

			req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
			req = req.WithContext(ctx)
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.wantError != "" {
				var response map[string]any
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
				assert.Equal(t, "error", response["status"])
				errorMsg, ok := response["error"].(string)
				require.True(t, ok)
				assert.Contains(t, errorMsg, tt.wantError)
			}

			if tt.validateResp != nil {
				var resp setupResponse
				require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
				tt.validateResp(t, resp)
			}
		})
	}
}

func TestHandler_SetupTokenNotFromEnv(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "https://panel.example.com", nil, 0, "", 0)

	session := &auth.Session{
		Login: "admin",
		Email: "admin@example.com",
		User:  &testUser1,
	}
	ctx := auth.ContextWithSession(context.Background(), session)

	req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	val, err := cacheInstance.Get(context.Background(), daemonbase.AutoSetupTokenCacheKey)
	require.NoError(t, err)
	assert.NotNil(t, val)

	tokenStr, ok := val.(string)
	require.True(t, ok)
	assert.NotEmpty(t, tokenStr)
}

func TestHandler_HostDetection(t *testing.T) {
	tests := []struct {
		name       string
		panelHost  string
		headers    map[string]string
		host       string
		useTLS     bool
		wantHost   string
		wantInLink string
	}{
		{
			name:       "uses_configured_panel_host",
			panelHost:  "configured.example.com",
			host:       "request.example.com",
			wantHost:   "http://configured.example.com",
			wantInLink: "http://configured.example.com/gdaemon/setup/",
		},
		{
			name:       "detects_from_request_host_without_configured_host",
			panelHost:  "",
			host:       "detected.example.com",
			wantHost:   "http://detected.example.com",
			wantInLink: "http://detected.example.com/gdaemon/setup/",
		},
		{
			name:      "uses_x_forwarded_host_header",
			panelHost: "",
			headers: map[string]string{
				"X-Forwarded-Host":  "forwarded.example.com",
				"X-Forwarded-Proto": "https",
			},
			host:       "request.example.com",
			wantHost:   "https://forwarded.example.com",
			wantInLink: "https://forwarded.example.com/gdaemon/setup/",
		},
		{
			name:       "tls_request_returns_https_scheme",
			panelHost:  "",
			host:       "secure.example.com",
			useTLS:     true,
			wantHost:   "https://secure.example.com",
			wantInLink: "https://secure.example.com/gdaemon/setup/",
		},
		{
			name:       "request_host_with_port_kept_in_url_when_no_panel_host",
			panelHost:  "",
			host:       "1.2.3.4:8080",
			wantHost:   "http://1.2.3.4:8080",
			wantInLink: "http://1.2.3.4:8080/gdaemon/setup/",
		},
		{
			name:      "x_forwarded_host_with_port_used_in_url",
			panelHost: "",
			headers: map[string]string{
				"X-Forwarded-Host": "example.com:9000",
			},
			host:       "request.example.com",
			wantHost:   "http://example.com:9000",
			wantInLink: "http://example.com:9000/gdaemon/setup/",
		},
		{
			name:      "x_forwarded_proto_https_overrides_scheme_without_tls",
			panelHost: "panel.example.com",
			headers: map[string]string{
				"X-Forwarded-Proto": "https",
			},
			host:       "request.example.com",
			wantHost:   "https://panel.example.com",
			wantInLink: "https://panel.example.com/gdaemon/setup/",
		},
		{
			name:      "x_forwarded_proto_overrides_tls_detection",
			panelHost: "panel.example.com",
			headers: map[string]string{
				"X-Forwarded-Proto": "http",
			},
			host:       "panel.example.com",
			useTLS:     true,
			wantHost:   "http://panel.example.com",
			wantInLink: "http://panel.example.com/gdaemon/setup/",
		},
		{
			name:       "panel_host_with_https_prefix_is_stripped",
			panelHost:  "https://panel.example.com",
			host:       "request.example.com",
			wantHost:   "http://panel.example.com",
			wantInLink: "http://panel.example.com/gdaemon/setup/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, tt.panelHost, nil, 0, "", 0)

			ctx := newAuthCtx()

			req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
			req = req.WithContext(ctx)
			req.Host = tt.host

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			if tt.useTLS {
				req.TLS = &tls.ConnectionState{}
			}

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			var resp setupResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			assert.Equal(t, tt.wantHost, resp.Host, "Host field must match expected")
			assert.Contains(t, resp.Link, tt.wantInLink, "Link must use detected base URL")
		})
	}
}

func TestNewLegacySetupResponse(t *testing.T) {
	token := "test-token"
	host := "https://example.com"

	resp := newLegacySetupResponse(token, host)

	assert.Equal(t, "test-token", resp.Token)
	assert.Equal(t, "https://example.com", resp.Host)
	assert.Equal(t, "https://example.com/gdaemon/setup/test-token", resp.Link)
	assert.False(t, resp.GRPCEnabled)
}

func TestHandler_LegacyCacheSetError(t *testing.T) {
	cacheInstance := newFakeCache(errors.New("storage backend offline"))
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com", nil, 0, "", 0)

	ctx := newAuthCtx()
	req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response["status"])

	errorMsg, ok := response["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errorMsg, "Internal Server Error",
		"500 responses should hide internal error detail per pkg/api responder contract")
}

func TestHandler_GRPCMode(t *testing.T) {
	tests := []struct {
		name           string
		panelHost      string
		grpcPort       uint16
		grpcExtHost    string
		grpcExtPort    uint16
		host           string
		headers        map[string]string
		validateResp   func(t *testing.T, resp setupResponse)
		expectedStatus int
	}{
		{
			name:           "grpc_mode_success_uses_internal_grpc_port",
			panelHost:      "panel.example.com",
			grpcPort:       31718,
			host:           "request.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.True(t, resp.GRPCEnabled, "grpc_enabled flag must be true in gRPC mode")
				assert.NotEmpty(t, resp.Token, "Token (setup key) must be returned")
				assert.Len(t, resp.Token, 32, "setup key length must match enrollment.setupKeyLength")

				assert.Equal(t, "http://panel.example.com", resp.Host)
				assert.Equal(t, resp.Link, resp.SetupLink, "SetupLink should mirror Link in gRPC mode")
				assert.Contains(t, resp.Link, "http://panel.example.com/nodes/setup/"+resp.Token,
					"Link must point to /nodes/setup/<key>")

				assert.Equal(t, "grpc://panel.example.com:31718/"+resp.Token, resp.ConnectURL,
					"connect URL must use scheme grpc + panel host + grpcPort + key")

				assert.Equal(t, "curl -sLf '"+resp.Link+"' | bash", resp.LinuxCmd,
					"Linux command must wrap the setup link in curl|bash")
				assert.Equal(t, "gameapctl daemon install --connect="+resp.ConnectURL, resp.WindowsCmd,
					"Windows command must contain --connect=<connect-url>")
			},
		},
		{
			name:           "grpc_mode_uses_grpc_ext_port_when_set",
			panelHost:      "panel.example.com",
			grpcPort:       31718,
			grpcExtPort:    9999,
			host:           "request.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.Contains(t, resp.ConnectURL, ":9999/", "external grpc port must override internal port")
				assert.NotContains(t, resp.ConnectURL, ":31718/", "internal grpc port must not appear")
			},
		},
		{
			name:           "grpc_mode_uses_grpc_ext_host_when_set",
			panelHost:      "panel.example.com",
			grpcPort:       31718,
			grpcExtHost:    "grpc.public.example.com",
			host:           "request.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.Equal(t, "http://panel.example.com", resp.Host,
					"public Host should still come from panel host detection")
				assert.Contains(t, resp.ConnectURL, "grpc://grpc.public.example.com:31718/",
					"grpc connect URL must use grpcExtHost")
			},
		},
		{
			name:        "grpc_mode_strips_port_from_request_host_for_grpc_url",
			panelHost:   "",
			grpcPort:    31718,
			host:        "1.2.3.4:8080",
			grpcExtHost: "",
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.Contains(t, resp.ConnectURL, "grpc://1.2.3.4:31718/",
					"resolveGRPCHost must strip the port from r.Host before assembling connect URL")
				assert.NotContains(t, resp.ConnectURL, ":8080:")
			},
			expectedStatus: http.StatusOK,
		},
		{
			name:      "grpc_mode_falls_back_to_x_forwarded_host_when_panel_host_empty",
			panelHost: "",
			grpcPort:  31718,
			host:      "request.example.com",
			headers: map[string]string{
				"X-Forwarded-Host": "forwarded.example.com:9000",
			},
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp setupResponse) {
				t.Helper()

				assert.Contains(t, resp.ConnectURL, "grpc://forwarded.example.com:31718/",
					"resolveGRPCHost falls back to X-Forwarded-Host with stripped port")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			enrollSvc := newEnrollmentService(t, cacheInstance)
			handler := NewHandler(
				cacheInstance,
				responder,
				tt.panelHost,
				enrollSvc,
				tt.grpcPort,
				tt.grpcExtHost,
				tt.grpcExtPort,
			)

			req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
			req = req.WithContext(newAuthCtx())
			req.Host = tt.host

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, tt.expectedStatus, w.Code)

			var resp setupResponse
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

			tt.validateResp(t, resp)

			storedKey, err := cacheInstance.Get(context.Background(), enrollment.SetupKeyCacheKey)
			require.NoError(t, err, "generated setup key must be persisted in cache for later validation")
			assert.Equal(t, resp.Token, storedKey,
				"the key returned to the client must match the one stored in cache")
		})
	}
}

func TestHandler_GRPCModeGenerateError(t *testing.T) {
	cacheInstance := newFakeCache(errors.New("redis cluster down"))
	responder := api.NewResponder()
	enrollSvc := newEnrollmentService(t, cacheInstance)
	handler := NewHandler(cacheInstance, responder, "panel.example.com", enrollSvc, 31718, "", 0)

	req := httptest.NewRequest(http.MethodGet, "/api/dedicated_servers/setup", nil)
	req = req.WithContext(newAuthCtx())
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusInternalServerError, w.Code,
		"setup key generation failure must surface as 500")

	var response map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, "error", response["status"])

	errorMsg, ok := response["error"].(string)
	require.True(t, ok)
	assert.Contains(t, errorMsg, "Internal Server Error",
		"500 responses should hide internal error detail per pkg/api responder contract")

	_, err := cacheInstance.Get(context.Background(), enrollment.SetupKeyCacheKey)
	assert.Error(t, err, "no setup key should be persisted when Generate fails")
}
