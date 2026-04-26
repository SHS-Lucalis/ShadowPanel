package metricsbase

import (
	"context"
	"log/slog"
	"time"

	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/proto"
)

const (
	TypeMetrics    = "metrics"
	TypeReplay     = "metrics.replay"
	TypeReplayDone = "metrics.replay.done"
	TypeError      = "metrics.error"
)

// Pump subscribes to the metrics hub for nodeID, sends initial replay
// and then forwards live samples to the client. Returns once the
// initial replay has been delivered; live forwarding continues in a
// background goroutine until the client closes or ctx is cancelled.
func Pump(
	ctx context.Context,
	hub metrics.Hub,
	nodeID uint64,
	filter metrics.SeriesFilter,
	client *ws.Client,
	replayWindow time.Duration,
	logger *slog.Logger,
) {
	subCtx, cancel := context.WithCancel(ctx)

	sub, replay, err := hub.Subscribe(subCtx, nodeID, replayWindow)
	if err != nil {
		cancel()
		logger.Warn("failed to subscribe to metrics hub",
			"node_id", nodeID,
			"error", err,
		)
		client.SendMessage(ws.NewOutboundMessage(TypeError, map[string]string{"error": err.Error()}))

		return
	}

	if len(replay) > 0 {
		client.SendMessage(ws.NewOutboundMessage(TypeReplay, encodeReplay(replay, filter)))
	}
	client.SendMessage(ws.NewOutboundMessage(TypeReplayDone, struct{}{}))

	go func() {
		defer cancel()
		defer sub.Close()

		for {
			select {
			case <-client.Done():
				return
			case <-subCtx.Done():
				return
			case sample, ok := <-sub.Samples():
				if !ok {
					return
				}
				wire := metrics.ToWire(sample, filter)
				if wire == nil {
					continue
				}
				client.SendMessage(ws.NewOutboundMessage(TypeMetrics, wire))
			}
		}
	}()
}

func encodeReplay(entries []*proto.MetricsResponse, filter metrics.SeriesFilter) []*metrics.WireResponse {
	out := make([]*metrics.WireResponse, 0, len(entries))
	for _, e := range entries {
		w := metrics.ToWire(e, filter)
		if w == nil {
			continue
		}
		out = append(out, w)
	}

	return out
}
