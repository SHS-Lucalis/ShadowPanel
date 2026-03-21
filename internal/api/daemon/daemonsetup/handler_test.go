package daemonsetup

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	daemonbase "github.com/gameap/gameap/internal/api/daemon/base"
	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name           string
		token          string
		setupEnv       func(t *testing.T)
		setupCache     func(cache.Cache)
		panelHost      string
		expectedStatus int
		wantError      bool
		validateResp   func(*testing.T, string)
	}{
		{
			name:  "successful_setup_with_env_token",
			token: "test-env-token",
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("DAEMON_SETUP_TOKEN", "test-env-token")
			},
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp string) {
				t.Helper()

				assert.Contains(t, resp, "export CREATE_TOKEN=")
				assert.Contains(t, resp, "export PANEL_HOST=http://panel.example.com")
				assert.Contains(t, resp, "curl -sL https://raw.githubusercontent.com/gameap/auto-install-scripts/master/install-gdaemon.sh | bash --")
			},
		},
		{
			name:  "successful_setup_with_cache_token",
			token: "cached-token",
			setupCache: func(c cache.Cache) {
				err := c.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "cached-token", cache.WithExpiration(300*time.Second))
				require.NoError(t, err)
			},
			panelHost:      "https://panel.example.com",
			expectedStatus: http.StatusOK,
			validateResp: func(t *testing.T, resp string) {
				t.Helper()

				assert.Contains(t, resp, "export CREATE_TOKEN=")
				assert.Contains(t, resp, "export PANEL_HOST=http://panel.example.com")
				assert.Contains(t, resp, "curl -sL https://raw.githubusercontent.com/gameap/auto-install-scripts/master/install-gdaemon.sh")
			},
		},
		{
			name:           "invalid_token",
			token:          "wrong-token",
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:  "token_mismatch_with_env",
			token: "wrong-token",
			setupEnv: func(t *testing.T) {
				t.Helper()
				t.Setenv("DAEMON_SETUP_TOKEN", "correct-token")
			},
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:  "token_mismatch_with_cache",
			token: "wrong-token",
			setupCache: func(c cache.Cache) {
				err := c.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "correct-token", cache.WithExpiration(300*time.Second))
				require.NoError(t, err)
			},
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusForbidden,
			wantError:      true,
		},
		{
			name:           "token_not_found_in_cache",
			token:          "some-token",
			setupCache:     func(_ cache.Cache) {},
			panelHost:      "panel.example.com",
			expectedStatus: http.StatusForbidden,
			wantError:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, tt.panelHost)

			if tt.setupCache != nil {
				tt.setupCache(cacheInstance)
			}

			if tt.setupEnv != nil {
				tt.setupEnv(t)
			}

			req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/"+tt.token, nil)
			req = mux.SetURLVars(req, map[string]string{"token": tt.token})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.wantError {
				assert.Contains(t, w.Body.String(), "error")
			}

			if tt.validateResp != nil {
				tt.validateResp(t, w.Body.String())
			}
		})
	}
}

