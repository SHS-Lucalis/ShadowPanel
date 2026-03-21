package interceptors

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

type LoggingInterceptor struct {
	logger *slog.Logger
}

func NewLoggingInterceptor(logger *slog.Logger) *LoggingInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return &LoggingInterceptor{
		logger: logger,
	}
}

func (i *LoggingInterceptor) StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		start := time.Now()
		ctx := ss.Context()

		peerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		i.logger.Debug("gRPC stream started",
			"method", info.FullMethod,
			"peer", peerAddr,
		)

		err := handler(srv, ss)

		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logLevel := slog.LevelDebug
		if code != codes.OK && code != codes.Canceled {
			logLevel = slog.LevelWarn
		}

		i.logger.Log(ctx, logLevel, "gRPC stream ended",
			"method", info.FullMethod,
			"peer", peerAddr,
			"duration_ms", duration.Milliseconds(),
			"code", code.String(),
		)

		return err
	}
}

func (i *LoggingInterceptor) UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		start := time.Now()

		peerAddr := "unknown"
		if p, ok := peer.FromContext(ctx); ok {
			peerAddr = p.Addr.String()
		}

		i.logger.Debug("gRPC request started",
			"method", info.FullMethod,
			"peer", peerAddr,
		)

		resp, err := handler(ctx, req)

		duration := time.Since(start)
		code := codes.OK
		if err != nil {
			code = status.Code(err)
		}

		logLevel := slog.LevelDebug
		if code != codes.OK {
			logLevel = slog.LevelWarn
		}

		i.logger.Log(ctx, logLevel, "gRPC request completed",
			"method", info.FullMethod,
			"peer", peerAddr,
			"duration_ms", duration.Milliseconds(),
			"code", code.String(),
		)

		return resp, err
	}
}
