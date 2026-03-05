package getserver

import (
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

type Handler struct {
	serverFinder      *serversbase.ServerFinder
	gameRepo          repositories.GameRepository
	gameModRepo       repositories.GameModRepository
	serverSettingRepo repositories.ServerSettingRepository
	rbac              base.RBAC
	responder         base.Responder
}

func NewHandler(
	serverRepo repositories.ServerRepository,
	gameRepo repositories.GameRepository,
	gameModRepo repositories.GameModRepository,
	serverSettingRepo repositories.ServerSettingRepository,
	rbac base.RBAC,
	responder base.Responder,
) *Handler {
	return &Handler{
		serverFinder:      serversbase.NewServerFinder(serverRepo, rbac),
		gameRepo:          gameRepo,
		gameModRepo:       gameModRepo,
		serverSettingRepo: serverSettingRepo,
		rbac:              rbac,
		responder:         responder,
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

	input := api.NewInputReader(r)

	serverID, err := input.ReadUint("id")
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "invalid server id"),
			http.StatusBadRequest,
		))

		return
	}

	server, err := h.serverFinder.FindUserServer(ctx, session.User, serverID)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	games, err := h.gameRepo.Find(ctx, filters.FindGameByCodes(server.GameID), nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to fetch game"),
			http.StatusInternalServerError,
		))

		return
	}

	var game *domain.Game
	if len(games) > 0 {
		game = &games[0]
	}

	gameMods, err := h.gameModRepo.Find(ctx, &filters.FindGameMod{IDs: []uint{server.GameModID}}, nil, nil)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to fetch game mod"),
			http.StatusInternalServerError,
		))

		return
	}

	var gameMod *domain.GameMod
	if len(gameMods) > 0 {
		gameMod = &gameMods[0]
	}

	settings, err := h.serverSettingRepo.Find(
		ctx,
		&filters.FindServerSetting{ServerIDs: []uint{server.ID}},
		nil,
		nil,
	)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to fetch server settings"),
			http.StatusInternalServerError,
		))

		return
	}

	isAdmin, err := h.rbac.Can(ctx, session.User.ID, []domain.AbilityName{domain.AbilityNameAdminRolesPermissions})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to check admin permissions"))

		return
	}

	if isAdmin {
		h.responder.Write(ctx, rw, newAdminServerResponseFromServer(server, game, gameMod, settings))

		return
	}

	h.responder.Write(ctx, rw, newUserServerResponseFromServer(server, game))
}
