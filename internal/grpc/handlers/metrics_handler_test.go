package handlers

import (
	"context"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/gameap/gameap/internal/pubsub/channels"
	"github.com/gameap/gameap/internal/pubsub/memory"
	"github.com/gameap/gameap/internal/pubsub/messages"
	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestMetricsHandler_HandleMetricsBatch(t *testing.T) {
	t.Run("publishes_batch_to_node_channel", func(t *testing.T) {
		ps := memory.New()
		t.Cleanup(func() { _ = ps.Close() })

		const nodeID uint64 = 42
		var (
			received   *pubsub.Message
			receivedMu sync.Mutex
			done       = make(chan struct{})
		)

		err := ps.Subscribe(context.Background(), channels.RealtimeMetricsAll, func(_ context.Context, msg *pubsub.Message) error {
			receivedMu.Lock()
			received = msg
			receivedMu.Unlock()
			close(done)

			return nil
		})
		require.NoError(t, err)

		handler := NewMetricsHandler(ps, slog.Default())

		batch := &proto.MetricsBatch{
			Timestamp:    timestamppb.Now(),
			CommonLabels: map[string]string{"env": "prod"},
			Series: []*proto.MetricSeries{
				{
					Name: "cpu_usage_percent",
					Type: proto.MetricType_METRIC_TYPE_GAUGE,
					Unit: proto.MetricUnit_METRIC_UNIT_PERCENT,
					Points: []*proto.MetricPoint{
						{
							Timestamp: timestamppb.Now(),
							Value:     &proto.MetricPoint_DoubleValue{DoubleValue: 42.5},
						},
					},
				},
			},
		}

		err = handler.HandleMetricsBatch(context.Background(), nodeID, batch)
		require.NoError(t, err)

		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for pubsub delivery")
		}

		receivedMu.Lock()
		defer receivedMu.Unlock()

		require.NotNil(t, received)
		assert.Equal(t, channels.BuildRealtimeMetricsChannel(nodeID), received.Channel)
		assert.Equal(t, messages.TypeMetricsBatch, received.Type)

		payload, err := messages.ParsePayload[messages.MetricsBatchPayload](received)
		require.NoError(t, err)
		assert.Equal(t, nodeID, payload.NodeID)
		require.NotEmpty(t, payload.Data)

		var decoded proto.MetricsBatch
		require.NoError(t, decoded.UnmarshalVT(payload.Data))
		require.Len(t, decoded.Series, 1)
		assert.Equal(t, "cpu_usage_percent", decoded.Series[0].Name)
	})

	t.Run("nil_publisher_is_noop", func(t *testing.T) {
		handler := NewMetricsHandler(nil, slog.Default())

		err := handler.HandleMetricsBatch(context.Background(), 1, &proto.MetricsBatch{})
		assert.NoError(t, err)
	})
}
