package getservers

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type Handler struct {
	serverRepo repositories.ServerRepository
	gameRepo   repositories.GameRepository
	rbac       base.RBAC
	responder  base.Responder
}

func NewHandler(
	serverRepo repositories.ServerRepository,
	gameRepo repositories.GameRepository,
	rbac base.RBAC,
	responder base.Responder,
) *Handler {
	return &Handler{
		serverRepo: serverRepo,
		gameRepo:   gameRepo,
		rbac:       rbac,
		responder:  responder,
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

	isAdmin, err := h.rbac.Can(ctx, session.User.ID, []domain.AbilityName{domain.AbilityNameAdminRolesPermissions})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to check admin permissions"))

		return
	}

	filter := buildFilter(input, session.User.ID, isAdmin)

	total, err := h.serverRepo.Count(ctx, filter)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to count servers"))

		return
	}

	servers, err := h.serverRepo.Find(ctx, filter, buildSorting(input), buildPagination(input))
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find user servers"))

		return
	}

	games, err := h.gameRepo.FindAll(ctx, nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to load games"))

		return
	}

	serversResponse := newServersResponseFromServers(servers, games)
	response := base.NewPaginatedResponse(serversResponse, input.PageNumber, input.PageSize, total)

	h.responder.Write(ctx, rw, response)
}
