package hostlibrary

import (
	"context"

	"github.com/gameap/gameap/internal/domain"
	"github.com/gameap/gameap/internal/filters"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/plugin/sdk/servercontrol"
	"github.com/samber/lo"
	"github.com/tetratelabs/wazero"
)

type ServerController interface {
	Start(ctx context.Context, server *domain.Server) (uint, error)
	Stop(ctx context.Context, server *domain.Server) (uint, error)
	Restart(ctx context.Context, server *domain.Server) (uint, error)
	Update(ctx context.Context, server *domain.Server) (uint, error)
	Install(ctx context.Context, server *domain.Server) (uint, error)
	Reinstall(ctx context.Context, server *domain.Server) (uint, error)
}

type ServerControlServiceImpl struct {
	serverRepo       repositories.ServerRepository
	serverController ServerController
}

func NewServerControlService(
	serverRepo repositories.ServerRepository,
	serverController ServerController,
) *ServerControlServiceImpl {
	return &ServerControlServiceImpl{
		serverRepo:       serverRepo,
		serverController: serverController,
	}
}

func (s *ServerControlServiceImpl) getServer(
	ctx context.Context,
	serverID uint64,
) (*domain.Server, error) {
	servers, err := s.serverRepo.Find(
		ctx,
		filters.FindServerByIDs(uint(serverID)),
		nil,
		nil,
	)
	if err != nil {
		return nil, err
	}

	if len(servers) == 0 {
		return nil, nil
	}

	return &servers[0], nil
}

func (s *ServerControlServiceImpl) StartServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Start(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

func (s *ServerControlServiceImpl) StopServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Stop(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

func (s *ServerControlServiceImpl) RestartServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Restart(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

func (s *ServerControlServiceImpl) UpdateServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Update(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

func (s *ServerControlServiceImpl) InstallServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Install(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

func (s *ServerControlServiceImpl) ReinstallServer(
	ctx context.Context,
	req *servercontrol.ServerControlRequest,
) (*servercontrol.ServerControlResponse, error) {
	server, err := s.getServer(ctx, req.ServerId)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	if server == nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr("server not found"),
		}, nil
	}

	taskID, err := s.serverController.Reinstall(ctx, server)
	if err != nil {
		return &servercontrol.ServerControlResponse{
			Success: false,
			Error:   lo.ToPtr(err.Error()),
		}, nil
	}

	return &servercontrol.ServerControlResponse{
		Success: true,
		TaskId:  lo.ToPtr(uint64(taskID)),
	}, nil
}

type ServerControlHostLibrary struct {
	impl *ServerControlServiceImpl
}

func NewServerControlHostLibrary(
	serverRepo repositories.ServerRepository,
	serverController ServerController,
) *ServerControlHostLibrary {
	return &ServerControlHostLibrary{
		impl: NewServerControlService(serverRepo, serverController),
	}
}

func (l *ServerControlHostLibrary) Instantiate(ctx context.Context, r wazero.Runtime) error {
	return servercontrol.Instantiate(ctx, r, l.impl)
}
