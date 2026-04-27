package nodesmetrics

import (
	"context"
	"log/slog"
	"maps"
	"strconv"
	"sync"

	"github.com/gameap/gameap/internal/api/ws/metricsbase"
	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/internal/ws"
	"github.com/gameap/gameap/pkg/proto"
)

const commonLabelNodeID = "node_id"

// clientSink is the subset of ws.Client that pumpAll needs. The narrow
// interface lets tests substitute an in-memory recorder.
type clientSink interface {
	SendMessage(*ws.OutboundMessage)
	Done() <-chan struct{}
}

// pumpAll subscribes to the metrics hub for every node in nodeIDs and
// forwards live samples to the client, tagging each envelope with
// node_id in CommonLabels so the browser can demultiplex.
//
// No replay is sent — the multi-node stream is for current-value cards
// only; modal sparkline history is served by the per-node endpoint.
//
// pumpAll returns once subscriptions are open. A single fan-in goroutine
// runs in the background until the client disconnects or ctx is
// cancelled, then closes every subscription.
func pumpAll(
	ctx context.Context,
	hub metrics.Hub,
	nodeIDs []uint64,
	client clientSink,
	logger *slog.Logger,
) {
	subCtx, cancel := context.WithCancel(ctx)

	subs := openSubscriptions(subCtx, hub, nodeIDs, logger)

	client.SendMessage(ws.NewOutboundMessage(metricsbase.TypeReplayDone, struct{}{}))

	if len(subs) == 0 {
		cancel()

		return
	}

	go runFanIn(subCtx, cancel, subs, client)
}

type nodeSub struct {
	nodeID uint64
	sub    metrics.Subscription
}

func openSubscriptions(
	ctx context.Context,
	hub metrics.Hub,
	nodeIDs []uint64,
	logger *slog.Logger,
) []nodeSub {
	subs := make([]nodeSub, 0, len(nodeIDs))
	for _, nodeID := range nodeIDs {
		sub, _, err := hub.Subscribe(ctx, nodeID, 0)
		if err != nil {
			logger.Warn("failed to subscribe to metrics hub",
				"node_id", nodeID,
				"error", err,
			)

			continue
		}
		subs = append(subs, nodeSub{nodeID: nodeID, sub: sub})
	}

	return subs
}

func runFanIn(
	ctx context.Context,
	cancel context.CancelFunc,
	subs []nodeSub,
	client clientSink,
) {
	defer cancel()
	defer func() {
		for _, ns := range subs {
			ns.sub.Close()
		}
	}()

	samples := make(chan nodeSample, len(subs))
	var wg sync.WaitGroup

	for _, ns := range subs {
		wg.Add(1)
		go forwardSamples(ctx, ns, client, samples, &wg)
	}

	go func() {
		wg.Wait()
		close(samples)
	}()

	dispatchSamples(ctx, client, samples)
}

func forwardSamples(
	ctx context.Context,
	ns nodeSub,
	client clientSink,
	out chan<- nodeSample,
	wg *sync.WaitGroup,
) {
	defer wg.Done()
	for {
		select {
		case <-client.Done():
			return
		case <-ctx.Done():
			return
		case sample, ok := <-ns.sub.Samples():
			if !ok {
				return
			}
			select {
			case out <- nodeSample{nodeID: ns.nodeID, response: sample}:
			case <-client.Done():
				return
			case <-ctx.Done():
				return
			}
		}
	}
}

func dispatchSamples(ctx context.Context, client clientSink, samples <-chan nodeSample) {
	for {
		select {
		case <-client.Done():
			return
		case <-ctx.Done():
			return
		case s, ok := <-samples:
			if !ok {
				return
			}
			wire := metrics.ToWire(tagNodeID(s.response, s.nodeID), metricsbase.NodePrefixFilter())
			if wire == nil {
				continue
			}
			client.SendMessage(ws.NewOutboundMessage(metricsbase.TypeMetrics, wire))
		}
	}
}

type nodeSample struct {
	nodeID   uint64
	response *proto.MetricsResponse
}

// tagNodeID injects node_id into the response's common labels so the
// browser can route the envelope to the right card. The proto map is
// shared with other subscribers, so we copy before mutating.
func tagNodeID(resp *proto.MetricsResponse, nodeID uint64) *proto.MetricsResponse {
	if resp == nil {
		return nil
	}

	idStr := strconv.FormatUint(nodeID, 10)

	cloned := &proto.MetricsResponse{
		Timestamp:           resp.GetTimestamp(),
		Series:              resp.GetSeries(),
		ActualWindowSeconds: resp.GetActualWindowSeconds(),
	}

	srcLabels := resp.GetCommonLabels()
	cloned.CommonLabels = make(map[string]string, len(srcLabels)+1)
	maps.Copy(cloned.CommonLabels, srcLabels)
	cloned.CommonLabels[commonLabelNodeID] = idStr

	return cloned
}
