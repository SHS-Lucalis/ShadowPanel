package pubsub

import "errors"

var (
	ErrClosed             = errors.New("pubsub: closed")
	ErrPayloadTooLarge    = errors.New("pubsub: payload too large")
	ErrEmptyPattern       = errors.New("pubsub: empty pattern")
	ErrMaxRetriesExceeded = errors.New("pubsub: max retries exceeded")
)
