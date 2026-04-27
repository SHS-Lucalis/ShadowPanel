package nodemetrics

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/api/ws/metricsbase"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

const defaultReplayWindow = 30 * time.Minute

type Handler struct {
	metricsHub     metrics.Hub
	rbac           base.RBAC
	nodeRepo       repositories.NodeRepository
	hub            *ws.Hub
	originPatterns []string
	responder      base.Responder
	logger         *slog.Logger
}

func NewHandler(
	metricsHub metrics.Hub,
	rbac base.RBAC,
	nodeRepo repositories.NodeRepository,
	hub *ws.Hub,
	originPatterns []string,
	responder base.Responder,
) *Handler {
	return &Handler{
		metricsHub:     metricsHub,
		rbac:           rbac,
		nodeRepo:       nodeRepo,
		hub:            hub,
		originPatterns: originPatterns,
		responder:      responder,
		logger:         slog.Default(),
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		h.responder.WriteError(ctx, rw, api.NewError(http.StatusUnauthorized, "user not authenticated"))

		return
	}

	if err := h.checkAdmin(ctx, session.User); err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	input := api.NewInputReader(r)

	nodeID, err := input.ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid node id"),
			http.StatusBadRequest,
		))

		return
	}

	if err := h.verifyNodeExists(ctx, nodeID); err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	conn, err := ws.Accept(rw, r, &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)

		return
	}

	client := ws.NewClient(ctx, conn, h.hub, nil, h.logger)
	h.hub.Register(client)

	metricsbase.Pump(
		ctx,
		h.metricsHub,
		uint64(nodeID),
		metricsbase.NodePrefixFilter(),
		client,
		defaultReplayWindow,
		h.logger,
	)

	client.Run()
}

func (h *Handler) checkAdmin(ctx context.Context, user *domain.User) error {
	isAdmin, err := h.rbac.Can(ctx, user.ID, []domain.AbilityName{domain.AbilityNameAdminRolesPermissions})
	if err != nil {
		return errors.WithMessage(err, "failed to check admin permissions")
	}
	if !isAdmin {
		return api.NewError(http.StatusForbidden, "access denied: admin only")
	}

	return nil
}

func (h *Handler) verifyNodeExists(ctx context.Context, nodeID uint) error {
	nodes, err := h.nodeRepo.Find(ctx, &filters.FindNode{IDs: []uint{nodeID}}, nil, nil)
	if err != nil {
		return errors.WithMessage(err, "failed to find node")
	}
	if len(nodes) == 0 {
		return api.NewNotFoundError("node not found")
	}

	return nil
}
