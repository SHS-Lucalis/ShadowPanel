package servermetrics

import (
	"testing"

	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
)

func TestServerIDFilter(t *testing.T) {
	const wantedServer uint = 7

	tests := []struct {
		name   string
		series *proto.MetricSeries
		want   bool
	}{
		{
			name: "matching_server_id__pass",
			series: &proto.MetricSeries{
				Name:   "gameap_server_cpu",
				Labels: map[string]string{"server_id": "7"},
			},
			want: true,
		},
		{
			name: "different_server_id__drop",
			series: &proto.MetricSeries{
				Name:   "gameap_server_cpu",
				Labels: map[string]string{"server_id": "8"},
			},
			want: false,
		},
		{
			name: "no_server_id_label__drop",
			series: &proto.MetricSeries{
				Name:   "gameap_node_cpu",
				Labels: map[string]string{"host": "node-a"},
			},
			want: false,
		},
		{
			name: "empty_server_id__drop",
			series: &proto.MetricSeries{
				Name:   "gameap_server_cpu",
				Labels: map[string]string{"server_id": ""},
			},
			want: false,
		},
		{
			name: "nil_labels__drop",
			series: &proto.MetricSeries{
				Name: "gameap_node_loadavg",
			},
			want: false,
		},
	}

	filter := serverIDFilter(wantedServer)
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, filter(tc.series))
		})
	}
}
