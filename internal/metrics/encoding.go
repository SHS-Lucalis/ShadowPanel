package metrics

import (
	"strconv"
	"time"

	"github.com/gameap/gameap/pkg/proto"
)

// WireResponse is the JSON-friendly representation of a MetricsResponse
// emitted over the WebSocket to the browser. Series-grouped to match
// the wire shape used everywhere else.
type WireResponse struct {
	Timestamp           time.Time         `json:"timestamp"`
	CommonLabels        map[string]string `json:"common_labels,omitempty"`
	Series              []WireSeries      `json:"series"`
	ActualWindowSeconds uint32            `json:"actual_window_seconds,omitempty"`
}

type WireSeries struct {
	Name   string            `json:"name"`
	Type   string            `json:"type"`
	Unit   string            `json:"unit"`
	Labels map[string]string `json:"labels,omitempty"`
	Points []WirePoint       `json:"points"`
}

type WirePoint struct {
	Timestamp   time.Time `json:"timestamp"`
	Value       string    `json:"value"`
	Min         *float64  `json:"min,omitempty"`
	Max         *float64  `json:"max,omitempty"`
	Avg         *float64  `json:"avg,omitempty"`
	SampleCount *uint32   `json:"sample_count,omitempty"`
}

// SeriesFilter decides whether a series should be included on the
// outbound wire. Returning false drops the series entirely.
type SeriesFilter func(*proto.MetricSeries) bool

// ToWire converts a MetricsResponse to its JSON-ready form, optionally
// filtering series. Returns nil if every series is dropped.
func ToWire(resp *proto.MetricsResponse, filter SeriesFilter) *WireResponse {
	if resp == nil {
		return nil
	}

	series := make([]WireSeries, 0, len(resp.Series))
	for _, s := range resp.Series {
		if filter != nil && !filter(s) {
			continue
		}
		series = append(series, encodeSeries(s))
	}

	if len(series) == 0 {
		return nil
	}

	out := &WireResponse{
		Timestamp:           resp.GetTimestamp().AsTime(),
		CommonLabels:        resp.GetCommonLabels(),
		Series:              series,
		ActualWindowSeconds: resp.GetActualWindowSeconds(),
	}

	return out
}

func encodeSeries(s *proto.MetricSeries) WireSeries {
	points := make([]WirePoint, 0, len(s.Points))
	for _, p := range s.Points {
		points = append(points, encodePoint(p))
	}

	return WireSeries{
		Name:   s.GetName(),
		Type:   metricTypeName(s.GetType()),
		Unit:   metricUnitName(s.GetUnit()),
		Labels: s.GetLabels(),
		Points: points,
	}
}

func encodePoint(p *proto.MetricPoint) WirePoint {
	wp := WirePoint{
		Timestamp: p.GetTimestamp().AsTime(),
		Value:     pointValue(p),
	}

	if p.Min != nil {
		v := p.GetMin()
		wp.Min = &v
	}
	if p.Max != nil {
		v := p.GetMax()
		wp.Max = &v
	}
	if p.Avg != nil {
		v := p.GetAvg()
		wp.Avg = &v
	}
	if p.SampleCount != nil {
		v := p.GetSampleCount()
		wp.SampleCount = &v
	}

	return wp
}

func pointValue(p *proto.MetricPoint) string {
	switch v := p.GetValue().(type) {
	case *proto.MetricPoint_DoubleValue:
		return strconv.FormatFloat(v.DoubleValue, 'g', -1, 64)
	case *proto.MetricPoint_UintValue:
		return strconv.FormatUint(v.UintValue, 10)
	case *proto.MetricPoint_IntValue:
		return strconv.FormatInt(v.IntValue, 10)
	}

	return ""
}

func metricTypeName(t proto.MetricType) string {
	switch t {
	case proto.MetricType_METRIC_TYPE_GAUGE:
		return "gauge"
	case proto.MetricType_METRIC_TYPE_COUNTER:
		return "counter"
	}

	return "unspecified"
}

func metricUnitName(u proto.MetricUnit) string {
	switch u {
	case proto.MetricUnit_METRIC_UNIT_COUNT:
		return "count"
	case proto.MetricUnit_METRIC_UNIT_PERCENT:
		return "percent"
	case proto.MetricUnit_METRIC_UNIT_RATIO:
		return "ratio"
	case proto.MetricUnit_METRIC_UNIT_BYTES:
		return "bytes"
	case proto.MetricUnit_METRIC_UNIT_BITS:
		return "bits"
	case proto.MetricUnit_METRIC_UNIT_SECONDS:
		return "seconds"
	case proto.MetricUnit_METRIC_UNIT_MILLISECONDS:
		return "milliseconds"
	case proto.MetricUnit_METRIC_UNIT_MICROSECONDS:
		return "microseconds"
	case proto.MetricUnit_METRIC_UNIT_NANOSECONDS:
		return "nanoseconds"
	case proto.MetricUnit_METRIC_UNIT_HERTZ:
		return "hertz"
	case proto.MetricUnit_METRIC_UNIT_CELSIUS:
		return "celsius"
	case proto.MetricUnit_METRIC_UNIT_WATTS:
		return "watts"
	case proto.MetricUnit_METRIC_UNIT_VOLTS:
		return "volts"
	case proto.MetricUnit_METRIC_UNIT_RPM:
		return "rpm"
	}

	return "unspecified"
}
