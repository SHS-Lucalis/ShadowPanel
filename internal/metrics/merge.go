package metrics

import (
	"sort"
	"strings"

	"github.com/gameap/gameap/pkg/proto"
)

// mergeResponses consolidates multiple per-tick MetricsResponse entries into
// a single response by grouping series with identical (name + labels) and
// concatenating their points sorted by timestamp.
//
// Returns nil if entries is empty. Returns the input verbatim if it has a
// single element. Series order in the output follows first-occurrence in the
// input. Points with duplicate timestamps within the same series are
// deduplicated defensively — the ring may briefly hold both per-tick entries
// and the multi-point GetHistory fallback response (see hub.gatherReplay).
func mergeResponses(entries []*proto.MetricsResponse) *proto.MetricsResponse {
	if len(entries) == 0 {
		return nil
	}
	if len(entries) == 1 {
		return entries[0]
	}

	type bucket struct {
		series *proto.MetricSeries
		seen   map[int64]struct{}
	}

	buckets := make(map[string]*bucket, len(entries[0].GetSeries()))
	order := make([]string, 0, len(entries[0].GetSeries()))

	for _, e := range entries {
		for _, s := range e.GetSeries() {
			key := seriesIdentityKey(s)

			b, ok := buckets[key]
			if !ok {
				cloned := &proto.MetricSeries{
					Name:   s.GetName(),
					Type:   s.GetType(),
					Unit:   s.GetUnit(),
					Labels: s.GetLabels(),
					Points: make([]*proto.MetricPoint, 0, len(s.GetPoints())),
				}
				b = &bucket{series: cloned, seen: make(map[int64]struct{})}
				buckets[key] = b
				order = append(order, key)
			}

			for _, p := range s.GetPoints() {
				ts := p.GetTimestamp().AsTime().UnixNano()
				if _, dup := b.seen[ts]; dup {
					continue
				}
				b.seen[ts] = struct{}{}
				b.series.Points = append(b.series.Points, p)
			}
		}
	}

	merged := make([]*proto.MetricSeries, 0, len(order))
	for _, key := range order {
		s := buckets[key].series
		sort.SliceStable(s.Points, func(i, j int) bool {
			return s.Points[i].GetTimestamp().AsTime().Before(s.Points[j].GetTimestamp().AsTime())
		})
		merged = append(merged, s)
	}

	newest := entries[len(entries)-1]
	var window uint32
	for _, e := range entries {
		if w := e.GetActualWindowSeconds(); w > window {
			window = w
		}
	}

	return &proto.MetricsResponse{
		Timestamp:           newest.GetTimestamp(),
		CommonLabels:        newest.GetCommonLabels(),
		Series:              merged,
		ActualWindowSeconds: window,
	}
}

// seriesIdentityKey produces a stable key for grouping series across entries:
// the series name joined with its labels in sorted-key order.
func seriesIdentityKey(s *proto.MetricSeries) string {
	labels := s.GetLabels()
	if len(labels) == 0 {
		return s.GetName()
	}

	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.Grow(len(s.GetName()) + len(keys)*16)
	b.WriteString(s.GetName())
	for _, k := range keys {
		b.WriteByte('|')
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(labels[k])
	}

	return b.String()
}
