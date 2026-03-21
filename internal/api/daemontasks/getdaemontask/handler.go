package getdaemontask

import (
	"net/http"

	stderrors "errors"

	"github.com/gameap/gameap/internal/api/base"
	daemontasksbase "github.com/gameap/gameap/internal/api/daemontasks/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type Handler struct {
	daemonTasksRepo repositories.DaemonTaskRepository
	serverFinder    *serversbase.ServerFinder
	abilityChecker  *serversbase.AbilityChecker
	rbac            base.RBAC
	responder       base.Responder
	withOutput      bool
}

func NewHandler(
	daemonTasksRepo repositories.DaemonTaskRepository,
	serverRepo repositories.ServerRepository,
	rbac base.RBAC,
	responder base.Responder,
	withOutput bool,
) *Handler {
	return &Handler{
		daemonTasksRepo: daemonTasksRepo,
		serverFinder:    serversbase.NewServerFinder(serverRepo, rbac),
		abilityChecker:  serversbase.NewAbilityChecker(rbac),
		rbac:            rbac,
		responder:       responder,
		withOutput:      withOutput,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	session := auth.SessionFromContext(ctx)
	if !session.IsAuthenticated() {
		h.responder.WriteError(ctx, rw, api.NewError(http.StatusUnauthorized, "user not authenticated"))

		return
	}

	inputReader := api.NewInputReader(r)

	taskID, err := inputReader.ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid task id"),
			http.StatusBadRequest,
		))

		return
	}

	filter := &filters.FindDaemonTask{
		IDs: []uint{taskID},
	}

	var tasks []domain.DaemonTask

	if h.withOutput {
		tasks, err = h.daemonTasksRepo.FindWithOutput(ctx, filter, nil, &filters.Pagination{
			Limit:  1,
			Offset: 0,
		})
	} else {
		tasks, err = h.daemonTasksRepo.Find(ctx, filter, nil, &filters.Pagination{
			Limit:  1,
			Offset: 0,
		})
	}
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find daemon task"))

		return
	}

	if len(tasks) == 0 {
		h.responder.WriteError(ctx, rw, api.NewNotFoundError("daemon task not found"))

		return
	}

	task := &tasks[0]

	if err := h.checkAuthorization(r, session.User, task); err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	response := newDaemonTaskOutputResponseFromDaemonTask(task)

	h.responder.Write(ctx, rw, response)
}

func (h *Handler) checkAuthorization(r *http.Request, user *domain.User, task *domain.DaemonTask) error {
	ctx := r.Context()

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
