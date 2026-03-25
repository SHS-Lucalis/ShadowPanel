package application

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/soheilhy/cmux"
	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc"
)

type MultiplexedServer struct {
	listener   net.Listener
	mux        cmux.CMux
	grpcServer *grpc.Server
	httpServer *http.Server
	logger     *slog.Logger
}

type MultiplexerConfig struct {
	Address    string
	TLSConfig  *tls.Config
	GRPCServer *grpc.Server
	HTTPServer *http.Server
	Logger     *slog.Logger
}

func NewMultiplexedServer(ctx context.Context, config *MultiplexerConfig) (*MultiplexedServer, error) {
	logger := config.Logger
	if logger == nil {
		logger = slog.Default()
	}

	var listener net.Listener
	var err error

	if config.TLSConfig != nil {
		listener, err = tls.Listen("tcp", config.Address, config.TLSConfig)
	} else {
		listener, err = new(net.ListenConfig).Listen(ctx, "tcp", config.Address)
	}

	if err != nil {
		return nil, err
	}

	mux := cmux.New(listener)

	return &MultiplexedServer{
		listener:   listener,
		mux:        mux,
		grpcServer: config.GRPCServer,
		httpServer: config.HTTPServer,
		logger:     logger,
	}, nil
}

func (s *MultiplexedServer) Serve(ctx context.Context) error {
	grpcL := s.mux.MatchWithWriters(
		cmux.HTTP2MatchHeaderFieldSendSettings("content-type", "application/grpc"),
	)

	httpL := s.mux.Match(cmux.Any())

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		s.logger.Info("starting gRPC server")
		if err := s.grpcServer.Serve(grpcL); err != nil {
			if errors.Is(err, grpc.ErrServerStopped) {
				return nil
			}
			s.logger.Error("gRPC server error", "error", err)

			return err
		}

		return nil
	})

	g.Go(func() error {
		s.logger.Info("starting HTTP server")
		if err := s.httpServer.Serve(httpL); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			s.logger.Error("HTTP server error", "error", err)

			return err
		}

		return nil
	})

	g.Go(func() error {
		s.logger.Info("starting multiplexer", "address", s.listener.Addr().String())
		if err := s.mux.Serve(); err != nil {
			if errors.Is(err, cmux.ErrListenerClosed) {
				return nil
			}
			s.logger.Error("cmux server error", "error", err)

			return err
		}

		return nil
	})

	g.Go(func() error {
		<-gctx.Done()

		return s.shutdown(context.Background())
	})

	return g.Wait()
}

func (s *MultiplexedServer) shutdown(ctx context.Context) error {
	s.logger.Info("shutting down multiplexed server")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.grpcServer.GracefulStop()
		close(done)
	}()

	select {
	case <-done:
		s.logger.Debug("gRPC server stopped gracefully")
	case <-shutdownCtx.Done():
		s.logger.Warn("gRPC server force stop due to timeout")
		s.grpcServer.Stop()
	}

	if err := s.httpServer.Shutdown(shutdownCtx); err != nil {
		s.logger.Error("HTTP server shutdown error", "error", err)
	} else {
		s.logger.Debug("HTTP server stopped gracefully")
	}

	if err := s.listener.Close(); err != nil {
		s.logger.Debug("listener close", "error", err)
	}

	return nil
}

func (s *MultiplexedServer) Address() net.Addr {
	return s.listener.Addr()
}

func (s *MultiplexedServer) Close() error {
	return s.listener.Close()
}
