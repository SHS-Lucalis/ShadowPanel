package updatetask

import (
	"context"
	"encoding/json"
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
	"github.com/gameap/gameap/pkg/auth"
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

	daemonSession := auth.DaemonSessionFromContext(ctx)
	if daemonSession == nil || daemonSession.Node == nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("daemon session not found"),
			http.StatusUnauthorized,
		))

		return
	}

	node := daemonSession.Node

	taskID, err := api.NewInputReader(r).ReadUint("gdaemon_task")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid task ID"),
			http.StatusBadRequest,
		))

		return
	}

	input := &updateTaskInput{}

	err = json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid request"),
			http.StatusBadRequest,
		))

		return
	}

	err = input.Validate()
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid input"),
			http.StatusBadRequest,
		))

		return
	}

	filter := &filters.FindDaemonTask{
		IDs:                []uint{taskID},
		DedicatedServerIDs: []uint{node.ID},
	}

	tasks, err := h.daemonTaskRepo.Find(ctx, filter, nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to find daemon task"),
			http.StatusInternalServerError,
		))

		return
	}

	if len(tasks) == 0 {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("daemon task not found"),
			http.StatusNotFound,
		))

		return
	}

	task := &tasks[0]

	task.Status = input.ToStatus()

	err = h.daemonTaskRepo.Save(ctx, task)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to update daemon task"),
			http.StatusInternalServerError,
		))

		return
	}

	h.publishTaskStatus(ctx, task)

	h.responder.Write(ctx, rw, newUpdateTaskResponse())
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

	if task.Status == domain.DaemonTaskStatusSuccess ||
		task.Status == domain.DaemonTaskStatusError ||
		task.Status == domain.DaemonTaskStatusCanceled {
		completeChannel := channels.BuildRealtimeTaskStatusChannel(taskID)
		completeMsg, err := messages.NewMessage(completeChannel, messages.TypeTaskComplete, messages.TaskCompletePayload{
			TaskID:   taskID,
			Status:   string(task.Status),
			ServerID: task.DedicatedServerID,
		})
		if err != nil {
			return
		}

		_ = h.publisher.Publish(ctx, completeChannel, completeMsg)
	}
}
