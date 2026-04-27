package servermetrics

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/api/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/api/ws/metricsbase"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const defaultReplayWindow = 30 * time.Minute

type Handler struct {
	metricsHub     metrics.Hub
	serverFinder   *serversbase.ServerFinder
	abilityChecker *serversbase.AbilityChecker
	hub            *ws.Hub
	originPatterns []string
	responder      base.Responder
	logger         *slog.Logger
}

func NewHandler(
	metricsHub metrics.Hub,
	serverRepo repositories.ServerRepository,
	rbac base.RBAC,
	hub *ws.Hub,
	originPatterns []string,
	responder base.Responder,
) *Handler {
	return &Handler{
		metricsHub:     metricsHub,
		serverFinder:   serversbase.NewServerFinder(serverRepo, rbac),
		abilityChecker: serversbase.NewAbilityChecker(rbac),
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

	input := api.NewInputReader(r)

	serverID, err := input.ReadUint("server")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid server id"),
			http.StatusBadRequest,
		))

		return
	}

	server, err := h.serverFinder.FindUserServer(ctx, session.User, serverID)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	if err := h.abilityChecker.CheckOrError(
		ctx, session.User.ID, server.ID, []domain.AbilityName{
			domain.AbilityNameGameServerCommon,
			domain.AbilityNameGameServerMetrics,
		},
	); err != nil {
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

	filter := serverIDFilter(server.ID)

	metricsbase.Pump(ctx, h.metricsHub, uint64(server.DSID), filter, client, defaultReplayWindow, h.logger)

	client.Run()
}

// serverIDFilter passes only series whose server_id label matches the
// requested server. Series without a server_id label (node-level metrics
// like CPU/RAM/disk and any other host metrics) are dropped — those
// belong to the node WS endpoint, not the per-server WS.
func serverIDFilter(serverID uint) metrics.SeriesFilter {
	wantedID := strconv.FormatUint(uint64(serverID), 10)

	return func(s *proto.MetricSeries) bool {
		raw, ok := s.GetLabels()["server_id"]
		if !ok {
			return false
		}

		return raw == wantedID
	}
}
