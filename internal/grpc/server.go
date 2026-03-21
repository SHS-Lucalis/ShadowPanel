package grpc

import (
	"log/slog"

	"github.com/gameap/gameap/internal/grpc/filetransfer"
	"github.com/gameap/gameap/internal/grpc/gateway"
	"github.com/gameap/gameap/internal/grpc/interceptors"
	"github.com/gameap/gameap/internal/repositories"
	"github.com/gameap/gameap/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
)

type ServerConfig struct {
	MaxRecvMsgSize       int
	MaxSendMsgSize       int
	MaxConcurrentStreams uint32
	RequireMTLS          bool
	FileTransferBasePath string
}

func DefaultServerConfig() *ServerConfig {
	return &ServerConfig{
		MaxRecvMsgSize:       10 << 20,
		MaxSendMsgSize:       10 << 20,
		MaxConcurrentStreams: 100,
		RequireMTLS:          false,
	}
}

type ServerDependencies struct {
	GatewayService      *gateway.Service
	FileTransferService *filetransfer.Service
	NodeRepo            repositories.NodeRepository
	Logger              *slog.Logger
}

func NewServer(config *ServerConfig, deps *ServerDependencies) *grpc.Server {
	if config == nil {
		config = DefaultServerConfig()
	}

	logger := deps.Logger
	if logger == nil {
		logger = slog.Default()
	}

	recoveryInterceptor := interceptors.NewRecoveryInterceptor(logger)
	loggingInterceptor := interceptors.NewLoggingInterceptor(logger)
	authInterceptor := interceptors.NewAuthInterceptor(deps.NodeRepo, config.RequireMTLS, logger)

	opts := []grpc.ServerOption{
		grpc.MaxRecvMsgSize(config.MaxRecvMsgSize),
		grpc.MaxSendMsgSize(config.MaxSendMsgSize),
		grpc.MaxConcurrentStreams(config.MaxConcurrentStreams),
		grpc.ChainStreamInterceptor(
			recoveryInterceptor.StreamServerInterceptor(),
			loggingInterceptor.StreamServerInterceptor(),
			authInterceptor.StreamServerInterceptor(),
		),
		grpc.ChainUnaryInterceptor(
			recoveryInterceptor.UnaryServerInterceptor(),
			loggingInterceptor.UnaryServerInterceptor(),
			authInterceptor.UnaryServerInterceptor(),
		),
	}

	server := grpc.NewServer(opts...)

	if deps.GatewayService != nil {
		proto.RegisterDaemonGatewayServer(server, deps.GatewayService)
	}

	if deps.FileTransferService != nil {
		proto.RegisterFileTransferServiceServer(server, deps.FileTransferService)
	}

	healthServer := health.NewServer()
	healthpb.RegisterHealthServer(server, healthServer)
	healthServer.SetServingStatus("gameap.DaemonGateway", healthpb.HealthCheckResponse_SERVING)
	healthServer.SetServingStatus("gameap.FileTransferService", healthpb.HealthCheckResponse_SERVING)

	return server
}
