package metrics

import (
	"testing"
	"time"

	"github.com/gameap/gameap/pkg/proto"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestSubscription_Samples_ReturnsBufferedChannel(t *testing.T) {
	// ARRANGE
	sub := newSubscription(nil, 7, 4)

	// ACT
	ch := sub.Samples()

	// ASSERT
	require.NotNil(t, ch)
	assert.Equal(t, 4, cap(ch), "channel capacity must equal the requested buffer size")
}

func TestSubscription_Deliver_PutsEntryOnChannel(t *testing.T) {
	// ARRANGE
	sub := newSubscription(nil, 7, 4)
	entry := &proto.MetricsResponse{
		Timestamp: timestamppb.New(time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)),
	}

	// ACT
	sub.deliver(entry)

	// ASSERT
	select {
	case got := <-sub.Samples():
		assert.Same(t, entry, got, "delivered entry must be the same pointer as enqueued")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for delivered entry")
	}
}

func TestSubscription_Deliver_WhenBufferFull_DropsSilently(t *testing.T) {
	// ARRANGE
	sub := newSubscription(nil, 7, 1)
	first := &proto.MetricsResponse{Timestamp: timestamppb.Now()}
	second := &proto.MetricsResponse{Timestamp: timestamppb.Now()}

	// ACT
	sub.deliver(first)
	sub.deliver(second)

	// ASSERT
	select {
	case got := <-sub.Samples():
		assert.Same(t, first, got, "first entry occupies the buffer")
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for first entry")
	}

	// Buffer-full second deliver must drop without queueing — no further entry should be available.
	select {
	case got := <-sub.Samples():
		t.Fatalf("expected no more entries, got %v", got)
	case <-time.After(50 * time.Millisecond):
	}
}

func TestSubscription_Deliver_AfterCloseChannel_DoesNotPanic(t *testing.T) {
	// ARRANGE
	sub := newSubscription(nil, 7, 4)
	sub.closeChannel()

	// ACT / ASSERT — closed flag must short-circuit the send before it touches the closed channel.
	assert.NotPanics(t, func() {
		sub.deliver(&proto.MetricsResponse{Timestamp: timestamppb.Now()})
	})
}

func TestSubscription_CloseChannel_FirstCallClosesUnderlyingChannel(t *testing.T) {
	// ARRANGE
	// closeChannel is intentionally a single-call contract: a second call would panic by
	// re-closing the channel. We only verify the documented first-call behaviour here.
	sub := newSubscription(nil, 7, 4)
	first := &proto.MetricsResponse{Timestamp: timestamppb.Now()}
	second := &proto.MetricsResponse{Timestamp: timestamppb.Now()}
	sub.deliver(first)
	sub.deliver(second)

	// ACT
	sub.closeChannel()

	// ASSERT — ranging over a closed channel must drain remaining buffered entries and exit.
	drained := make([]*proto.MetricsResponse, 0, 2)
	for entry := range sub.Samples() {
		drained = append(drained, entry)
	}
	require.Len(t, drained, 2)
	assert.Same(t, first, drained[0])
	assert.Same(t, second, drained[1])
}
