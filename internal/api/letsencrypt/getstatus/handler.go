package getstatus

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
)

type Handler struct {
	cfg         Config
	acmeService ACMEService
	responder   base.Responder
}

func NewHandler(cfg Config, acmeService ACMEService, responder base.Responder) *Handler {
	return &Handler{
		cfg:         cfg,
		acmeService: acmeService,
		responder:   responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if !h.cfg.ACMEEnabled() || h.acmeService == nil {
		h.responder.Write(ctx, rw, disabledResponse())

		return
	}

	h.responder.Write(ctx, rw, newResponse(h.acmeService.Status()))
}
