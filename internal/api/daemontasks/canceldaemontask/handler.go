package canceldaemontask

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/pkg/errors"
)

type Handler struct {
	daemonTaskRepo repositories.DaemonTaskRepository
	publisher      pubsub.Publisher
	responder      base.Responder
}

func NewHandler(
	daemonTaskRepo repositories.DaemonTaskRepository,
	publisher pubsub.Publisher,
	responder base.Responder,
) *Handler {
	return &Handler{
		daemonTaskRepo: daemonTaskRepo,
		publisher:      publisher,
		responder:      responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	taskID, err := api.NewInputReader(r).ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid task id"),
			http.StatusBadRequest,
		))

		return
	}

	tasks, err := h.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		IDs: []uint{taskID},
	}, nil, &filters.Pagination{
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find daemon task"))

		return
	}

	if len(tasks) == 0 {
		h.responder.WriteError(ctx, rw, api.NewNotFoundError("daemon task not found"))

		return
	}

	task := &tasks[0]

	if task.Status != domain.DaemonTaskStatusWaiting {
		h.responder.WriteError(ctx, rw, api.NewError(
			http.StatusUnprocessableEntity,
			"gdaemon_tasks.cancel_fail_cannot_be_canceled",
		))

		return
	}

	task.Status = domain.DaemonTaskStatusCanceled

	err = h.daemonTaskRepo.Save(ctx, task)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to cancel daemon task"))

		return
	}

	h.publishTaskStatus(ctx, task)

	h.responder.Write(ctx, rw, newCancelDaemonTaskResponse())
}

func (h *Handler) publishTaskStatus(ctx context.Context, task *domain.DaemonTask) {
	if h.publisher == nil {
		return
	}

	taskID := uint64(task.ID)

	channel := channels.BuildRealtimeTaskStatusChannel(taskID)
	msg, err := messages.NewMessage(channel, messages.TypeTaskStatus, messages.TaskStatusPayload{
		TaskID:   taskID,
		Status:   string(task.Status),
		ServerID: task.DedicatedServerID,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to create task status message", "error", err)

		return
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		slog.WarnContext(ctx, "failed to publish task status", "task_id", taskID, "error", err)
	}

	completeMsg, err := messages.NewMessage(channel, messages.TypeTaskComplete, messages.TaskCompletePayload{
		TaskID:   taskID,
		Status:   string(task.Status),
		ServerID: task.DedicatedServerID,
	})
	if err != nil {
		return
	}

	_ = h.publisher.Publish(ctx, channel, completeMsg)
}
