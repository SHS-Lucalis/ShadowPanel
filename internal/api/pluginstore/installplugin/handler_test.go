package installplugin_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/internal/api/pluginstore/installplugin"
	"github.com/gameap/gameap/internal/cache"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services/pluginstore"
	"github.com/gameap/gameap/pkg/api"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallPlugin(t *testing.T) {
	pluginDetails := pluginstore.PluginDetails{
		ID:            "testplugin123",
		Name:          "Test Plugin",
		Summary:       "A test plugin",
		Description:   "Full description",
		Author:        pluginstore.Author{ID: 1, Username: "TestAuthor"},
		LatestVersion: "1.0.0",
	}

	versions := pluginstore.PaginatedResponse[pluginstore.PluginVersion]{
		Data: []pluginstore.PluginVersion{
			{
				ID:       1,
				Version:  "1.0.0",
				FileHash: "916f0027a575074ce72a331777c3478d6513f786a591bd892da1a577bf2335f9",
				IsStable: true,
			},
		},
		Total: 1,
	}

	wasmContent := []byte("test data")

	tests := []struct {
		name       string
		body       string
		wantStatus int
		wantBody   string
	}{
		{
			name:       "successful_install",
			body:       "",
			wantStatus: http.StatusOK,
			wantBody: `{
				"name": "Test Plugin",
				"version": "1.0.0",
				"description": "Full description",
				"author": "TestAuthor",
				"status": "active"
			}`,
		},
		{
			name:       "install_with_version",
			body:       `{"version": "1.0.0"}`,
			wantStatus: http.StatusOK,
			wantBody: `{
				"name": "Test Plugin",
				"version": "1.0.0",
				"description": "Full description",
				"author": "TestAuthor",
				"status": "active"
			}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				switch r.URL.Path {
				case "/plugins/testplugin123":
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(pluginDetails)
				case "/plugins/testplugin123/versions":
					w.WriteHeader(http.StatusOK)
					_ = json.NewEncoder(w).Encode(versions)
				case "/plugins/testplugin123/versions/1.0.0/download":
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(wasmContent)
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer mockServer.Close()

			storeService := pluginstore.NewService(mockServer.URL, "", cache.NewInMemory())
			pluginRepo := inmemory.NewPluginRepository()
			fileManager := files.NewInMemoryFileManager()

			h := installplugin.NewHandler(
				storeService,
				pluginRepo,
				fileManager,
				nil,
				"plugins",
				api.NewResponder(),
			)
			recorder := httptest.NewRecorder()

			var body *bytes.Reader
			if tt.body != "" {
				body = bytes.NewReader([]byte(tt.body))
			} else {
				body = bytes.NewReader([]byte{})
			}

			req := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/store/plugins/testplugin123/install", body)
			req.Header.Set("Content-Type", "application/json")
			req = mux.SetURLVars(req, map[string]string{"id": "testplugin123"})

			h.ServeHTTP(recorder, req)

			assert.Equal(t, tt.wantStatus, recorder.Code)

			if tt.wantStatus == http.StatusOK {
				var resp map[string]any
				err := json.Unmarshal(recorder.Body.Bytes(), &resp)
				require.NoError(t, err)

				assert.NotNil(t, resp["id"])
				assert.NotNil(t, resp["installed_at"])
				delete(resp, "id")
				delete(resp, "installed_at")

				if tt.wantBody != "" {
					respWithoutDynamic, err := json.Marshal(resp)
					require.NoError(t, err)
					assert.JSONEq(t, tt.wantBody, string(respWithoutDynamic))
				}

				installed, err := pluginRepo.Find(context.Background(), nil, nil, nil)
				require.NoError(t, err)
				require.Len(t, installed, 1)
			}
		})
	}
}

func TestInstallPlugin_already_installed(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	storeService := pluginstore.NewService(mockServer.URL, "", cache.NewInMemory())
	pluginRepo := inmemory.NewPluginRepository()
	fileManager := files.NewInMemoryFileManager()

	existingPlugin := domain.Plugin{
		ID:      pkgplugin.ParsePluginID("testplugin"),
		Name:    "Test Plugin",
		Version: "1.0.0",
		Status:  domain.PluginStatusActive,
	}
	err := pluginRepo.Save(context.Background(), &existingPlugin)
	require.NoError(t, err)

	h := installplugin.NewHandler(
		storeService,
		pluginRepo,
		fileManager,
		nil,
		"plugins",
		api.NewResponder(),
	)
	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/store/plugins/testplugin/install", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "testplugin"})

	h.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusConflict, recorder.Code)
}
