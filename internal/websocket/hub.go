package websocket

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

// Hub maintains the set of active clients and broadcasts messages to them
type Hub struct {
	// Registered clients organized by room
	// rooms[roomID][client] = true
	rooms map[uuid.UUID]map[*Client]bool

	// Register requests from clients
	register chan *Client

	// Unregister requests from clients
	unregister chan *Client

	// Broadcast messages to a specific room
	broadcast chan *BroadcastMessage

	// Logger
	log logger.Logger

	// Mutex for thread-safe access to rooms
	mu sync.RWMutex

	// Context for graceful shutdown
	ctx context.Context
}

// BroadcastMessage contains a message and the target room
type BroadcastMessage struct {
	RoomID  uuid.UUID
	Message *Message
}

// NewHub creates a new Hub instance
func NewHub(ctx context.Context, log logger.Logger) *Hub {
	return &Hub{
		rooms:      make(map[uuid.UUID]map[*Client]bool),
		register:   make(chan *Client),
		unregister: make(chan *Client),
		broadcast:  make(chan *BroadcastMessage, 256),
		log:        log,
		ctx:        ctx,
	}
}

// Run starts the hub's main loop
// This should be run in a goroutine
func (h *Hub) Run() {
	h.log.Info("WebSocket hub started")

	for {
		select {
		case client := <-h.register:
			h.registerClient(client)

		case client := <-h.unregister:
			h.unregisterClient(client)

		case broadcast := <-h.broadcast:
			h.broadcastToRoom(broadcast)

		case <-h.ctx.Done():
			h.log.Info("WebSocket hub shutting down")
			h.closeAllConnections()
			return
		}
	}
}

// registerClient adds a client to a room
func (h *Hub) registerClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// Create room map if it doesn't exist
	if h.rooms[client.roomID] == nil {
		h.rooms[client.roomID] = make(map[*Client]bool)
	}

	// Add client to room
	h.rooms[client.roomID][client] = true

	h.log.Debug("Client registered",
		"user_id", client.userID,
		"username", client.username,
		"room_id", client.roomID,
		"room_size", len(h.rooms[client.roomID]),
	)

	// Notify other users in the room
	userJoinedMsg := NewUserJoined(client.userID, client.username)
	h.broadcastToRoomExcept(client.roomID, userJoinedMsg, client)
}

// unregisterClient removes a client from a room
func (h *Hub) unregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if _, ok := h.rooms[client.roomID]; ok {
		if _, ok := h.rooms[client.roomID][client]; ok {
			// Remove client from room
			delete(h.rooms[client.roomID], client)
			close(client.send)

			// Clean up empty room
			if len(h.rooms[client.roomID]) == 0 {
				delete(h.rooms, client.roomID)
			}

			h.log.Debug("Client unregistered",
				"user_id", client.userID,
				"room_id", client.roomID,
			)

			// Notify other users in the room
			userLeftMsg := NewUserLeft(client.userID)
			h.broadcastToRoomExcept(client.roomID, userLeftMsg, client)
		}
	}
}

// broadcastToRoom sends a message to all clients in a room
func (h *Hub) broadcastToRoom(bm *BroadcastMessage) {
	h.mu.RLock()
	clients := h.rooms[bm.RoomID]
	h.mu.RUnlock()

	if clients == nil {
		h.log.Debug("No clients in room to broadcast to", "room_id", bm.RoomID)
		return
	}

	h.log.Debug("Broadcasting to room",
		"room_id", bm.RoomID,
		"message_type", bm.Message.Type,
		"client_count", len(clients),
	)

	for client := range clients {
		select {
		case client.send <- bm.Message:
			// Message sent successfully
		default:
			// Client's send channel is full, disconnect them
			h.log.Warn("Client send channel full, disconnecting",
				"user_id", client.userID,
				"room_id", client.roomID,
			)
			h.unregisterClient(client)
		}
	}
}

// broadcastToRoomExcept sends a message to all clients in a room except one
func (h *Hub) broadcastToRoomExcept(roomID uuid.UUID, message *Message, except *Client) {
	clients := h.rooms[roomID]
	if clients == nil {
		return
	}

	for client := range clients {
		if client == except {
			continue
		}

		select {
		case client.send <- message:
			// Message sent successfully
		default:
			// Client's send channel is full, disconnect them
			h.log.Warn("Client send channel full, disconnecting",
				"user_id", client.userID,
				"room_id", client.roomID,
			)
			h.unregisterClient(client)
		}
	}
}

// BroadcastToRoom is the public method for broadcasting to a room
// This is what other packages will use
func (h *Hub) BroadcastToRoom(roomID uuid.UUID, message any) {
	// Convert any to *Message
	var msg *Message

	switch v := message.(type) {
	case *Message:
		msg = v
	case map[string]any:
		// Handle raw map from voice handler
		msg = &Message{
			Type: MessageType(v["type"].(string)),
			Data: v["data"],
		}
	default:
		h.log.Error("Invalid message type for broadcast", "type", fmt.Sprintf("%T", message))
		return
	}

	h.broadcast <- &BroadcastMessage{
		RoomID:  roomID,
		Message: msg,
	}
}

// GetRoomClientCount returns the number of connected clients in a room
func (h *Hub) GetRoomClientCount(roomID uuid.UUID) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.rooms[roomID]; ok {
		return len(clients)
	}
	return 0
}

// closeAllConnections gracefully closes all client connections
func (h *Hub) closeAllConnections() {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for roomID, clients := range h.rooms {
		for client := range clients {
			close(client.send)
			h.log.Debug("Closed client connection during shutdown",
				"user_id", client.userID,
				"room_id", roomID,
			)
		}
	}

	// Clear all rooms
	h.rooms = make(map[uuid.UUID]map[*Client]bool)
}
