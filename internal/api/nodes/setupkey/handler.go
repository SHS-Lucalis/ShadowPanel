package setupkey

import (
	"encoding/json"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/enrollment"
	"github.com/pkg/errors"
)

type GetHandler struct {
	enrollmentSvc *enrollment.Service
	responder     base.Responder
}

func NewGetHandler(
	enrollmentSvc *enrollment.Service,
	responder base.Responder,
) *GetHandler {
	return &GetHandler{
		enrollmentSvc: enrollmentSvc,
		responder:     responder,
	}
}

func (h *GetHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	key, err := h.enrollmentSvc.SetupKeyManager().Get(ctx)
	if err != nil {
		if errors.Is(err, enrollment.ErrSetupKeyNotConfigured) {
			h.responder.Write(ctx, rw, setupKeyResponse{SetupKey: ""})

			return
		}

		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to get setup key"))

		return
	}

	h.responder.Write(ctx, rw, setupKeyResponse{SetupKey: key})
}

type PostHandler struct {
	enrollmentSvc *enrollment.Service
	responder     base.Responder
}

func NewPostHandler(
	enrollmentSvc *enrollment.Service,
	responder base.Responder,
) *PostHandler {
	return &PostHandler{
		enrollmentSvc: enrollmentSvc,
		responder:     responder,
	}
}

func (h *PostHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var input struct {
		SetupKey string `json:"setup_key"`
	}
	_ = json.NewDecoder(r.Body).Decode(&input)

	if input.SetupKey != "" {
		key := input.SetupKey
		if err := h.enrollmentSvc.SetupKeyManager().Set(ctx, key); err != nil {
			h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to set setup key"))

			return
		}

		h.responder.Write(ctx, rw, setupKeyResponse{SetupKey: key})

		return
	}

	generatedKey, err := h.enrollmentSvc.SetupKeyManager().Generate(ctx)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to generate setup key"))

		return
	}

	h.responder.Write(ctx, rw, setupKeyResponse{SetupKey: generatedKey})
}

type DeleteHandler struct {
	enrollmentSvc *enrollment.Service
	responder     base.Responder
}

func NewDeleteHandler(
	enrollmentSvc *enrollment.Service,
	responder base.Responder,
) *DeleteHandler {
	return &DeleteHandler{
		enrollmentSvc: enrollmentSvc,
		responder:     responder,
	}
}

func (h *DeleteHandler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if err := h.enrollmentSvc.SetupKeyManager().Delete(ctx); err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to delete setup key"))

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

type setupKeyResponse struct {
	SetupKey string `json:"setup_key"`
}
