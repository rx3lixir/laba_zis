package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
)

type Hub struct {
	// Room identifier
	roomID uuid.UUID

	// Registered clients (only accessed by hub goroutine)
	clients map[*Client]bool

	// Inbound messages from clients
	broadcast chan ServerMessage

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Shutdown signal
	shutdown chan struct{}

	// Metrics
	metrics *HubMetrics

	log *slog.Logger
}

type HubMetrics struct {
	ConnectedClients int
	MessagesSent     int64
	MessagesDropped  int64
	LastActivity     time.Time
}

func NewHub(roomID uuid.UUID, log *slog.Logger) *Hub {
	return &Hub{
		roomID:     roomID,
		clients:    make(map[*Client]bool),
		broadcast:  make(chan ServerMessage, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		shutdown:   make(chan struct{}),
		metrics:    &HubMetrics{LastActivity: time.Now()},
		log:        log,
	}
}

// Run is the main event loop - handles ALL state changes sequentially
func (h *Hub) Run() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case client := <-h.register:
			h.handleRegister(client)

		case client := <-h.unregister:
			h.handleUnregister(client)

		case message := <-h.broadcast:
			h.handleBroadcast(message)

		case <-ticker.C:
			h.handleHealthCheck()

		case <-h.shutdown:
			h.handleShutdown()
			return
		}
	}
}

func (h *Hub) handleRegister(client *Client) {
	h.clients[client] = true
	h.metrics.ConnectedClients = len(h.clients)

	h.log.Info("client registered",
		"room_id", h.roomID,
		"user_id", client.userID,
		"total_clients", len(h.clients),
	)

	// Send acknowledgment
	ack := ServerMessage{
		Type: TypeConnectionAck,
		Data: map[string]any{
			"room_id": h.roomID,
			"user_id": client.userID,
		},
		Timestamp: time.Now().Unix(),
	}
	client.SendMessage(ack)

	// Notify others
	h.broadcastUserJoined(client.userID)
}

func (h *Hub) handleUnregister(client *Client) {
	if _, ok := h.clients[client]; ok {
		delete(h.clients, client)
		close(client.send) // Signal client to stop
		h.metrics.ConnectedClients = len(h.clients)

		h.log.Info("client unregistered",
			"room_id", h.roomID,
			"user_id", client.userID,
			"remaining_clients", len(h.clients),
		)

		// Notify others
		h.broadcastUserLeft(client.userID)
	}
}

func (h *Hub) handleBroadcast(message ServerMessage) {
	h.metrics.LastActivity = time.Now()
	message.Timestamp = time.Now().Unix()

	data, err := json.Marshal(message)
	if err != nil {
		h.log.Error("failed to marshal message", "error", err)
		return
	}

	// Send to all clients
	for client := range h.clients {
		select {
		case client.send <- data:
			h.metrics.MessagesSent++
		default:
			// Client is too slow, disconnect it
			h.log.Warn("client buffer full, disconnecting",
				"user_id", client.userID,
				"room_id", h.roomID,
			)
			h.metrics.MessagesDropped++
			h.handleUnregister(client)
		}
	}
}

func (h *Hub) handleHealthCheck() {
	// If no clients and idle for 5 minutes, could signal for cleanup
	if len(h.clients) == 0 && time.Since(h.metrics.LastActivity) > 5*time.Minute {
		h.log.Info("hub idle, considering cleanup", "room_id", h.roomID)
		// Manager could implement cleanup logic
	}
}

func (h *Hub) handleShutdown() {
	h.log.Info("shutting down hub", "room_id", h.roomID)

	// Gracefully close all clients
	for client := range h.clients {
		close(client.send)
		client.conn.Close()
	}

	close(h.broadcast)
	h.clients = nil
}

func (h *Hub) broadcastUserJoined(userID uuid.UUID) {
	h.broadcast <- ServerMessage{
		Type: TypeUserJoined,
		Data: map[string]any{"user_id": userID},
	}
}

func (h *Hub) broadcastUserLeft(userID uuid.UUID) {
	h.broadcast <- ServerMessage{
		Type: TypeUserLeft,
		Data: map[string]any{"user_id": userID},
	}
}

func (h *Hub) Send(message ServerMessage) {
	select {
	case h.broadcast <- message:
	default:
		h.log.Error("hub broadcast channel full", "room_id", h.roomID)
		h.metrics.MessagesDropped++
	}
}

func (h *Hub) Shutdown() {
	close(h.shutdown)
}
