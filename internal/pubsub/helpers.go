package pubsub

import (
	"context"
	"log/slog"
	"strings"
)

// MatchPattern checks if a channel matches a pattern.
// Only trailing wildcard (*) is supported for cross-driver compatibility.
func MatchPattern(pattern, channel string) bool {
	if !strings.Contains(pattern, "*") {
		return pattern == channel
	}

	if prefix, found := strings.CutSuffix(pattern, "*"); found {
		return strings.HasPrefix(channel, prefix)
	}

	return false
}

// SafeCall executes a handler with panic recovery.
func SafeCall(ctx context.Context, handler Handler, msg *Message, logger *slog.Logger) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error("handler panic recovered",
				slog.Any("panic", r),
				slog.String("channel", msg.Channel),
			)
		}
	}()

	if err := handler(ctx, msg); err != nil {
		logger.Error("handler error",
			slog.String("channel", msg.Channel),
			slog.String("error", err.Error()),
		)
	}
}
