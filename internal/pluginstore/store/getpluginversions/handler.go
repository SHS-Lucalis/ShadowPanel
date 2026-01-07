package getpluginversions

import (
	"net/http"
	"strconv"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/services/pluginstore"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

type Handler struct {
	storeService *pluginstore.Service
	responder    base.Responder
}

func NewHandler(storeService *pluginstore.Service, responder base.Responder) *Handler {
	return &Handler{
		storeService: storeService,
		responder:    responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	pluginID, err := api.NewInputReader(r).ReadString("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to read plugin ID"))

		return
	}

	params := h.parseParams(r)

	versions, err := h.storeService.GetPluginVersions(ctx, pluginID, params)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to get plugin versions from store"))

		return
	}

	h.responder.Write(ctx, rw, newVersionsResponse(versions))
}

func (h *Handler) parseParams(r *http.Request) pluginstore.GetPluginVersionsParams {
	params := pluginstore.GetPluginVersionsParams{}

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

	return params
}
