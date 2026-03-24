package ws

import (
	"encoding/json"
	"time"
)

const (
	TypePing = "ping"
	TypePong = "pong"

	TypeError = "error"
)

type InboundMessage struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
	ID      string          `json:"id,omitempty"`
}

type OutboundMessage struct {
	Type      string `json:"type"`
	Payload   any    `json:"payload,omitempty"`
	ID        string `json:"id,omitempty"`
	Timestamp int64  `json:"ts"`
	Error     string `json:"error,omitempty"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

func NewOutboundMessage(msgType string, payload any) *OutboundMessage {
	return &OutboundMessage{
		Type:      msgType,
		Payload:   payload,
		Timestamp: time.Now().Unix(),
	}
}

func NewErrorMessage(message string) *OutboundMessage {
	return &OutboundMessage{
		Type:      TypeError,
		Payload:   ErrorPayload{Message: message},
		Timestamp: time.Now().Unix(),
	}
}
