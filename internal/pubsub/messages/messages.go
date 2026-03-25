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

	TypeDaemonConnected    = "daemon.connected"
	TypeDaemonClosed       = "daemon.closed"
	TypeDaemonTask         = "daemon.task"
	TypeDaemonCommand      = "daemon.command"
	TypeDaemonServerConfig = "daemon.server_config"

	TypeTaskStatus    = "task.status"
	TypeTaskOutput    = "task.output"
	TypeTaskComplete  = "task.complete"
	TypeConsoleOutput = "console.output"
	TypeConsoleResult = "console.result"

	TypeDaemonFileRequest          = "daemon.file.request"
	TypeDaemonFileResponse         = "daemon.file.response"
	TypeDaemonFileTransferComplete = "daemon.file.transfer.complete"
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

type DaemonSessionPayload struct {
	NodeID      uint64    `json:"node_id"`
	InstanceID  string    `json:"instance_id"`
	Version     string    `json:"version"`
	ConnectedAt time.Time `json:"connected_at"`
}

type DaemonTaskDispatchPayload struct {
	NodeID    uint64 `json:"node_id"`
	RequestID string `json:"request_id"`
	TaskID    uint64 `json:"task_id"`
	TaskData  []byte `json:"task_data"`
}

type DaemonCommandDispatchPayload struct {
	NodeID    uint64 `json:"node_id"`
	RequestID string `json:"request_id"`
	CommandID string `json:"command_id"`
	ServerID  uint64 `json:"server_id"`
	Command   string `json:"command"`
	Timeout   int32  `json:"timeout"`
}

type TaskStatusPayload struct {
	TaskID   uint64 `json:"task_id"`
	Status   string `json:"status"`
	ServerID uint   `json:"server_id"`
	Message  string `json:"message,omitempty"`
}

type TaskOutputPayload struct {
	TaskID  uint64 `json:"task_id"`
	Chunk   string `json:"chunk"`
	IsFinal bool   `json:"is_final"`
}

type TaskCompletePayload struct {
	TaskID   uint64 `json:"task_id"`
	Status   string `json:"status"`
	ServerID uint   `json:"server_id"`
}

type ConsoleOutputPayload struct {
	ServerID  uint64 `json:"server_id"`
	CommandID string `json:"command_id,omitempty"`
	Chunk     string `json:"chunk"`
}

type ConsoleResultPayload struct {
	ServerID  uint64 `json:"server_id"`
	CommandID string `json:"command_id,omitempty"`
	ExitCode  int32  `json:"exit_code"`
	Error     string `json:"error,omitempty"`
}

type DaemonFileRequestPayload struct {
	NodeID      uint64 `json:"node_id"`
	RequestID   string `json:"request_id"`
	InstanceID  string `json:"instance_id"`
	Operation   string `json:"operation"`
	Data        []byte `json:"data,omitempty"`
	TransferID  string `json:"transfer_id,omitempty"`
	StoragePath string `json:"storage_path,omitempty"`
}

type DaemonFileResponsePayload struct {
	RequestID   string `json:"request_id"`
	Error       string `json:"error,omitempty"`
	Data        []byte `json:"data,omitempty"`
	StoragePath string `json:"storage_path,omitempty"`
}

type FileTransferCompletePayload struct {
	TransferID string `json:"transfer_id"`
	Success    bool   `json:"success"`
	Error      string `json:"error,omitempty"`
	Checksum   string `json:"checksum,omitempty"`
}

type DaemonServerConfigPayload struct {
	NodeID     uint64 `json:"node_id"`
	RequestID  string `json:"request_id"`
	ConfigData []byte `json:"config_data"`
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
