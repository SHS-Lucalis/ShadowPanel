package handlers

import (
	"context"
	"log/slog"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

type ServerStatusHandler struct {
	serverRepo repositories.ServerRepository
	logger     *slog.Logger
}

func NewServerStatusHandler(serverRepo repositories.ServerRepository, logger *slog.Logger) *ServerStatusHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &ServerStatusHandler{
		serverRepo: serverRepo,
		logger:     logger,
	}
}

func (h *ServerStatusHandler) HandleServerStatuses(
	ctx context.Context,
	nodeID uint64,
	batch *proto.ServerStatusBatch,
) error {
	if batch == nil || len(batch.Statuses) == 0 {
		return nil
	}

	serverIDs := make([]uint, 0, len(batch.Statuses))
	for _, status := range batch.Statuses {
		serverIDs = append(serverIDs, uint(status.ServerId))
	}

	servers, err := h.serverRepo.Find(ctx, &filters.FindServer{
		IDs:   serverIDs,
		DSIDs: []uint{uint(nodeID)},
	}, nil, nil)
	if err != nil {
		return errors.Wrap(err, "find servers for status update")
	}

	serverMap := make(map[uint]int)
	for i, srv := range servers {
		serverMap[srv.ID] = i
	}

	now := time.Now()
	for _, status := range batch.Statuses {
		idx, ok := serverMap[uint(status.ServerId)]
		if !ok {
			h.logger.Warn("server not found for status update",
				"server_id", status.ServerId,
				"node_id", nodeID,
			)

			continue
		}

		servers[idx].ProcessActive = status.IsRunning
		servers[idx].LastProcessCheck = &now

		h.logger.Debug("server status updated",
			"server_id", status.ServerId,
			"is_running", status.IsRunning,
		)
	}

	updatedServers := make([]*domain.Server, 0, len(servers))
	for i := range servers {
		updatedServers = append(updatedServers, &servers[i])
	}

	if err := h.serverRepo.SaveBulk(ctx, updatedServers); err != nil {
		return errors.Wrap(err, "save server statuses")
	}

	h.logger.Debug("processed server status batch",
		"node_id", nodeID,
		"count", len(batch.Statuses),
	)

	return nil
}