func TestHandler_CreateTokenStoredInCache(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com")

	t.Setenv("DAEMON_SETUP_TOKEN", "test-token")

	req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/test-token", nil)
	req = mux.SetURLVars(req, map[string]string{"token": "test-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	val, err := cacheInstance.Get(context.Background(), daemonbase.AutoCreateTokenCacheKey)
	require.NoError(t, err)
	assert.NotNil(t, val)

	tokenStr, ok := val.(string)
	require.True(t, ok)
	assert.NotEmpty(t, tokenStr)
}

func TestHandler_SetupTokenDeletedFromCache(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com")

	err := cacheInstance.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "test-token", cache.WithExpiration(300*time.Second))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/test-token", nil)
	req = mux.SetURLVars(req, map[string]string{"token": "test-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	val, err := cacheInstance.Get(context.Background(), daemonbase.AutoSetupTokenCacheKey)
	if err == nil {
		assert.Nil(t, val)
	}
}

func TestHandler_HostDetection(t *testing.T) {
	tests := []struct {
		name             string
		panelHost        string
		headers          map[string]string
		host             string
		wantHostInScript string
	}{
		{
			name:             "uses_configured_panel_host",
			panelHost:        "configured.example.com",
			host:             "request.example.com",
			wantHostInScript: "http://configured.example.com",
		},
		{
			name:             "detects_from_request_host_without_configured_host",
			panelHost:        "",
			host:             "detected.example.com",
			wantHostInScript: "http://detected.example.com",
		},
		{
			name:      "uses_X-Forwarded-Host_header",
			panelHost: "",
			headers: map[string]string{
				"X-Forwarded-Host":  "forwarded.example.com",
				"X-Forwarded-Proto": "https",
			},
			host:             "request.example.com",
			wantHostInScript: "https://forwarded.example.com",
		},
		{
			name:             "strips_http_prefix_from_panel_host",
			panelHost:        "http://panel.example.com",
			host:             "request.example.com",
			wantHostInScript: "http://panel.example.com",
		},
		{
			name:             "strips_https_prefix_from_panel_host",
			panelHost:        "https://panel.example.com",
			host:             "request.example.com",
			wantHostInScript: "http://panel.example.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, tt.panelHost)

			t.Setenv("DAEMON_SETUP_TOKEN", "test-token")

			req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/test-token", nil)
			req = mux.SetURLVars(req, map[string]string{"token": "test-token"})
			req.Host = tt.host

			for key, value := range tt.headers {
				req.Header.Set(key, value)
			}

			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			body := w.Body.String()
			assert.Contains(t, body, "export PANEL_HOST="+tt.wantHostInScript)
		})
	}
}

func TestHandler_ResponseContentType(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com")

	t.Setenv("DAEMON_SETUP_TOKEN", "test-token")

	req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/test-token", nil)
	req = mux.SetURLVars(req, map[string]string{"token": "test-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "text/plain", w.Header().Get("Content-Type"))
}

func TestHandler_BuildSetupScript(t *testing.T) {
	tests := []struct {
		name         string
		createToken  string
		panelHost    string
		config       string
		wantContains []string
		wantAbsent   []string
		wantLines    int
	}{
		{
			name:        "without_config",
			createToken: "test-create-token",
			panelHost:   "http://panel.example.com",
			config:      "",
			wantContains: []string{
				"export CREATE_TOKEN=test-create-token",
				"export PANEL_HOST=http://panel.example.com",
				"curl -sL https://raw.githubusercontent.com/gameap/auto-install-scripts/master/install-gdaemon.sh | bash --",
			},
			wantAbsent: []string{"export CONFIG="},
			wantLines:  3,
		},
		{
			name:        "with_config",
			createToken: "test-create-token",
			panelHost:   "http://panel.example.com",
			config:      "process_manager.name=podman",
			wantContains: []string{
				"export CREATE_TOKEN=test-create-token",
				"export PANEL_HOST=http://panel.example.com",
				"export CONFIG=cHJvY2Vzc19tYW5hZ2VyLm5hbWU9cG9kbWFu",
				"curl -sL https://raw.githubusercontent.com/gameap/auto-install-scripts/master/install-gdaemon.sh | bash --",
			},
			wantLines: 4,
		},
		{
			name:        "with_config_special_chars",
			createToken: "test-create-token",
			panelHost:   "http://panel.example.com",
			config:      "process_manager.name=podman;process_manager.config.image=debian:bookworm-slim",
			wantContains: []string{
				"export CREATE_TOKEN=test-create-token",
				"export PANEL_HOST=http://panel.example.com",
				"export CONFIG=cHJvY2Vzc19tYW5hZ2VyLm5hbWU9cG9kbWFuO3Byb2Nlc3NfbWFuYWdlci5jb25maWcuaW1hZ2U9ZGViaWFuOmJvb2t3b3JtLXNsaW0=",
				"curl -sL https://raw.githubusercontent.com/gameap/auto-install-scripts/master/install-gdaemon.sh | bash --",
			},
			wantLines: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, "panel.example.com")

			script := handler.buildSetupScript(tt.createToken, tt.panelHost, tt.config)

			for _, want := range tt.wantContains {
				assert.Contains(t, script, want)
			}

			for _, absent := range tt.wantAbsent {
				assert.NotContains(t, script, absent)
			}

			lines := strings.Split(script, "\n")
			assert.GreaterOrEqual(t, len(lines), tt.wantLines)
		})
	}
}

func TestHandler_EnvTokenPriority(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com")

	err := cacheInstance.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "cache-token", cache.WithExpiration(300*time.Second))
	require.NoError(t, err)

	t.Setenv("DAEMON_SETUP_TOKEN", "env-token")

	req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/env-token", nil)
	req = mux.SetURLVars(req, map[string]string{"token": "env-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_CacheTokenUsedWhenEnvEmpty(t *testing.T) {
	cacheInstance := cache.NewInMemory()
	responder := api.NewResponder()
	handler := NewHandler(cacheInstance, responder, "panel.example.com")

	err := cacheInstance.Set(context.Background(), daemonbase.AutoSetupTokenCacheKey, "cache-token", cache.WithExpiration(300*time.Second))
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodGet, "/gdaemon/setup/cache-token", nil)
	req = mux.SetURLVars(req, map[string]string{"token": "cache-token"})
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestHandler_ConfigQueryParameter(t *testing.T) {
	tests := []struct {
		name       string
		config     string
		wantConfig bool
	}{
		{
			name:       "without_config_parameter",
			config:     "",
			wantConfig: false,
		},
		{
			name:       "with_simple_config",
			config:     "process_manager.name=podman",
			wantConfig: true,
		},
		{
			name:       "with_complex_config",
			config:     "process_manager.name=podman;process_manager.config.image=debian:bookworm-slim",
			wantConfig: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cacheInstance := cache.NewInMemory()
			responder := api.NewResponder()
			handler := NewHandler(cacheInstance, responder, "panel.example.com")

			t.Setenv("DAEMON_SETUP_TOKEN", "test-token")

			reqURL := "/gdaemon/setup/test-token"
			if tt.config != "" {
				reqURL += "?config=" + url.QueryEscape(tt.config)
			}

			req := httptest.NewRequest(http.MethodGet, reqURL, nil)
			req = mux.SetURLVars(req, map[string]string{"token": "test-token"})
			w := httptest.NewRecorder()

			handler.ServeHTTP(w, req)

			require.Equal(t, http.StatusOK, w.Code)

			body := w.Body.String()
			assert.Contains(t, body, "export CREATE_TOKEN=")
			assert.Contains(t, body, "export PANEL_HOST=")

			if tt.wantConfig {
				expectedBase64 := base64.StdEncoding.EncodeToString([]byte(tt.config))
				assert.Contains(t, body, "export CONFIG="+expectedBase64)
			} else {
				assert.NotContains(t, body, "export CONFIG=")
			}
		})
	}
}
