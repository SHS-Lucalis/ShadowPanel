package publicconfig

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockResponder struct {
	writeCalled      bool
	writeErrorCalled bool
	lastResult       any
	lastError        error
}

func (m *mockResponder) WriteError(_ context.Context, _ http.ResponseWriter, err error) {
	m.writeErrorCalled = true
	m.lastError = err
}

func (m *mockResponder) Write(_ context.Context, _ http.ResponseWriter, result any) {
	m.writeCalled = true
	m.lastResult = result
}

func TestNewHandler(t *testing.T) {
	cfg := &config.Config{}
	responder := &mockResponder{}

	handler := NewHandler(cfg, responder)

	require.NotNil(t, handler)
	assert.Equal(t, cfg, handler.config)
	assert.Equal(t, responder, handler.responder)
}

func TestHandler_ServeHTTP(t *testing.T) {
	tests := []struct {
		name            string
		defaultLanguage string
		expectedResult  Response
	}{
		{
			name:            "returns_empty_when_not_set",
			defaultLanguage: "",
			expectedResult:  Response{DefaultLanguage: ""},
		},
		{
			name:            "returns_en_when_set",
			defaultLanguage: "en",
			expectedResult:  Response{DefaultLanguage: "en"},
		},
		{
			name:            "returns_ru_when_set",
			defaultLanguage: "ru",
			expectedResult:  Response{DefaultLanguage: "ru"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.UI.DefaultLanguage = tt.defaultLanguage

			responder := &mockResponder{}
			handler := NewHandler(cfg, responder)

			req := httptest.NewRequest(http.MethodGet, "/api/config/public", nil)
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			assert.False(t, responder.writeErrorCalled)
			assert.True(t, responder.writeCalled)
			assert.Equal(t, tt.expectedResult, responder.lastResult)
		})
	}
}
