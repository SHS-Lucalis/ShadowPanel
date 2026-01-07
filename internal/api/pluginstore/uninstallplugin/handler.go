package uninstallplugin

import (
	"net/http"
	"path"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/files"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/plugin"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/pkg/errors"
)

type Handler struct {
	pluginRepo  repositories.PluginRepository
	fileManager files.FileManager
	loader      *plugin.Loader
	pluginsDir  string
	responder   base.Responder
}

func NewHandler(
	pluginRepo repositories.PluginRepository,
	fileManager files.FileManager,
	loader *plugin.Loader,
	pluginsDir string,
	responder base.Responder,
) *Handler {
	return &Handler{
		pluginRepo:  pluginRepo,
		fileManager: fileManager,
		loader:      loader,
		pluginsDir:  pluginsDir,
		responder:   responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	storePluginID, err := api.NewInputReader(r).ReadString("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to read plugin ID"))

		return
	}

	dbID := pkgplugin.ParsePluginID(storePluginID)

	installedPlugins, err := h.pluginRepo.Find(ctx, filters.FindPluginByIDs(dbID), nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find installed plugin"))

		return
	}

	if len(installedPlugins) == 0 {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("plugin not installed"),
			http.StatusNotFound,
		))

		return
	}

	pluginRecord := &installedPlugins[0]

	if h.loader != nil {
		if managerID, ok := h.loader.GetPluginManagerID(dbID); ok {
			if err := h.loader.Unload(ctx, managerID); err != nil {
				h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to unload plugin"))

				return
			}
		}
	}

	filename := storePluginID + ".wasm"
	if pluginRecord.Filename != nil && *pluginRecord.Filename != "" {
		filename = *pluginRecord.Filename
	}

	pluginPath := path.Join(h.pluginsDir, filename)

	if h.fileManager.Exists(ctx, pluginPath) {
		if err := h.fileManager.Delete(ctx, pluginPath); err != nil {
			h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to delete plugin file"))

			return
		}
	}

	if err := h.pluginRepo.Delete(ctx, dbID); err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to delete plugin record"))

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}
