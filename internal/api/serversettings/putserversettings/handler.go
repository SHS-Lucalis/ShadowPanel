package putserversettings

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gameap/gameap/internal/api/base"
	serversbase "github.com/gameap/gameap/internal/api/servers/base"
	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/internal/services/serverconfigpush"
	"github.com/gameap/gameap/pkg/api"
	"github.com/gameap/gameap/pkg/auth"
	"github.com/pkg/errors"
)

const (
	autostartSettingKey         = "autostart"
	autostartCurrentSettingKey  = "autostart_current"
	updateBeforeStartSettingKey = "update_before_start"
)

type Handler struct {
	serverSettingsRepo repositories.ServerSettingRepository
	serverFinder       *serversbase.ServerFinder
	abilityChecker     *serversbase.AbilityChecker
	gameModsRepo       repositories.GameModRepository
	configPusher       *serverconfigpush.Pusher
	rbac               base.RBAC
	responder          base.Responder
}

func NewHandler(
	serverSettingsRepo repositories.ServerSettingRepository,
	serverRepo repositories.ServerRepository,
	gameModsRepo repositories.GameModRepository,
	configPusher *serverconfigpush.Pusher,
	rbac base.RBAC,
	responder base.Responder,
) *Handler {
	return &Handler{
		serverSettingsRepo: serverSettingsRepo,
		serverFinder:       serversbase.NewServerFinder(serverRepo, rbac),
		abilityChecker:     serversbase.NewAbilityChecker(rbac),
		gameModsRepo:       gameModsRepo,
		configPusher:       configPusher,
		rbac:               rbac,
		responder:          responder,
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

	serverID, err := input.ReadUint("server")
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

	isAdmin, err := h.rbac.Can(ctx, session.User.ID, []domain.AbilityName{domain.AbilityNameAdminRolesPermissions})
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to check admin permissions"))

		return
	}

	err = h.abilityChecker.CheckOrError(
		ctx,
		session.User.ID,
		server.ID,
		[]domain.AbilityName{domain.AbilityNameGameServerCommon, domain.AbilityNameGameServerSettings},
	)
	if err != nil {
		h.responder.WriteError(ctx, rw, err)

		return
	}

	var settingsInput saveSettingsInput
	err = json.NewDecoder(r.Body).Decode(&settingsInput)
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			errors.WithMessage(err, "failed to read request body"),
			http.StatusBadRequest,
		))

		return
	}

	err = settingsInput.Validate()
	if err != nil {
		h.responder.WriteError(ctx, rw, api.WrapHTTPError(
			err,
			http.StatusBadRequest,
		))

		return
	}

	settingsInputMap := settingsInput.ToSettingsMap()

	err = h.saveSettings(ctx, server, settingsInputMap, isAdmin)
	if err != nil {
		h.responder.WriteError(ctx, rw, errors.WithMessage(err, "failed to save settings"))

		return
	}

	if h.configPusher != nil {
		h.configPusher.PushServerConfig(ctx, server.ID)
	}

	h.responder.Write(ctx, rw, SuccessResponse{})
}

func (h *Handler) findGameMod(ctx context.Context, gameModID uint) (*domain.GameMod, error) {
	gameMods, err := h.gameModsRepo.Find(ctx, &filters.FindGameMod{
		IDs: []uint{gameModID},
	}, nil, &filters.Pagination{
		Limit:  1,
		Offset: 0,
	})
	if err != nil {
		return nil, err
	}

	if len(gameMods) == 0 {
		return nil, nil
	}

	return &gameMods[0], nil
}

func (h *Handler) buildAllowedSettings(gameMod *domain.GameMod, isAdmin bool) map[string]settingMetadata {
	allowedSettings := make(map[string]settingMetadata)

	allowedSettings[autostartSettingKey] = settingMetadata{
		name:     autostartSettingKey,
		adminVar: false,
	}

	allowedSettings[updateBeforeStartSettingKey] = settingMetadata{
		name:     updateBeforeStartSettingKey,
		adminVar: false,
	}

	if gameMod != nil {
		for _, gmVar := range gameMod.Vars {
			if gmVar.AdminVar && !isAdmin {
				continue
			}

			allowedSettings[gmVar.Var] = settingMetadata{
				name:     gmVar.Var,
				adminVar: gmVar.AdminVar,
			}
		}
	}

	return allowedSettings
}

func (h *Handler) saveSettings(
	ctx context.Context,
	server *domain.Server,
	settingsInputMap map[string]any,
	isAdmin bool,
) error {
	gameMod, err := h.findGameMod(ctx, server.GameModID)
	if err != nil {
		return errors.WithMessage(err, "failed to find game mod")
	}
	if gameMod == nil {
		return api.NewNotFoundError("game mod not found")
	}

	allowedSettings := h.buildAllowedSettings(gameMod, isAdmin)

	existingSettings, err := h.serverSettingsRepo.Find(ctx, &filters.FindServerSetting{
		ServerIDs: []uint{server.ID},
	}, nil, nil)
	if err != nil {
		return errors.WithMessage(err, "failed to find server settings")
	}

	existingSettingsMap := make(map[string]*domain.ServerSetting)
	for i := range existingSettings {
		existingSettingsMap[existingSettings[i].Name] = &existingSettings[i]
	}

	for settingName, settingValue := range settingsInputMap {
		allowedSetting, isAllowed := allowedSettings[settingName]
		if !isAllowed {
			continue
		}

		if allowedSetting.adminVar && !isAdmin {
			continue
		}

		existingSetting, exists := existingSettingsMap[settingName]
		if exists {
			updatedSetting := &domain.ServerSetting{
				ID:       existingSetting.ID,
				ServerID: server.ID,
				Name:     settingName,
				Value:    domain.NewServerSettingValue(settingValue),
			}
			err := h.serverSettingsRepo.Save(ctx, updatedSetting)
			if err != nil {
				return errors.WithMessage(err, "failed to update setting")
			}
		} else {
			newSetting := &domain.ServerSetting{
				ServerID: server.ID,
				Name:     settingName,
				Value:    domain.NewServerSettingValue(settingValue),
			}
			err := h.serverSettingsRepo.Save(ctx, newSetting)
			if err != nil {
				return errors.WithMessage(err, "failed to create setting")
			}
		}
	}

	return nil
}

type settingMetadata struct {
	name     string
	adminVar bool
}
