package getstatus_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/acme"
	"github.com/gameap/gameap/internal/api/letsencrypt/getstatus"
	"github.com/gameap/gameap/pkg/api"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubConfig struct {
	enabled bool
}

func (s stubConfig) ACMEEnabled() bool { return s.enabled }

type stubService struct {
	status acme.Status
}

func (s stubService) Status() acme.Status { return s.status }

func TestHandler_ReturnsDisabledWhenACMEOff(t *testing.T) {
	tests := []struct {
		name    string
		cfg     getstatus.Config
		service getstatus.ACMEService
	}{
		{
			name:    "config_disabled",
			cfg:     stubConfig{enabled: false},
			service: stubService{},
		},
		{
			name:    "service_nil",
			cfg:     stubConfig{enabled: true},
			service: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := getstatus.NewHandler(tt.cfg, tt.service, api.NewResponder())

			rw := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/api/admin/letsencrypt/status", nil)

			h.ServeHTTP(rw, req)

			require.Equal(t, http.StatusOK, rw.Code)

			var body map[string]any
			require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &body))

			assert.Equal(t, false, body["enabled"])
			assert.Equal(t, "disabled", body["state"])
		})
	}
}

func TestHandler_ReturnsActiveStatus(t *testing.T) {
	notBefore := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	notAfter := time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)
	last := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	next := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC)

	cfg := stubConfig{enabled: true}
	svc := stubService{
		status: acme.Status{
			Enabled:            true,
			State:              acme.StateActive,
			Domains:            []string{"*.example.com", "example.com"},
			DNSProvider:        "gameap-cloudflare:cloudflare",
			NotBefore:          notBefore,
			NotAfter:           notAfter,
			LastRenewalAt:      last,
			NextRenewalCheckAt: next,
		},
	}

	h := getstatus.NewHandler(cfg, svc, api.NewResponder())

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/letsencrypt/status", nil)

	h.ServeHTTP(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &body))

	assert.Equal(t, true, body["enabled"])
	assert.Equal(t, "active", body["state"])
	assert.Equal(t, "gameap-cloudflare:cloudflare", body["dns_provider"])

	domains, ok := body["domains"].([]any)
	require.True(t, ok)
	assert.Len(t, domains, 2)
	assert.Equal(t, "*.example.com", domains[0])
}

func TestHandler_ReportsFailureState(t *testing.T) {
	cfg := stubConfig{enabled: true}
	svc := stubService{
		status: acme.Status{
			Enabled:   true,
			State:     acme.StateFailed,
			Domains:   []string{"example.com"},
			LastError: "DNS provider rejected token: invalid credentials",
		},
	}

	h := getstatus.NewHandler(cfg, svc, api.NewResponder())

	rw := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/admin/letsencrypt/status", nil)

	h.ServeHTTP(rw, req)

	require.Equal(t, http.StatusOK, rw.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(rw.Body.Bytes(), &body))

	assert.Equal(t, "failed", body["state"])
	assert.Contains(t, body["last_error"], "DNS provider rejected token")
}
