package loaded

import (
	"context"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/plugin"
	"github.com/gameap/gameap/internal/repositories"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
)

type LoaderManager interface {
	GetPlugins() []*pkgplugin.LoadedPlugin
}

type Handler struct {
	manager    LoaderManager
	loader     *plugin.Loader
	pluginRepo repositories.PluginRepository
	responder  base.Responder
}

func NewHandler(
	manager LoaderManager,
	loader *plugin.Loader,
	pluginRepo repositories.PluginRepository,
	responder base.Responder,
) *Handler {
	return &Handler{
		manager:    manager,
		loader:     loader,
		pluginRepo: pluginRepo,
		responder:  responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	loadedPlugins := h.manager.GetPlugins()

	dbPlugins := h.fetchDBPlugins(ctx, loadedPlugins)

	response := &listResponse{
		Data: make([]*loadedPluginResponse, 0, len(loadedPlugins)),
	}

	for _, loaded := range loadedPlugins {
		managerID := pkgplugin.CompactPluginID(pkgplugin.ParsePluginID(loaded.Info.Id))
		var dbID *domain.Uint64ID
		var source string

		if dbPlugin, ok := dbPlugins[managerID]; ok {
			dbID = new(domain.Uint64ID)
			*dbID = dbPlugin.ID
			if dbPlugin.Source != nil {
				source = *dbPlugin.Source
			}
		}

		response.Data = append(response.Data, newLoadedPluginResponse(loaded, dbID, source))
	}

	h.responder.Write(ctx, rw, response)
}

func (h *Handler) fetchDBPlugins(
	ctx context.Context,
	loadedPlugins []*pkgplugin.LoadedPlugin,
) map[string]*domain.Plugin {
	if len(loadedPlugins) == 0 {
		return nil
	}

	ids := make([]domain.Uint64ID, 0, len(loadedPlugins))
	for _, loaded := range loadedPlugins {
		ids = append(ids, pkgplugin.ParsePluginID(loaded.Info.Id))
	}

	plugins, err := h.pluginRepo.Find(ctx, &filters.FindPlugin{IDs: ids}, nil, nil)
	if err != nil {
		return nil
	}

	result := make(map[string]*domain.Plugin, len(plugins))
	for i := range plugins {
		managerID := pkgplugin.CompactPluginID(plugins[i].ID)
		result[managerID] = &plugins[i]
	}

	return result
}
