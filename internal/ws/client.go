package ws

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/coder/websocket"
)

const (
	defaultSendBufferSize = 64
	defaultMaxMessageSize = 8192
	defaultPingInterval   = 30 * time.Second
	defaultPongTimeout    = 60 * time.Second
)

type MessageHandler func(ctx context.Context, msg *InboundMessage)

type Client struct {
	conn    *websocket.Conn
	hub     *Hub
	send    chan []byte
	ctx     context.Context
	cancel  context.CancelFunc
	handler MessageHandler
	logger  *slog.Logger
}

type ClientConfig struct {
	PingInterval time.Duration
	PongTimeout  time.Duration
}

func NewClient(
	ctx context.Context,
	conn *websocket.Conn,
	hub *Hub,
	handler MessageHandler,
	logger *slog.Logger,
) *Client {
	ctx, cancel := context.WithCancel(ctx)

	if logger == nil {
		logger = slog.Default()
	}

	return &Client{
		conn:    conn,
		hub:     hub,
		send:    make(chan []byte, defaultSendBufferSize),
		ctx:     ctx,
		cancel:  cancel,
		handler: handler,
		logger:  logger,
	}
}

func (c *Client) SetMessageHandler(handler MessageHandler) {
	c.handler = handler
}

func (c *Client) Run() {
	go c.writePump()
	c.readPump()
}

func (c *Client) Send(msg []byte) {
	select {
	case c.send <- msg:
	default:
		c.logger.Warn("client send buffer full, dropping message")
	}
}

func (c *Client) SendMessage(msg *OutboundMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.logger.Warn("failed to marshal outbound message", "error", err)

		return
	}

	c.Send(data)
}

func (c *Client) Close() {
	c.cancel()
}

func (c *Client) Done() <-chan struct{} {
	return c.ctx.Done()
}

func (c *Client) readPump() {
	defer func() {
		c.hub.Unregister(c)
		c.cancel()
		c.conn.CloseNow() //nolint:errcheck
	}()

	c.conn.SetReadLimit(defaultMaxMessageSize)

	for {
		_, data, err := c.conn.Read(c.ctx)
		if err != nil {
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				c.logger.Debug("client disconnected normally")
			} else {
				select {
				case <-c.ctx.Done():
				default:
					c.logger.Debug("read error", "error", err)
				}
			}

			return
		}

		var msg InboundMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Debug("invalid message format", "error", err)

			continue
		}

		if msg.Type == TypePing {
			c.SendMessage(&OutboundMessage{
				Type:      TypePong,
				Timestamp: time.Now().Unix(),
			})

			continue
		}

		if c.handler != nil {
			c.handler(c.ctx, &msg)
		}
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(defaultPingInterval)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close(websocket.StatusGoingAway, "server shutting down")
	}()

	for {
		select {
		case <-c.ctx.Done():
			return

		case msg := <-c.send:
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.conn.Write(ctx, websocket.MessageText, msg)
			cancel()

			if err != nil {
				c.logger.Debug("write error", "error", err)

				return
			}

		case <-ticker.C:
			ctx, cancel := context.WithTimeout(c.ctx, 5*time.Second)
			err := c.conn.Ping(ctx)
			cancel()

			if err != nil {
				c.logger.Debug("ping failed", "error", err)

				return
			}
		}
	}
}
