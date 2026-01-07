package getlabels

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/services/pluginstore"
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

	lang := pluginstore.ExtractLanguage(r)

	labels, err := h.storeService.GetLabels(ctx, lang)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to get labels from plugin store"))

		return
	}

	h.responder.Write(ctx, rw, newLabelsResponse(labels))
}
