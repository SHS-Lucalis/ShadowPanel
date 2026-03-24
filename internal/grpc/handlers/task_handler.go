package handlers

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/grpc/gateway"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

type TaskHandler struct {
	daemonTaskRepo repositories.DaemonTaskRepository
	publisher      pubsub.Publisher
	logger         *slog.Logger
}

func NewTaskHandler(
	daemonTaskRepo repositories.DaemonTaskRepository,
	publisher pubsub.Publisher,
	logger *slog.Logger,
) *TaskHandler {
	if logger == nil {
		logger = slog.Default()
	}

	return &TaskHandler{
		daemonTaskRepo: daemonTaskRepo,
		publisher:      publisher,
		logger:         logger,
	}
}

func (h *TaskHandler) HandleTaskStatusUpdate(ctx context.Context, nodeID uint64, update *proto.TaskStatusUpdate) error {
	status := gateway.ProtoTaskStatusToDomain(update.Status)

	tasks, err := h.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		IDs: []uint{uint(update.TaskId)},
	}, nil, nil)
	if err != nil {
		return errors.Wrap(err, "find task")
	}

	if len(tasks) == 0 {
		h.logger.Warn("task not found for status update",
			"task_id", update.TaskId,
			"node_id", nodeID,
		)

		return nil
	}

	task := tasks[0]
	task.Status = status

	if err := h.daemonTaskRepo.Save(ctx, &task); err != nil {
		return errors.Wrap(err, "save task status")
	}

	h.logger.Info("publishing task status to pubsub",
		"task_id", update.TaskId,
		"status", string(task.Status),
		"server_id", task.DedicatedServerID,
	)

	h.publishTaskStatus(ctx, update.TaskId, string(task.Status), task.DedicatedServerID, update.Message)

	if isTerminalStatus(task.Status) {
		h.publishTaskComplete(ctx, update.TaskId, string(task.Status), task.DedicatedServerID)
	}

	h.logger.Debug("task status updated",
		"task_id", update.TaskId,
		"status", status,
		"message", update.Message,
	)

	return nil
}

func (h *TaskHandler) HandleTaskOutput(ctx context.Context, _ uint64, output *proto.TaskOutput) error {
	if len(output.OutputChunk) == 0 {
		return nil
	}

	if err := h.daemonTaskRepo.AppendOutput(ctx, uint(output.TaskId), string(output.OutputChunk)); err != nil {
		return errors.Wrap(err, "append task output")
	}

	h.publishTaskOutput(ctx, output.TaskId, string(output.OutputChunk), output.IsFinal)

	h.logger.Debug("task output appended",
		"task_id", output.TaskId,
		"bytes", len(output.OutputChunk),
		"is_final", output.IsFinal,
	)

	return nil
}

func (h *TaskHandler) GetPendingTasks(ctx context.Context, nodeID uint64) ([]*proto.DaemonTask, error) {
	tasks, err := h.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		DedicatedServerIDs: []uint{uint(nodeID)},
		Statuses:           []domain.DaemonTaskStatus{domain.DaemonTaskStatusWaiting},
	}, nil, nil)
	if err != nil {
		return nil, errors.Wrap(err, "find pending tasks")
	}

	protoTasks := make([]*proto.DaemonTask, 0, len(tasks))
	for i := range tasks {
		protoTasks = append(protoTasks, gateway.DomainDaemonTaskToProto(&tasks[i]))
	}

	h.logger.Debug("retrieved pending tasks",
		"node_id", nodeID,
		"count", len(protoTasks),
	)

	return protoTasks, nil
}

func (h *TaskHandler) publishTaskStatus(
	ctx context.Context, taskID uint64, status string, serverID uint, message string,
) {
	if h.publisher == nil {
		return
	}

	channel := channels.BuildRealtimeTaskStatusChannel(taskID)

	msg, err := messages.NewMessage(channel, messages.TypeTaskStatus, messages.TaskStatusPayload{
		TaskID:   taskID,
		Status:   status,
		ServerID: serverID,
		Message:  message,
	})
	if err != nil {
		h.logger.Warn("failed to create task status message", "error", err)

		return
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish task status", "task_id", taskID, "error", err)
	}
}

func (h *TaskHandler) publishTaskComplete(ctx context.Context, taskID uint64, status string, serverID uint) {
	if h.publisher == nil {
		return
	}

	channel := channels.BuildRealtimeTaskStatusChannel(taskID)

	msg, err := messages.NewMessage(channel, messages.TypeTaskComplete, messages.TaskCompletePayload{
		TaskID:   taskID,
		Status:   status,
		ServerID: serverID,
	})
	if err != nil {
		h.logger.Warn("failed to create task complete message", "error", err)

		return
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish task complete", "task_id", taskID, "error", err)
	}
}

func (h *TaskHandler) publishTaskOutput(ctx context.Context, taskID uint64, chunk string, isFinal bool) {
	if h.publisher == nil {
		return
	}

	channel := channels.BuildRealtimeTaskOutputChannel(taskID)

	msg, err := messages.NewMessage(channel, messages.TypeTaskOutput, messages.TaskOutputPayload{
		TaskID:  taskID,
		Chunk:   chunk,
		IsFinal: isFinal,
	})
	if err != nil {
		h.logger.Warn("failed to create task output message", "error", err)

		return
	}

	if err := h.publisher.Publish(ctx, channel, msg); err != nil {
		h.logger.Warn("failed to publish task output", "task_id", taskID, "error", err)
	}
}

func isTerminalStatus(status domain.DaemonTaskStatus) bool {
	return status == domain.DaemonTaskStatusSuccess ||
		status == domain.DaemonTaskStatusError ||
		status == domain.DaemonTaskStatusCanceled
}
