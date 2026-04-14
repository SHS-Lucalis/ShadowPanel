package grpc

import (
	"context"
	"log/slog"
	"net"
	"time"

	"google.golang.org/grpc/stats"
)

type connContextKey struct{}

type connContext struct {
	remoteAddr string
	localAddr  string
	startedAt  time.Time
}

type ConnectionLogger struct {
	logger *slog.Logger
}

func NewConnectionLogger(logger *slog.Logger) *ConnectionLogger {
	if logger == nil {
		logger = slog.Default()
	}

	return &ConnectionLogger{logger: logger}
}

func (h *ConnectionLogger) TagConn(ctx context.Context, info *stats.ConnTagInfo) context.Context {
	cc := &connContext{
		remoteAddr: addrString(info.RemoteAddr),
		localAddr:  addrString(info.LocalAddr),
		startedAt:  time.Now(),
	}

	return context.WithValue(ctx, connContextKey{}, cc)
}

func (h *ConnectionLogger) HandleConn(ctx context.Context, s stats.ConnStats) {
	cc, ok := ctx.Value(connContextKey{}).(*connContext)
	if !ok {
		return
	}

	switch s.(type) {
	case *stats.ConnBegin:
		h.logger.Info("gRPC connection opened",
			"peer", cc.remoteAddr,
			"local", cc.localAddr,
		)
	case *stats.ConnEnd:
		h.logger.Info("gRPC connection closed",
			"peer", cc.remoteAddr,
			"local", cc.localAddr,
			"duration_ms", time.Since(cc.startedAt).Milliseconds(),
		)
	}
}

func (h *ConnectionLogger) TagRPC(ctx context.Context, _ *stats.RPCTagInfo) context.Context {
	return ctx
}

func (h *ConnectionLogger) HandleRPC(_ context.Context, _ stats.RPCStats) {
}

func addrString(addr net.Addr) string {
	if addr == nil {
		return "unknown"
	}

	return addr.String()
}
