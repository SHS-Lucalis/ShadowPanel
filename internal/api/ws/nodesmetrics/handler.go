package nodesmetrics

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type Handler struct {
	metricsHub     metrics.Hub
	rbac           base.RBAC
	nodes          nodesProvider
	hub            *ws.Hub
	originPatterns []string
	responder      base.Responder
	logger         *slog.Logger
}

func NewHandler(
	metricsHub metrics.Hub,
	rbac base.RBAC,
	nodes nodesProvider,
	hub *ws.Hub,
	originPatterns []string,
	responder base.Responder,
) *Handler {
	return &Handler{
		metricsHub:     metricsHub,
		rbac:           rbac,
		nodes:          nodes,
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

	nodeIDs, err := h.enabledNodeIDs(ctx)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to load nodes"),
			http.StatusInternalServerError,
		))

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

	pumpAll(ctx, h.metricsHub, nodeIDs, client, h.logger)

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

func (h *Handler) enabledNodeIDs(ctx context.Context) ([]uint64, error) {
	nodes, err := h.nodes.FindAll(ctx, nil, nil)
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find nodes")
	}

	ids := make([]uint64, 0, len(nodes))
	for _, n := range nodes {
		if !n.Enabled {
			continue
		}
		ids = append(ids, uint64(n.ID))
	}

	return ids, nil
}
