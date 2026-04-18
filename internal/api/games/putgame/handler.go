package putgame

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/services/serverconfigpush"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

var ErrGameNotFound = api.NewNotFoundError("game not found")

type Handler struct {
	repo         repositories.GameRepository
	serverRepo   repositories.ServerRepository
	configPusher *serverconfigpush.Pusher
	responder    base.Responder
}

func NewHandler(
	repo repositories.GameRepository,
	serverRepo repositories.ServerRepository,
	configPusher *serverconfigpush.Pusher,
	responder base.Responder,
) *Handler {
	return &Handler{
		repo:         repo,
		serverRepo:   serverRepo,
		configPusher: configPusher,
		responder:    responder,
	}
}

func (h *Handler) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	vars := mux.Vars(r)
	gameCode := vars["code"]

	if gameCode == "" {
		h.responder.WriteError(ctx, rw, api.NewValidationError("game code is required"))

		return
	}

	input := &updateGameInput{}

	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "invalid request"))

		return
	}

	err = input.Validate()
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "invalid input"))

		return
	}

	games, err := h.repo.Find(ctx, filters.FindGameByCodes(gameCode), nil, &filters.Pagination{
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to find game"))

		return
	}

	if len(games) == 0 {
		h.responder.WriteError(ctx, rw, ErrGameNotFound)

		return
	}

	game := &games[0]

	input.Apply(game)

	err = h.repo.Save(ctx, game)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to update game"))

		return
	}

	h.pushConfigForGameServers(ctx, game.Code)

	h.responder.Write(ctx, rw, base.Success)
}

func (h *Handler) pushConfigForGameServers(ctx context.Context, gameCode string) {
	if h.configPusher == nil || h.serverRepo == nil {
		return
	}

	servers, err := h.serverRepo.Find(ctx, &filters.FindServer{
		GameIDs: []string{gameCode},
	}, nil, nil)
	if err != nil {
		slog.Default().Warn("failed to load servers for game config push",
			"game_code", gameCode,
			"error", err,
		)

		return
	}

	for i := range servers {
		h.configPusher.PushServerConfig(ctx, servers[i].ID)
	}
}
