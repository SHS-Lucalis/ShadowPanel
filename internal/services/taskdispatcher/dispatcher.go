package taskdispatcher

import (
	"context"
	"log/slog"
	"time"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/grpc/gateway"
	"github.com/gameap/gameap/internal/grpc/session"
	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

type Dispatcher struct {
	registry       *session.Registry
	daemonTaskRepo repositories.DaemonTaskRepository
	publisher      pubsub.Publisher
	logger         *slog.Logger
}

func NewDispatcher(
	registry *session.Registry,
	daemonTaskRepo repositories.DaemonTaskRepository,
	publisher pubsub.Publisher,
	logger *slog.Logger,
) *Dispatcher {
	if logger == nil {
		logger = slog.Default()
	}

	return &Dispatcher{
		registry:       registry,
		daemonTaskRepo: daemonTaskRepo,
		publisher:      publisher,
		logger:         logger,
	}
}

func (d *Dispatcher) Dispatch(ctx context.Context, task *domain.DaemonTask) error {
	if err := d.daemonTaskRepo.Save(ctx, task); err != nil {
		return errors.Wrap(err, "persist task")
	}

	protoTask := gateway.DomainDaemonTaskToProto(task)
	msg := &proto.GatewayMessage{
		RequestId: generateRequestID(task.ID),
		Payload: &proto.GatewayMessage_Task{
			Task: protoTask,
		},
	}

	err := d.registry.SendTask(ctx, uint64(task.DedicatedServerID), msg)
	if err == nil {
		if err := d.daemonTaskRepo.Save(ctx, task); err != nil {
			d.logger.Warn("failed to update task status after dispatch",
				"task_id", task.ID,
				"error", err,
			)
		}
		d.logger.Info("task dispatched",
			"task_id", task.ID,
			"node_id", task.DedicatedServerID,
		)

		return nil
	}

	d.logger.Warn("task dispatch failed, will retry on reconnect",
		"task_id", task.ID,
		"node_id", task.DedicatedServerID,
		"error", err,
	)

	return nil
}

func (d *Dispatcher) FlushPending(ctx context.Context, nodeID uint64) error {
	sess, ok := d.registry.GetSession(nodeID)
	if !ok {
		return errors.New("session not found")
	}

	tasks, err := d.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		DedicatedServerIDs: []uint{uint(nodeID)},
		Statuses:           []domain.DaemonTaskStatus{domain.DaemonTaskStatusWaiting},
	}, nil, nil)
	if err != nil {
		return errors.Wrap(err, "find pending tasks")
	}

	for i := range tasks {
		task := &tasks[i]
		protoTask := gateway.DomainDaemonTaskToProto(task)
		msg := &proto.GatewayMessage{
			RequestId: generateRequestID(task.ID),
			Payload: &proto.GatewayMessage_Task{
				Task: protoTask,
			},
		}

		if err := sess.Send(msg); err != nil {
			d.logger.Error("failed to send pending task",
				"task_id", task.ID,
				"node_id", nodeID,
				"error", err,
			)

			return errors.Wrap(err, "send pending task")
		}

		task.Status = domain.DaemonTaskStatusWorking
		if err := d.daemonTaskRepo.Save(ctx, task); err != nil {
			d.logger.Warn("failed to update task status",
				"task_id", task.ID,
				"error", err,
			)
		}
	}

	d.logger.Info("flushed pending tasks",
		"node_id", nodeID,
		"count", len(tasks),
	)

	return nil
}

func (d *Dispatcher) GetPendingTasks(ctx context.Context, nodeID uint64) ([]*proto.DaemonTask, error) {
	tasks, err := d.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
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

	return protoTasks, nil
}

