package messages

import (
	"encoding/json"
	"time"

	"github.com/gameap/gameap/internal/pubsub"
	"github.com/google/uuid"
)

const (
	TypeCacheInvalidate = "cache.invalidate"
	TypePluginEvent     = "plugin.event"
	TypeServerStatus    = "server.status"
	TypeTaskProgress    = "task.progress"
	TypeNotification    = "notification"
)

type CacheInvalidatePayload struct {
	EntityType string   `json:"entity_type"`
	EntityIDs  []string `json:"entity_ids,omitempty"`
	Pattern    string   `json:"pattern,omitempty"`
}

type PluginEventPayload struct {
	EventType int32             `json:"event_type"`
	ServerID  *uint             `json:"server_id,omitempty"`
	TaskID    *uint             `json:"task_id,omitempty"`
	NodeID    *uint             `json:"node_id,omitempty"`
	ExtraData map[string]string `json:"extra_data,omitempty"`
}

type ServerStatusPayload struct {
	ServerID      uint   `json:"server_id"`
	Status        string `json:"status"`
	PlayersOnline int    `json:"players_online"`
	MaxPlayers    int    `json:"max_players"`
}

type TaskProgressPayload struct {
	TaskID   uint   `json:"task_id"`
	ServerID *uint  `json:"server_id,omitempty"`
	Status   string `json:"status"`
	Progress int    `json:"progress"`
	Message  string `json:"message,omitempty"`
}

func NewMessage(channel, msgType string, payload any) (*pubsub.Message, error) {
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	return &pubsub.Message{
		ID:        uuid.New().String(),
		Channel:   channel,
		Type:      msgType,
		Payload:   data,
		Timestamp: time.Now(),
	}, nil
}

func ParsePayload[T any](msg *pubsub.Message) (T, error) {
	var payload T

	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return payload, err
	}

	return payload, nil
}
