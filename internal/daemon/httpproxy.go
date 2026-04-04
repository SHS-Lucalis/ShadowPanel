package daemon

import (
	"context"
	"log/slog"

	"github.com/gameap/gameap/pkg/proto"
	"github.com/pkg/errors"
)

const capabilityHTTPProxy = "http_proxy"

type HTTPProxyService struct {
	gateway    HTTPProxyGateway
	registry   ConnectionChecker
	dispatcher HTTPProxyDispatcher
	logger     *slog.Logger
}

func NewHTTPProxyService(
	gateway HTTPProxyGateway,
	registry ConnectionChecker,
	dispatcher HTTPProxyDispatcher,
	logger *slog.Logger,
) *HTTPProxyService {
	if logger == nil {
		logger = slog.Default()
	}

	return &HTTPProxyService{
		gateway:    gateway,
		registry:   registry,
		dispatcher: dispatcher,
		logger:     logger,
	}
}

func (s *HTTPProxyService) ProxyHTTP(
	ctx context.Context,
	nodeID uint64,
	req *proto.HTTPProxyRequest,
) (*proto.HTTPProxyResponse, error) {
	if !s.registry.HasCapability(nodeID, capabilityHTTPProxy) {
		return nil, errors.New("node does not support http_proxy capability")
	}

	if s.registry.IsConnected(nodeID) {
		return s.proxyViaGateway(ctx, nodeID, req)
	}

	if s.registry.IsConnectedAnywhere(nodeID) {
		return s.proxyViaDispatcher(ctx, nodeID, req)
	}

	return nil, ErrDaemonNotConnected
}

func (s *HTTPProxyService) proxyViaGateway(
	ctx context.Context,
	nodeID uint64,
	req *proto.HTTPProxyRequest,
) (*proto.HTTPProxyResponse, error) {
	resp, err := s.gateway.RequestHTTPProxy(ctx, nodeID, req)
	if err != nil {
		return nil, errors.WithMessage(err, "gateway http proxy request")
	}

	return resp, nil
}

func (s *HTTPProxyService) proxyViaDispatcher(
	ctx context.Context,
	nodeID uint64,
	req *proto.HTTPProxyRequest,
) (*proto.HTTPProxyResponse, error) {
	resp, err := s.dispatcher.DispatchHTTPProxy(ctx, nodeID, req)
	if err != nil {
		return nil, errors.WithMessage(err, "dispatched http proxy request")
	}

	return resp, nil
}
