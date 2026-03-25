package application

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"

	"github.com/gameap/gameap/internal/application/defaults"
	"github.com/gameap/gameap/internal/config"
	"github.com/gameap/gameap/internal/pubsub/integration"
	"github.com/gameap/gameap/migrations"
	"github.com/pkg/errors"
)

type RunParams struct {
	EnvFile       string
	LegacyEnvFile string
}

//nolint:funlen
func Run(runParams RunParams) {
	if err := loadEnvFile(runParams.EnvFile); err != nil {
		slog.Error("Failed to load env file", slog.String("error", err.Error()))

		os.Exit(1)

		return
	}

	if err := loadLegacyEnv(runParams.LegacyEnvFile); err != nil {
		// Log the error but continue execution
		slog.Error("Failed to load legacy env file", slog.String("error", err.Error()))
	}

	cfg, err := config.LoadConfig()
	if err != nil {
		slog.Error("Failed to load config", slog.String("error", err.Error()))

		os.Exit(1)

		return
	}

	logLevel := slog.LevelInfo

	switch cfg.Logger.Level {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "error":
		logLevel = slog.LevelError
	}

	slog.SetLogLoggerLevel(logLevel)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)

	ctx, cancel := context.WithCancel(context.Background())

	container := NewContainer(cfg)
	container.SetContext(ctx)

	go func() {
		oscall := <-c

		slog.Info("Got signal: " + oscall.String())

		cancel()

		err = container.Shutdown()
		if err != nil {
			slog.ErrorContext(
				ctx,
				"Failed to shutdown container",
				slog.String("error", err.Error()),
			)
		}
	}()

	err = migrations.Run(ctx, container)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to run migrations",
			slog.String("error", err.Error()),
		)

		os.Exit(1)

		return
	}

	err = seed(ctx, container)
	if err != nil {
		slog.ErrorContext(
			ctx,
			"Failed to seed database",
			slog.String("error", err.Error()),
		)

		os.Exit(1)

		return
	}

	slog.InfoContext(
		ctx,
		"GameAP started",
		slog.String("version", defaults.Version),
		slog.String("build_date", defaults.BuildDate),
	)

	err = container.PluginLoader().LoadAll(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load plugins", slog.String("error", err.Error()))

		os.Exit(1)
	}

	err = container.PluginDispatcher().RefreshSubscriptions(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to refresh plugin subscriptions", slog.String("error", err.Error()))

		os.Exit(1)
	}

	startPubSub(ctx, container)

	if cfg.GRPC.Enabled {
		runWithGRPC(ctx, cfg, container)
	} else {
		runHTTPOnly(ctx, cfg, container)
	}
}

func runHTTPOnly(ctx context.Context, cfg *config.Config, container *Container) {
	slog.InfoContext(ctx, fmt.Sprintf("Starting HTTP server on %s:%d", cfg.HTTPHost, cfg.HTTPPort))

	if cfg.TLSEnabled() {
		startHTTPSServer(ctx, cfg, container)
	}

	server := container.HTTPServer()

	err := server.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func runWithGRPC(ctx context.Context, cfg *config.Config, container *Container) {
	if err := container.SessionRegistry().Start(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to start session registry", slog.String("error", err.Error()))
		os.Exit(1)
	}

	grpcServer := container.GRPCServer()
	grpcAddr := fmt.Sprintf("%s:%d", cfg.HTTPHost, cfg.GRPC.Port)

	lis, err := new(net.ListenConfig).Listen(ctx, "tcp", grpcAddr)
	if err != nil {
		slog.ErrorContext(ctx, "Failed to listen for gRPC", slog.String("error", err.Error()))
		os.Exit(1)
	}

	go func() {
		slog.InfoContext(ctx, "Starting gRPC server", slog.String("address", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			slog.ErrorContext(ctx, "gRPC server error", slog.String("error", err.Error()))
		}
	}()

	go func() {
		<-ctx.Done()
		grpcServer.GracefulStop()
	}()

	if cfg.TLSEnabled() {
		startHTTPSServer(ctx, cfg, container)
	}

	server := container.HTTPServer()
	slog.InfoContext(ctx, fmt.Sprintf("Starting HTTP server on %s:%d", cfg.HTTPHost, cfg.HTTPPort))

	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func startHTTPSServer(ctx context.Context, cfg *config.Config, container *Container) {
	cert, err := cfg.LoadTLSCertificate()
	if err != nil {
		slog.ErrorContext(ctx, "Failed to load TLS certificate", slog.String("error", err.Error()))

		os.Exit(1)

		return
	}

	httpsServer := container.HTTPSServer()
	httpsServer.TLSConfig = &tls.Config{
		Certificates: []tls.Certificate{*cert},
		MinVersion:   tls.VersionTLS12,
	}

	go func() {
		slog.InfoContext(ctx, fmt.Sprintf("Starting HTTPS server on %s:%d", cfg.HTTPHost, cfg.HTTPSPort))

		err := httpsServer.ListenAndServeTLS("", "")
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.ErrorContext(ctx, "HTTPS server error", slog.String("error", err.Error()))
		}
	}()
}

func startPubSub(ctx context.Context, container *Container) {
	ps := container.PubSub()

	cacheInvalidator := integration.NewCacheInvalidator(ps, container.Cache())
	if err := cacheInvalidator.Start(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to start cache invalidator", slog.String("error", err.Error()))
	}

	if err := container.WSBridge().Start(ctx); err != nil {
		slog.ErrorContext(ctx, "Failed to start WebSocket bridge", slog.String("error", err.Error()))
	}

	go func() {
		slog.InfoContext(ctx, "Starting pub-sub listener",
			slog.String("driver", container.Config().PubSub.Driver),
		)

		if err := ps.Start(ctx); err != nil && !errors.Is(err, context.Canceled) {
			slog.ErrorContext(ctx, "Pub-sub listener error", slog.String("error", err.Error()))
		}
	}()
}