func (d *Dispatcher) HandleTaskStatusUpdate(ctx context.Context, nodeID uint64, update *proto.TaskStatusUpdate) error {
	tasks, err := d.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		IDs: []uint{uint(update.TaskId)},
	}, nil, nil)
	if err != nil {
		return errors.Wrap(err, "find task")
	}

	if len(tasks) == 0 {
		d.logger.Warn("task not found for status update",
			"task_id", update.TaskId,
			"node_id", nodeID,
		)

		return nil
	}

	task := &tasks[0]

	if uint(nodeID) != task.DedicatedServerID {
		d.logger.Warn("task status update from wrong node",
			"task_id", update.TaskId,
			"expected_node_id", task.DedicatedServerID,
			"actual_node_id", nodeID,
		)

		return nil
	}

	task.Status = gateway.ProtoTaskStatusToDomain(update.Status)
	if err := d.daemonTaskRepo.Save(ctx, task); err != nil {
		return errors.Wrap(err, "update task status")
	}

	d.publishTaskStatus(ctx, update.TaskId, string(task.Status), task.DedicatedServerID, update.Message)

	d.logger.Debug("task status updated",
		"task_id", task.ID,
		"status", task.Status,
	)

	return nil
}

func (d *Dispatcher) HandleTaskOutput(ctx context.Context, _ uint64, output *proto.TaskOutput) error {
	if len(output.OutputChunk) == 0 {
		return nil
	}

	if err := d.daemonTaskRepo.AppendOutput(ctx, uint(output.TaskId), string(output.OutputChunk)); err != nil {
		return errors.Wrap(err, "append task output")
	}

	d.publishTaskOutput(ctx, output.TaskId, string(output.OutputChunk), output.IsFinal)

	return nil
}

func (d *Dispatcher) CancelTask(ctx context.Context, taskID uint64, reason string) error {
	tasks, err := d.daemonTaskRepo.Find(ctx, &filters.FindDaemonTask{
		IDs: []uint{uint(taskID)},
	}, nil, nil)
	if err != nil {
		return errors.Wrap(err, "find task")
	}

	if len(tasks) == 0 {
		return errors.New("task not found")
	}

	task := &tasks[0]
	nodeID := uint64(task.DedicatedServerID)

	msg := &proto.GatewayMessage{
		RequestId: generateRequestID(task.ID),
		Payload: &proto.GatewayMessage_TaskCancel{
			TaskCancel: &proto.TaskCancel{
				TaskId: taskID,
				Reason: reason,
			},
		},
	}

	if err := d.registry.SendTask(ctx, nodeID, msg); err != nil {
		d.logger.Warn("failed to send task cancel",
			"task_id", taskID,
			"node_id", nodeID,
			"error", err,
		)
	}

	task.Status = domain.DaemonTaskStatusCanceled
	if err := d.daemonTaskRepo.Save(ctx, task); err != nil {
		return errors.Wrap(err, "update task status")
	}

	return nil
}

func (d *Dispatcher) publishTaskStatus(
	ctx context.Context, taskID uint64, status string, serverID uint, message string,
) {
	if d.publisher == nil {
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
		d.logger.Warn("failed to create task status message", "error", err)

		return
	}

	if err := d.publisher.Publish(ctx, channel, msg); err != nil {
		d.logger.Warn("failed to publish task status", "task_id", taskID, "error", err)
	}
}

func (d *Dispatcher) publishTaskOutput(ctx context.Context, taskID uint64, chunk string, isFinal bool) {
	if d.publisher == nil {
		return
	}

	channel := channels.BuildRealtimeTaskOutputChannel(taskID)

	msg, err := messages.NewMessage(channel, messages.TypeTaskOutput, messages.TaskOutputPayload{
		TaskID:  taskID,
		Chunk:   chunk,
		IsFinal: isFinal,
	})
	if err != nil {
		d.logger.Warn("failed to create task output message", "error", err)

		return
	}

	if err := d.publisher.Publish(ctx, channel, msg); err != nil {
		d.logger.Warn("failed to publish task output", "task_id", taskID, "error", err)
	}
}

func generateRequestID(taskID uint) string {
	return time.Now().Format("20060102150405") + "-task-" + uintToString(taskID)
}

func uintToString(n uint) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}

	return string(buf[i:])
}
