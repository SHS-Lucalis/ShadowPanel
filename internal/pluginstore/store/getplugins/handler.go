package getplugins

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/services/pluginstore"
	pkgplugin "github.com/gameap/gameap/pkg/plugin"
	"github.com/pkg/errors"
)

type Handler struct {
	storeService *pluginstore.Service
	pluginRepo   repositories.PluginRepository
	responder    base.Responder
}

func NewHandler(
	storeService *pluginstore.Service,
	pluginRepo repositories.PluginRepository,
	responder base.Responder,
) *Handler {
	return &Handler{
		storeService: storeService,
		pluginRepo:   pluginRepo,
		responder:    responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	lang := pluginstore.ExtractLanguage(r)
	params := h.parseParams(r)

	storePlugins, err := h.storeService.GetPlugins(ctx, lang, params)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to get plugins from store"))

		return
	}

	installedPlugins, err := h.pluginRepo.Find(ctx, nil, nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to get installed plugins"))

		return
	}

	installedMap := make(map[uint]string)
	for _, p := range installedPlugins {
		installedMap[p.ID] = p.Version
	}

	response := newPluginsResponse(storePlugins, installedMap)

	h.responder.Write(ctx, rw, response)
}

func (h *Handler) parseParams(r *http.Request) pluginstore.GetPluginsParams {
	params := pluginstore.GetPluginsParams{}

	if pageStr := r.URL.Query().Get("page[number]"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil && page > 0 {
			params.Page = page
		}
	}

	if pageSizeStr := r.URL.Query().Get("page[size]"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil && pageSize > 0 && pageSize <= 100 {
			params.PerPage = pageSize
		}
	}

	if sort := r.URL.Query().Get("sort"); sort != "" {
		if sortBy, found := strings.CutPrefix(sort, "-"); found {
			params.SortBy = sortBy
			params.SortOrder = "desc"
		} else {
			params.SortBy = sort
			params.SortOrder = "asc"
		}
	}

	if category := r.URL.Query().Get("category"); category != "" {
		params.Category = category
	}

	if label := r.URL.Query().Get("label"); label != "" {
		params.Label = label
	}

	return params
}

func getInstalledVersion(storeID string, installedMap map[uint]string) *string {
	dbID := pkgplugin.ParsePluginID(storeID)
	if version, ok := installedMap[dbID]; ok {
		return &version
	}

	return nil
}

func isInstalled(storeID string, installedMap map[uint]string) bool {
	dbID := pkgplugin.ParsePluginID(storeID)
	_, ok := installedMap[dbID]

	return ok
}
