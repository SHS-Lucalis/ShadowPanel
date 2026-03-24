# WebSocket Infrastructure (`internal/ws`)

Shared WebSocket infrastructure for real-time browser communication.

## Architecture

```
Browser --WS--> Handler (upgrade) --> Client (read/write pumps)
                                          |
                                     Hub (topic-based fan-out)
                                          |
                                     Bridge (pub-sub subscriber)
                                          |
                                     Pub-Sub (realtime channels)
```

## Components

- **`message.go`** - JSON message envelope types (`InboundMessage`, `OutboundMessage`)
- **`client.go`** - Single WebSocket connection with read/write pumps and ping/pong heartbeat
- **`hub.go`** - Client registry with topic-based message fan-out
- **`bridge.go`** - Subscribes to pub-sub realtime channels and forwards events to the hub
- **`auth.go`** - Auth session extraction helpers

## Message Format

All WebSocket messages use a JSON envelope:

```json
{
  "type": "task.status",
  "payload": { ... },
  "ts": 1711234567
}
```

## Endpoints

- `GET /api/ws/tasks/{id}?token=<bearer>` - Real-time task status and output
- `GET /api/ws/servers/{server}/console?token=<bearer>` - Bidirectional server console

