package plugininstall_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/repositories/inmemory"
	"github.com/gameap/gameap/internal/services/plugininstall"
	"github.com/gameap/gameap/pkg/api"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/gameap/gameap/pkg/plugin/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCheckNotInstalled(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		setupRepo  func(*inmemory.PluginRepository)
		dbID       domain.Uint64ID
		wantError  string
		wantStatus int
	}{
		{
			name:       "plugin_not_installed",
			setupRepo:  func(_ *inmemory.PluginRepository) {},
			dbID:       12345,
			wantError:  "",
			wantStatus: 0,
		},
		{
			name: "plugin_already_installed_returns_409",
			setupRepo: func(repo *inmemory.PluginRepository) {
				_ = repo.Save(ctx, &domain.Plugin{
					ID:     12345,
					Name:   "Test Plugin",
					Status: domain.PluginStatusActive,
				})
			},
			dbID:       12345,
			wantError:  "plugin already installed",
			wantStatus: http.StatusConflict,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := inmemory.NewPluginRepository()
			tt.setupRepo(repo)

			err := plugininstall.CheckNotInstalled(ctx, repo, tt.dbID)

			if tt.wantError == "" {
				assert.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantError)

				var httpErr interface{ HTTPStatus() int }
				if assert.ErrorAs(t, err, &httpErr) {
					assert.Equal(t, tt.wantStatus, httpErr.HTTPStatus())
				}
			}
		})
	}
}

func TestBuildPluginRecord(t *testing.T) {
	tests := []struct {
		name       string
		dbID       domain.Uint64ID
		loaded     *pkgplugin.LoadedPlugin
		filename   string
		source     string
		wantRecord *domain.Plugin
	}{
		{
			name: "builds_correct_record",
			dbID: 12345,
			loaded: &pkgplugin.LoadedPlugin{
				Info: &proto.PluginInfo{
					Id:          "testplugin",
					Name:        "Test Plugin",
					Version:     "1.0.0",
					Description: "A test plugin",
					Author:      "Test Author",
					ApiVersion:  "v1",
				},
			},
			filename: "12345.wasm",
			source:   "file://12345.wasm",
		},
		{
			name: "builds_record_from_store",
			dbID: 67890,
			loaded: &pkgplugin.LoadedPlugin{
				Info: &proto.PluginInfo{
					Id:          "storeplugin",
					Name:        "Store Plugin",
					Version:     "2.0.0",
					Description: "A store plugin",
					Author:      "Store Author",
					ApiVersion:  "v2",
				},
			},
			filename: "storeplugin.wasm",
			source:   "https://store.gameap.com/plugins/storeplugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := plugininstall.BuildPluginRecord(tt.dbID, tt.loaded, tt.filename, tt.source)

			assert.Equal(t, tt.dbID, record.ID)
			assert.Equal(t, tt.loaded.Info.Name, record.Name)
			assert.Equal(t, tt.loaded.Info.Version, record.Version)
			assert.Equal(t, tt.loaded.Info.Description, record.Description)
			assert.Equal(t, tt.loaded.Info.Author, record.Author)
			assert.Equal(t, tt.loaded.Info.ApiVersion, record.APIVersion)
			require.NotNil(t, record.Filename)
			assert.Equal(t, tt.filename, *record.Filename)
			require.NotNil(t, record.Source)
			assert.Equal(t, tt.source, *record.Source)
			assert.Equal(t, domain.PluginStatusActive, record.Status)
			require.NotNil(t, record.InstalledAt)
		})
	}
}

func TestCheckNotInstalled_returns_409_status(t *testing.T) {
	ctx := context.Background()
	repo := inmemory.NewPluginRepository()

	_ = repo.Save(ctx, &domain.Plugin{
		ID:     pkgplugin.ParsePluginID("testplugin"),
		Name:   "Test Plugin",
		Status: domain.PluginStatusActive,
	})

	err := plugininstall.CheckNotInstalled(ctx, repo, pkgplugin.ParsePluginID("testplugin"))

	require.Error(t, err)

	var wrappedErr *api.WrappedError
	require.ErrorAs(t, err, &wrappedErr)
	assert.Equal(t, http.StatusConflict, wrappedErr.HTTPStatus())
}
