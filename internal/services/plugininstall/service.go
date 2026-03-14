package plugininstall

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/plugin"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/pkg/errors"

	"github.com/gameap/gameap/pkg/api"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
)

var ErrPluginAlreadyInstalled = errors.New("plugin already installed")

func CheckNotInstalled(ctx context.Context, repo repositories.PluginRepository, dbID domain.Uint64ID) error {
	exists, err := repo.Exists(ctx, filters.FindPluginByIDs(dbID))
	if err != nil {
		return errors.WithMessage(err, "failed to check if plugin exists")
	}
	if exists {
		return api.WrapHTTPError(ErrPluginAlreadyInstalled, http.StatusConflict)
	}

	return nil
}

func BuildPluginRecord(
	dbID domain.Uint64ID,
	loaded *pkgplugin.LoadedPlugin,
	filename string,
	source string,
) *domain.Plugin {
	return &domain.Plugin{
		ID:          dbID,
		Name:        loaded.Info.Name,
		Version:     loaded.Info.Version,
		Description: loaded.Info.Description,
		Author:      loaded.Info.Author,
		APIVersion:  loaded.Info.ApiVersion,
		Filename:    new(filename),
		Source:      new(source),
		Status:      domain.PluginStatusActive,
		InstalledAt: new(time.Now()),
	}
}

func TryLoadPlugin(
	ctx context.Context,
	loader *plugin.Loader,
	repo repositories.PluginRepository,
	pluginRecord *domain.Plugin,
	filename string,
) {
	if loader == nil {
		return
	}

	loaded, err := loader.LoadWithID(ctx, filename, uint64(pluginRecord.ID))
	if err != nil {
		slog.ErrorContext(ctx, "failed to load plugin",
			slog.String("filename", filename),
			slog.String("error", err.Error()))

		pluginRecord.Status = domain.PluginStatusError
		_ = repo.Save(ctx, pluginRecord)

		return
	}

	loader.RegisterPluginID(pluginRecord.ID, loaded.Info.Id)
}
