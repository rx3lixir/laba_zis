package websocket

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Manager struct {
	clients   ClientList
	roomIndex map[uuid.UUID]ClientList
	sync.RWMutex
	log *slog.Logger
}

func NewManager(log *slog.Logger) *Manager {
	return &Manager{
		clients:   make(ClientList),
		roomIndex: make(map[uuid.UUID]ClientList),
		log:       log,
	}
}

// AddClient registers client and indexes by room
func (m *Manager) AddClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true

	if _, exists := m.roomIndex[client.roomID]; !exists {
		m.roomIndex[client.roomID] = make(ClientList)
	}
	m.roomIndex[client.roomID][client] = true

	m.log.Debug("Client connected", "user_id", client.userID, "room_id", client.roomID)
}

// RemoveClient removes from both global and room index
func (m *Manager) RemoveClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}

	if roomClients, ok := m.roomIndex[client.roomID]; ok {
		delete(roomClients, client)
		if len(roomClients) == 0 {
			delete(m.roomIndex, client.roomID)
		}
	}

	m.log.Debug("Client disconnected", "user_id", client.userID, "room_id", client.roomID)
}

func (m *Manager) BroadcastToRoom(roomID uuid.UUID, event Event) {
	data, err := json.Marshal(event)
	if err != nil {
		return
	}

	m.RLock()
	defer m.RUnlock()

	clients, exists := m.roomIndex[roomID]
	if !exists {
		return
	}

	for client := range clients {
		select {
		case client.egress <- data:
		default:
			m.log.Warn("Dropping message for slow client", "user_id", client.userID)
		}
	}
}

func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request, userID, roomID uuid.UUID) {
	m.log.Info("Upgrading WebSocket connection", "user_id", userID, "room_id", roomID)

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		m.log.Error("Failed to upgrade WebSocket", "error", err)
		http.Error(w, "Failed to upgrade", http.StatusBadRequest)
		return
	}

	client := NewClient(conn, m, userID, roomID, m.log)
	m.AddClient(client)

	go client.readMessages()
	go client.writeMessages()
}
