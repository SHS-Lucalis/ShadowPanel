package putchunk

import (
	"math"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/api/filemanager/uploadsession"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

const maxChunkSlack = 1

type Handler struct {
	resolver  *uploadsession.Resolver
	service   uploadsession.Service
	responder base.Responder
	chunkSize uint64
}

func NewHandler(
	resolver *uploadsession.Resolver,
	service uploadsession.Service,
	responder base.Responder,
	chunkSize uint64,
) *Handler {
	return &Handler{
		resolver:  resolver,
		service:   service,
		responder: responder,
		chunkSize: chunkSize,
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
	index, err := reader.ReadUint("index")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid chunk index"),
			http.StatusBadRequest,
		))

		return
	}

	if _, resolveErr := h.resolver.Resolve(ctx, session.User, serverID); resolveErr != nil {
		h.responder.WriteError(ctx, rw, resolveErr)

		return
	}

	r.Body = http.MaxBytesReader(rw, r.Body, safeChunkLimit(h.chunkSize))
	defer func() { _ = r.Body.Close() }()

	if writeErr := h.service.WriteChunk(ctx, uploadID, session.User.ID, index, r.Body); writeErr != nil {
		h.responder.WriteError(ctx, rw, writeErr)

		return
	}

	rw.WriteHeader(http.StatusNoContent)
}

func safeChunkLimit(chunkSize uint64) int64 {
	if chunkSize > math.MaxInt64-maxChunkSlack {
		return math.MaxInt64
	}

	return int64(chunkSize) + maxChunkSlack
}
