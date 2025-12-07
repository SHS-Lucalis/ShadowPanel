package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/serversettings"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type ServerSettingsServiceImpl struct {
	serverSettingRepo repositories.ServerSettingRepository
}

func NewServerSettingsService(
	serverSettingRepo repositories.ServerSettingRepository,
) *ServerSettingsServiceImpl {
	return &ServerSettingsServiceImpl{
		serverSettingRepo: serverSettingRepo,
	}
}

func (s *ServerSettingsServiceImpl) FindServerSettings(
	ctx context.Context,
	req *serversettings.FindServerSettingsRequest,
) (*serversettings.FindServerSettingsResponse, error) {
	filter := &filters.FindServerSetting{
		ServerIDs: []uint{uint(req.ServerId)},
		Names:     req.Names,
	}

	settings, err := s.serverSettingRepo.Find(ctx, filter, nil, nil)
	if err != nil {
		return nil, err
	}

	return &serversettings.FindServerSettingsResponse{
		Settings: convertServerSettingsToProto(settings),
	}, nil
}

func (s *ServerSettingsServiceImpl) SaveServerSetting(
	ctx context.Context,
	req *serversettings.SaveServerSettingRequest,
) (*serversettings.SaveServerSettingResponse, error) {
	setting := &domain.ServerSetting{
		ServerID: uint(req.ServerId),
		Name:     req.Name,
		Value:    domain.NewServerSettingValue(req.Value),
	}

	if err := s.serverSettingRepo.Save(ctx, setting); err != nil {
		return &serversettings.SaveServerSettingResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &serversettings.SaveServerSettingResponse{Success: true}, nil
}

func convertServerSettingsToProto(settings []domain.ServerSetting) []*proto.ServerSetting {
	return lo.Map(settings, func(s domain.ServerSetting, _ int) *proto.ServerSetting {
		value, _ := s.Value.String()

		return &proto.ServerSetting{
			Id:       uint64(s.ID),
			ServerId: uint64(s.ServerID),
			Name:     s.Name,
			Value:    value,
		}
	})
}

type ServerSettingsHostLibrary struct {
	impl *ServerSettingsServiceImpl
}

func NewServerSettingsHostLibrary(
	serverSettingRepo repositories.ServerSettingRepository,
) *ServerSettingsHostLibrary {
	return &ServerSettingsHostLibrary{
		impl: NewServerSettingsService(serverSettingRepo),
	}
}

func (l *ServerSettingsHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return serversettings.Instantiate(ctx, r, l.impl)
}
