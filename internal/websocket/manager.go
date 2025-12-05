package websocket

import (
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

var websocketUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		origin := r.Header.Get("Origin")
		allowed := []string{
			"http://localhost:3000",
			"https://localhost:3000",
		}
		for _, o := range allowed {
			if origin == o {
				return true
			}
		}
		return false
	},
}

type Manager struct {
	clients ClientList
	sync.RWMutex
	log logger.Logger
}

func NewManager(log logger.Logger) *Manager {
	return &Manager{
		clients: make(ClientList),
		log:     log,
	}
}

func (m *Manager) ServeWS(w http.ResponseWriter, r *http.Request) {
	m.log.Info("New connection. Upgrading connection...")

	conn, err := websocketUpgrader.Upgrade(w, r, nil)
	if err != nil {
		m.log.Error("Failed to upgrade WebSocket", "error", err)
		http.Error(w, "Failed to upgrade to WebSocket", http.StatusBadRequest)
		return
	}

	client := NewClient(conn, m)

	m.AddClient(client)

	go client.readMessages()
	go client.writeMessages()
}

func (m *Manager) AddClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	m.clients[client] = true
}

func (m *Manager) RemoveClient(client *Client) {
	m.Lock()
	defer m.Unlock()

	if _, ok := m.clients[client]; ok {
		client.connection.Close()
		delete(m.clients, client)
	}
}
