package grpc

import (
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"google.golang.org/grpc/grpclog"
)

var installGRPCLogOnce sync.Once

func InstallGRPCLog(logger *slog.Logger) {
	installGRPCLogOnce.Do(func() {
		if logger == nil {
			logger = slog.Default()
		}

		grpclog.SetLoggerV2(&grpclogAdapter{
			logger: logger.With("component", "grpc-internal"),
		})
	})
}

type grpclogAdapter struct {
	logger *slog.Logger
	// verbosity is the maximum V-level this adapter enables. grpc-go passes
	// level codes 0..2 to V(); keeping it at 0 suppresses the noisiest paths
	// while still surfacing handshake/transport errors.
	verbosity int
}

func (a *grpclogAdapter) Info(args ...any) {
	a.logger.Info(fmt.Sprint(args...))
}

func (a *grpclogAdapter) Infoln(args ...any) {
	a.logger.Info(trimNewline(fmt.Sprintln(args...)))
}

func (a *grpclogAdapter) Infof(format string, args ...any) {
	a.logger.Info(fmt.Sprintf(format, args...))
}

func (a *grpclogAdapter) Warning(args ...any) {
	a.logger.Warn(fmt.Sprint(args...))
}

func (a *grpclogAdapter) Warningln(args ...any) {
	a.logger.Warn(trimNewline(fmt.Sprintln(args...)))
}

func (a *grpclogAdapter) Warningf(format string, args ...any) {
	a.logger.Warn(fmt.Sprintf(format, args...))
}

func (a *grpclogAdapter) Error(args ...any) {
	a.logger.Error(fmt.Sprint(args...))
}

func (a *grpclogAdapter) Errorln(args ...any) {
	a.logger.Error(trimNewline(fmt.Sprintln(args...)))
}

func (a *grpclogAdapter) Errorf(format string, args ...any) {
	a.logger.Error(fmt.Sprintf(format, args...))
}

func (a *grpclogAdapter) Fatal(args ...any) {
	a.logger.Error(fmt.Sprint(args...))
	os.Exit(1)
}

func (a *grpclogAdapter) Fatalln(args ...any) {
	a.logger.Error(trimNewline(fmt.Sprintln(args...)))
	os.Exit(1)
}

func (a *grpclogAdapter) Fatalf(format string, args ...any) {
	a.logger.Error(fmt.Sprintf(format, args...))
	os.Exit(1)
}

func (a *grpclogAdapter) V(level int) bool {
	return level <= a.verbosity
}

func trimNewline(s string) string {
	return strings.TrimRight(s, "\n")
}
