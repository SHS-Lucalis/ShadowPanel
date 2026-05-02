package getsession

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/api/filemanager/uploadsession"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type Handler struct {
	resolver  *uploadsession.Resolver
	service   uploadsession.Service
	responder base.Responder
}

func NewHandler(
	resolver *uploadsession.Resolver,
	service uploadsession.Service,
	responder base.Responder,
) *Handler {
	return &Handler{
		resolver:  resolver,
		service:   service,
		responder: responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		h.responder.WriteError(ctx, rw, uploadsession.ErrUserNotAuthenticated)

		return
	}

	reader := api.NewInputReader(r)
	serverID, err := reader.ReadUint("server")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid server id"),
			http.StatusBadRequest,
		))

		return
	}
	uploadID, _ := reader.ReadString("uploadID")
	if uploadID == "" {
		h.responder.WriteError(ctx, rw, uploadsession.ErrUploadIDRequired)

		return
	}

	if _, resolveErr := h.resolver.Resolve(ctx, session.User, serverID); resolveErr != nil {
		h.responder.WriteError(ctx, rw, resolveErr)

		return
	}

	status, err := h.service.Status(ctx, uploadID, session.User.ID)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	h.responder.Write(ctx, rw, newResponse(status))
}
