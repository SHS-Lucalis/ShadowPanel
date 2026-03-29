package daemon

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const defaultConsoleLogMaxBytes int64 = 65536

type ConsoleLogService struct {
	gateway    ConsoleLogGateway
	registry   ConnectionChecker
	dispatcher ConsoleLogDispatcher
	logger     *slog.Logger
}

func NewConsoleLogService(
	gateway ConsoleLogGateway,
	registry ConnectionChecker,
	dispatcher ConsoleLogDispatcher,
	logger *slog.Logger,
) *ConsoleLogService {
	if logger == nil {
		logger = slog.Default()
	}

	return &ConsoleLogService{
		gateway:    gateway,
		registry:   registry,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

func (s *ConsoleLogService) GetConsoleLog(
	ctx context.Context,
	nodeID uint64,
	serverID uint64,
	maxBytes int64,
) (string, error) {
	if maxBytes <= 0 {
		maxBytes = defaultConsoleLogMaxBytes
	}

	if s.registry.IsConnected(nodeID) {
		return s.requestViaGateway(ctx, nodeID, serverID, maxBytes)
	}

	if s.registry.IsConnectedAnywhere(nodeID) {
		return s.requestViaDispatcher(ctx, nodeID, serverID, maxBytes)
	}

	return "", ErrDaemonNotConnected
}

func (s *ConsoleLogService) requestViaGateway(
	ctx context.Context,
	nodeID uint64,
	serverID uint64,
	maxBytes int64,
) (string, error) {
	resp, err := s.gateway.RequestConsoleLog(ctx, nodeID, serverID, maxBytes)
	if err != nil {
		return "", errors.WithMessage(err, "gateway console log request")
	}

	return consoleLogResponseToString(resp)
}

func (s *ConsoleLogService) requestViaDispatcher(
	ctx context.Context,
	nodeID uint64,
	serverID uint64,
	maxBytes int64,
) (string, error) {
	resp, err := s.dispatcher.DispatchConsoleLog(ctx, nodeID, serverID, maxBytes)
	if err != nil {
		return "", errors.WithMessage(err, "dispatched console log request")
	}

	return consoleLogResponseToString(resp)
}

func consoleLogResponseToString(resp *proto.ConsoleLogResponse) (string, error) {
	if !resp.Success {
		return "", errors.New(resp.Error)
	}

	return string(resp.Data), nil
}
