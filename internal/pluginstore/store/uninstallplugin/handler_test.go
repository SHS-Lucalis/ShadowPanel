package uninstallplugin_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/pluginstore/store/uninstallplugin"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/pkg/api"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/gorilla/mux"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUninstallPlugin(t *testing.T) {
	pluginRepo := inmemory.NewPluginRepository()
	fileManager := files.NewInMemoryFileManager()

	existingPlugin := domain.Plugin{
		ID:       pkgplugin.ParsePluginID("testplugin123"),
		Name:     "Test Plugin",
		Version:  "1.0.0",
		Filename: lo.ToPtr("testplugin123.wasm"),
		Status:   domain.PluginStatusActive,
	}
	err := pluginRepo.Save(context.Background(), &existingPlugin)
	require.NoError(t, err)

	err = fileManager.Write(context.Background(), "plugins/testplugin123.wasm", []byte("wasm content"))
	require.NoError(t, err)

	h := uninstallplugin.NewHandler(
		pluginRepo,
		fileManager,
		nil,
		"plugins",
		api.NewResponder(),
	)
	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/plugins/store/plugins/testplugin123", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "testplugin123"})

	h.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNoContent, recorder.Code)

	remaining, err := pluginRepo.Find(context.Background(), nil, nil, nil)
	require.NoError(t, err)
	assert.Len(t, remaining, 0)

	assert.False(t, fileManager.Exists(context.Background(), "plugins/testplugin123.wasm"))
}

func TestUninstallPlugin_not_installed(t *testing.T) {
	pluginRepo := inmemory.NewPluginRepository()
	fileManager := files.NewInMemoryFileManager()

	h := uninstallplugin.NewHandler(
		pluginRepo,
		fileManager,
		nil,
		"plugins",
		api.NewResponder(),
	)
	recorder := httptest.NewRecorder()

	req := httptest.NewRequest(http.MethodDelete, "/api/admin/plugins/store/plugins/nonexistent", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "nonexistent"})

	h.ServeHTTP(recorder, req)

	assert.Equal(t, http.StatusNotFound, recorder.Code)
}
