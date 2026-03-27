package handlers

import (
	"context"
	"log/slog"
	"time"

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

	now := time.Now()
	statuses := make([]repositories.ServerStatusUpdate, 0, len(batch.Statuses))

	for _, s := range batch.Statuses {
		statuses = append(statuses, repositories.ServerStatusUpdate{
			ID:               uint(s.ServerId),
			ProcessActive:    s.IsRunning,
			LastProcessCheck: now,
		})

		h.logger.Debug("server status received",
			"server_id", s.ServerId,
			"is_running", s.IsRunning,
		)
	}

	if err := h.serverRepo.UpdateServerStatuses(ctx, uint(nodeID), statuses); err != nil {
		return errors.Wrap(err, "update server statuses")
	}

	h.logger.Debug("processed server status batch",
		"node_id", nodeID,
		"count", len(batch.Statuses),
	)

	return nil
}
