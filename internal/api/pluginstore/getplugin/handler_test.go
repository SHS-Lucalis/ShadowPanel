package getplugin_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/api/pluginstore/getplugin"
	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services/pluginstore"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gorilla/mux"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetPlugin(t *testing.T) {
	storeResp := pluginstore.PluginDetails{
		ID:            "hexeditor4jm2",
		URL:           "https://plugins.gameap.dev/plugins/hexeditor4jm2",
		Name:          "HEX Editor",
		Summary:       "Hex editor in filemanager",
		Description:   "Full description here",
		License:       "MIT",
		RepositoryURL: "https://github.com/gameap/plugin-hex-editor",
		Author:        pluginstore.Author{ID: 2, Username: "GameAP"},
		Category:      pluginstore.Category{ID: 3, Slug: "files", Name: "Files"},
		LatestVersion: "1.0.0",
		PublishedAt:   lo.Must(time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")),
		CreatedAt:     lo.Must(time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")),
		UpdatedAt:     lo.Must(time.Parse(time.RFC3339, "2026-01-01T00:00:00Z")),
	}

	tests := []struct {
		name       string
		statusCode int
		wantStatus int
		wantBody   string
	}{
		{
			name:       "successful_response",
			statusCode: http.StatusOK,
			wantStatus: http.StatusOK,
			wantBody: `{
				"id": "hexeditor4jm2",
				"url": "https://plugins.gameap.dev/plugins/hexeditor4jm2",
				"name": "HEX Editor",
				"summary": "Hex editor in filemanager",
				"description": "Full description here",
				"icon_url": "",
				"license": "MIT",
				"repository_url": "https://github.com/gameap/plugin-hex-editor",
				"min_gameap_version": "",
				"min_plugin_api_version": "",
				"author": {"id": 2, "username": "GameAP"},
				"category": {"id": 3, "slug": "files", "name": "Files", "description": "", "icon": ""},
				"labels": [],
				"download_count": 0,
				"rating_avg": 0,
				"rating_count": 0,
				"latest_version": "1.0.0",
				"requires_subscription": false,
				"published_at": "2026-01-01T00:00:00Z",
				"created_at": "2026-01-01T00:00:00Z",
				"updated_at": "2026-01-01T00:00:00Z",
				"installed": false
			}`,
		},
		{
			name:       "store_error",
			statusCode: http.StatusInternalServerError,
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					_ = json.NewEncoder(w).Encode(storeResp)
				}
			}))
			defer mockServer.Close()

			storeService := pluginstore.NewService(mockServer.URL, "", cache.NewInMemory())
			pluginRepo := inmemory.NewPluginRepository()

			h := getplugin.NewHandler(storeService, pluginRepo, api.NewResponder())
			recorder := httptest.NewRecorder()

			req := httptest.NewRequest(http.MethodGet, "/api/admin/plugins/store/plugins/hexeditor4jm2", nil)
			req = mux.SetURLVars(req, map[string]string{"id": "hexeditor4jm2"})

			h.ServeHTTP(recorder, req)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantStatus == http.StatusOK {
				var resp map[string]any
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)

				if tt.wantBody != "" {
					assert.JSONEq(t, tt.wantBody, recorder.Body.String())
				}
			}
		})
	}
}
