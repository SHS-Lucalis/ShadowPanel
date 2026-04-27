package getdaemontasks

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

//nolint:gochecknoglobals
var allowedSortFields = map[string]string{
	"id":                  "id",
	"dedicated_server_id": "dedicated_server_id",
	"server_id":           "server_id",
	"task":                "task",
	"status":              "status",
}

type Handler struct {
	daemonTasksRepo repositories.DaemonTaskRepository
	responder       base.Responder
}

func NewHandler(
	daemonTasksRepo repositories.DaemonTaskRepository,
	responder base.Responder,
) *Handler {
	return &Handler{
		daemonTasksRepo: daemonTasksRepo,
		responder:       responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("user not authenticated"),
			http.StatusUnauthorized,
		))

		return
	}

	input, err := readInput(r)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to read input"),
			http.StatusBadRequest,
		))

		return
	}

	filter := buildFilter(input)

	// Get total count for pagination
	total, err := h.daemonTasksRepo.Count(ctx, filter)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to count daemon tasks"))

		return
	}

	daemonTasks, err := h.daemonTasksRepo.Find(
		ctx,
		filter,
		buildSorting(input),
		buildPagination(input),
	)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find daemon tasks"))

		return
	}

	taskResponses := newDaemonTasksResponseFromDaemonTasks(daemonTasks)
	response := base.NewPaginatedResponse(taskResponses, input.PageNumber, input.PageSize, total)

	h.responder.Write(ctx, rw, response)
}

func buildFilter(input *input) *filters.FindDaemonTask {
	filter := &filters.FindDaemonTask{}

	if len(input.IDs) > 0 {
		filter.IDs = input.IDs
	}

	if len(input.DedicatedServerIDs) > 0 {
		filter.DedicatedServerIDs = input.DedicatedServerIDs
	}

	if len(input.ServerIDs) > 0 {
		serverIDs := make([]*uint, 0, len(input.ServerIDs))
		for _, id := range input.ServerIDs {
			serverID := id
			serverIDs = append(serverIDs, &serverID)
		}
		filter.ServerIDs = serverIDs
	}

	if len(input.Tasks) > 0 {
		filter.Tasks = input.Tasks
	}

	if len(input.Statuses) > 0 {
		filter.Statuses = input.Statuses
	}

	return filter
}

func buildSorting(input *input) []filters.Sorting {
	defaultSorting := []filters.Sorting{
		{Field: "created_at", Direction: filters.SortDirectionDesc},
		{Field: "id", Direction: filters.SortDirectionDesc},
	}

	sort, err := filters.ParseUserSort(input.Sort, allowedSortFields)
	if err != nil || sort == nil {
		return defaultSorting
	}

	return []filters.Sorting{*sort}
}

func buildPagination(input *input) *filters.Pagination {
	// Convert page-based pagination to offset-based
	offset := uint64((input.PageNumber - 1) * input.PageSize) //nolint:gosec // PageNumber and PageSize are validated

	return &filters.Pagination{
		Limit:  uint64(input.PageSize), //nolint:gosec // PageSize is validated
		Offset: offset,
	}
}
