package taskstatus

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	stderrors "errors"

	"github.com/coder/websocket"
	"github.com/gameap/gameap/internal/api/base"
	daemontasksbase "github.com/gameap/gameap/internal/api/daemontasks/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

const (
	typeTaskStatus   = "task.status"
	typeTaskOutput   = "task.output"
	typeTaskComplete = "task.complete"
)

type Handler struct {
	daemonTaskRepo repositories.DaemonTaskRepository
	serverFinder   *serversbase.ServerFinder
	abilityChecker *serversbase.AbilityChecker
	rbac           base.RBAC
	hub            *ws.Hub
	originPatterns []string
	responder      base.Responder
	logger         *slog.Logger
}

func NewHandler(
	daemonTaskRepo repositories.DaemonTaskRepository,
	serverRepo repositories.ServerRepository,
	rbac base.RBAC,
	hub *ws.Hub,
	originPatterns []string,
	responder base.Responder,
) *Handler {
	return &Handler{
		daemonTaskRepo: daemonTaskRepo,
		serverFinder:   serversbase.NewServerFinder(serverRepo, rbac),
		abilityChecker: serversbase.NewAbilityChecker(rbac),
		rbac:           rbac,
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

	taskID, err := input.ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid task id"),
			http.StatusBadRequest,
		))

		return
	}

	task, err := h.findAndAuthorize(ctx, session.User, taskID)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	conn, err := websocket.Accept(rw, r, &websocket.AcceptOptions{
		OriginPatterns: h.originPatterns,
	})
	if err != nil {
		h.logger.Warn("websocket accept failed", "error", err)

		return
	}

	client := ws.NewClient(ctx, conn, h.hub, nil, h.logger)

	taskIDStr := strconv.FormatUint(uint64(taskID), 10)
	statusTopic := ws.ChannelToTopic(channels.BuildRealtimeTaskStatusChannel(uint64(taskID)))
	outputTopic := ws.ChannelToTopic(channels.BuildRealtimeTaskOutputChannel(uint64(taskID)))
	h.hub.Register(client, statusTopic, outputTopic)

	h.logger.Debug("task status websocket connected", "task_id", taskIDStr)

	h.sendInitialState(client, task)

	client.Run()
}

func (h *Handler) findAndAuthorize(ctx context.Context, user *domain.User, taskID uint) (*domain.DaemonTask, error) {
	tasks, err := h.daemonTaskRepo.FindWithOutput(ctx, &filters.FindDaemonTask{
		IDs: []uint{taskID},
	}, nil, &filters.Pagination{
		Limit: 1,
	})
	if err != nil {
		return nil, errors.WithMessage(err, "failed to find daemon task")
	}

	if len(tasks) == 0 {
		return nil, api.NewNotFoundError("daemon task not found")
	}

	task := &tasks[0]

	if err := h.checkAuthorization(ctx, user, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (h *Handler) checkAuthorization(ctx context.Context, user *domain.User, task *domain.DaemonTask) error {
	isAdmin, err := h.rbac.Can(ctx, user.ID, []domain.AbilityName{domain.AbilityNameAdminRolesPermissions})
	if err != nil {
		return errors.WithMessage(err, "failed to check admin permissions")
	}

	if isAdmin {
		return nil
	}

	if task.ServerID == nil {
		return api.NewError(http.StatusForbidden, "access denied: task has no associated server")
	}

	requiredAbilities, ok := daemontasksbase.DaemonTaskTypeAbilities[task.Task]
	if !ok {
		return api.NewError(http.StatusForbidden, "access denied: task type not allowed for regular users")
	}

	_, err = h.serverFinder.FindUserServer(ctx, user, *task.ServerID)
	if err != nil {
		if target, ok := stderrors.AsType[*api.Error](err); ok {
			if target.HTTPStatus() == http.StatusNotFound {
				return api.NewError(http.StatusForbidden, "access denied: no access to the server")
			}
		}

		return errors.WithMessage(err, "failed to find server")
	}

	if err := h.abilityChecker.CheckOrError(ctx, user.ID, *task.ServerID, requiredAbilities); err != nil {
		return err
	}

	return nil
}

func (h *Handler) sendInitialState(client *ws.Client, task *domain.DaemonTask) {
	client.SendMessage(ws.NewOutboundMessage(typeTaskStatus, taskStatusPayload{
		TaskID:   task.ID,
		Status:   string(task.Status),
		ServerID: task.ServerID,
	}))

	if task.Output != nil && *task.Output != "" {
		client.SendMessage(ws.NewOutboundMessage(typeTaskOutput, taskOutputPayload{
			TaskID: task.ID,
			Chunk:  *task.Output,
		}))
	}

	if isTerminalStatus(task.Status) {
		client.SendMessage(ws.NewOutboundMessage(typeTaskComplete, taskCompletePayload{
			TaskID: task.ID,
			Status: string(task.Status),
		}))
	}
}

func isTerminalStatus(status domain.DaemonTaskStatus) bool {
	return status == domain.DaemonTaskStatusSuccess ||
		status == domain.DaemonTaskStatusError ||
		status == domain.DaemonTaskStatusCanceled
}

type taskStatusPayload struct {
	TaskID   uint   `json:"task_id"`
	Status   string `json:"status"`
	ServerID *uint  `json:"server_id,omitempty"`
}

type taskOutputPayload struct {
	TaskID  uint   `json:"task_id"`
	Chunk   string `json:"chunk"`
	IsFinal bool   `json:"is_final,omitempty"`
}

type taskCompletePayload struct {
	TaskID uint   `json:"task_id"`
	Status string `json:"status"`
}
