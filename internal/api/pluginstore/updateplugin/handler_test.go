package updateplugin_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/internal/api/pluginstore/updateplugin"
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

func TestUpdatePlugin(t *testing.T) {
	versions := pluginstore.PaginatedResponse[pluginstore.PluginVersion]{
		Data: []pluginstore.PluginVersion{
			{
				ID:       2,
				Version:  "2.0.0",
				FileHash: "916f0027a575074ce72a331777c3478d6513f786a591bd892da1a577bf2335f9",
				IsStable: true,
			},
		},
		Total: 1,
	}

	wasmContent := []byte("test data")

	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/plugins/testplugin123/versions":
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(versions)
		case "/plugins/testplugin123/versions/2.0.0/download":
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

	existingPlugin := domain.Plugin{
		ID:      pkgplugin.ParsePluginID("testplugin123"),
		Name:    "Test Plugin",
		Version: "1.0.0",
		Status:  domain.PluginStatusActive,
	}
	err := pluginRepo.Save(context.Background(), &existingPlugin)
	require.NoError(t, err)

	h := updateplugin.NewHandler(
		storeService,
		pluginRepo,
		fileManager,
		nil,
		"plugins",
		api.NewResponder(),
	)
	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/store/plugins/testplugin123/update", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "testplugin123"})

	h.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusOK, recorder.Code)

	var resp map[string]any
	err = json.Unmarshal(recorder.Body.Bytes(), &resp)
	require.NoError(t, err)

	assert.Equal(t, "Test Plugin", resp["name"])
	assert.Equal(t, "2.0.0", resp["version"])
	assert.Equal(t, "active", resp["status"])
	assert.NotNil(t, resp["updated_at"])

	updated, err := pluginRepo.Find(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	require.Len(t, updated, 1)
	assert.Equal(t, "2.0.0", updated[0].Version)
}

func TestUpdatePlugin_not_installed(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer mockServer.Close()

	storeService := pluginstore.NewService(mockServer.URL, "", cache.NewInMemory())
	pluginRepo := inmemory.NewPluginRepository()
	fileManager := files.NewInMemoryFileManager()

	h := updateplugin.NewHandler(
		storeService,
		pluginRepo,
		fileManager,
		nil,
		"plugins",
		api.NewResponder(),
	)
	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodPost, "/api/admin/plugins/store/plugins/nonexistent/update", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	h.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
