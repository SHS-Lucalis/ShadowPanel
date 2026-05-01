package serverconfigpush

import (
	"context"

	"github.com/gameap/gameap/pkg/proto"
)

// taskSender is the subset of *session.Registry that Pusher uses to dispatch
// gateway messages to a daemon. Defined as an interface so tests can substitute
// a stub without spinning up the real registry, pubsub, and gRPC streams.
type taskSender interface {
	SendTask(ctx context.Context, nodeID uint64, msg *proto.GatewayMessage) error
}
