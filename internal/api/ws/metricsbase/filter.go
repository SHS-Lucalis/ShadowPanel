package metricsbase

import (
	"strings"

	"github.com/gameap/gameap/internal/metrics"
	"github.com/gameap/gameap/pkg/proto"
)

const nodeMetricNamePrefix = "gameap_node_"

// NodePrefixFilter passes only series whose name starts with gameap_node_,
// dropping per-server (gameap_server_*) and any other prefixes. Used by
// node-scoped WS endpoints to keep the wire payload focused on host metrics.
func NodePrefixFilter() metrics.SeriesFilter {
	return func(s *proto.MetricSeries) bool {
		return strings.HasPrefix(s.GetName(), nodeMetricNamePrefix)
	}
}
