package ws

import (
	"log/slog"
	"sync"
)

type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}
	topics  map[string]map[*Client]struct{}
	logger  *slog.Logger
}

func NewHub(logger *slog.Logger) *Hub {
	if logger == nil {
		logger = slog.Default()
	}

	return &Hub{
		clients: make(map[*Client]struct{}),
		topics:  make(map[string]map[*Client]struct{}),
		logger:  logger,
	}
}

func (h *Hub) Register(client *Client, topics ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.clients[client] = struct{}{}
	for _, topic := range topics {
		if h.topics[topic] == nil {
			h.topics[topic] = make(map[*Client]struct{})
		}
		h.topics[topic][client] = struct{}{}
	}

	h.logger.Debug("client registered",
		"topics", topics,
		"total_clients", len(h.clients),
	)
}

func (h *Hub) Subscribe(client *Client, topics ...string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for _, topic := range topics {
		if h.topics[topic] == nil {
			h.topics[topic] = make(map[*Client]struct{})
		}
		h.topics[topic][client] = struct{}{}
	}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	delete(h.clients, client)
	for topic, clients := range h.topics {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.topics, topic)
		}
	}
}

func (h *Hub) Broadcast(topic string, msg []byte) {
	h.mu.RLock()
	clients := h.topics[topic]
	if len(clients) == 0 {
		h.mu.RUnlock()

		return
	}

	targets := make([]*Client, 0, len(clients))
	for c := range clients {
		targets = append(targets, c)
	}
	h.mu.RUnlock()

	for _, c := range targets {
		c.Send(msg)
	}
}

func (h *Hub) TopicSubscriberCount(topic string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	return len(h.topics[topic])
}

func (h *Hub) Close() {
	h.mu.Lock()
	clients := make([]*Client, 0, len(h.clients))
	for c := range h.clients {
		clients = append(clients, c)
	}
	h.mu.Unlock()

	for _, c := range clients {
		c.Close()
	}
}
