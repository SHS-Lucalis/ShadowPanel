package appendoutput

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
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

	input := &appendOutputInput{}

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

	exists, err := h.daemonTaskRepo.Exists(ctx, filter)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to check daemon task existence"),
			http.StatusInternalServerError,
		))

		return
	}

	if !exists {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.New("daemon task not found"),
			http.StatusNotFound,
		))

		return
	}

	err = h.daemonTaskRepo.AppendOutput(ctx, taskID, input.Output)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to append output to daemon task"),
			http.StatusInternalServerError,
		))

		return
	}

	h.publishTaskOutput(ctx, taskID, input.Output)

	h.responder.Write(ctx, rw, newAppendOutputResponse())
}

func (h *Handler) publishTaskOutput(ctx context.Context, taskID uint, output string) {
	if h.publisher == nil {
		return
	}

	id := uint64(taskID)
	channel := channels.BuildRealtimeTaskOutputChannel(id)

	msg, err := messages.NewMessage(channel, messages.TypeTaskOutput, messages.TaskOutputPayload{
		TaskID: id,
		Chunk:  output,
	})
	if err != nil {
		slog.WarnContext(ctx, "failed to create task output message", "error", err)

		return
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		slog.WarnContext(ctx, "failed to publish task output", "task_id", id, "error", err)
	}
}
