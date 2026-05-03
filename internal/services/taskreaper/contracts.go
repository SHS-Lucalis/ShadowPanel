package taskreaper

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
)

type DaemonTaskRepository interface {
	Find(
		ctx context.Context,
		filter *filters.FindDaemonTask,
		order []filters.Sorting,
		pagination *filters.Pagination,
	) ([]domain.DaemonTask, error)
}

type SessionRegistry interface {
	IsConnectedAnywhere(nodeID uint64) bool
}

type TaskReconciler interface {
	ReconcileWorkingTasks(ctx context.Context, nodeID uint64, inFlightIDs []uint64, reason string) (int, error)
}
