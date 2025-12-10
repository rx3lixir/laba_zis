package websocket

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Maximum message size allowed from peer
	maxMessageSize = 512
)

// Client represents a single websocket connection
type Client struct {
	manager    *Manager
	connection *websocket.Conn
	userID     uuid.UUID
	roomID     uuid.UUID
	egress     chan []byte
	log        *slog.Logger
}

type ClientList map[*Client]bool

type roomClients struct {
	sync.RWMutex
	clients map[uuid.UUID]ClientList
}

// NewClient creates a new websocket client
func NewClient(conn *websocket.Conn, manager *Manager, userID, roomID uuid.UUID, log *slog.Logger) *Client {
	return &Client{
		connection: conn,
		manager:    manager,
		userID:     userID,
		roomID:     roomID,
		egress:     make(chan []byte, 256),
		log:        log,
	}
}

// readMessages pumps messages from the websocket connection to the manager
func (c *Client) readMessages() {
	defer func() {
		c.manager.RemoveClient(c)
		c.connection.Close()
	}()

	c.connection.SetReadDeadline(time.Now().Add(pongWait))
	c.connection.SetPongHandler(func(string) error {
		c.connection.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	c.connection.SetReadLimit(maxMessageSize)

	for {
		_, msg, err := c.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error("Unexpected websocket close", "error", err, "user_id", c.userID, "room_id", c.roomID)
			}
			break
		}

		var message struct {
			Type string `json:"type"`
		}

		if err := json.Unmarshal(msg, &message); err != nil {
			c.log.Warn("Failed to parse message", "error", err)
			continue
		}

		// Handle ping/pong
		if message.Type == "ping" {
			response := Event{Type: "pong", Data: map[string]int64{"timestamp": time.Now().Unix()}}
			data, _ := json.Marshal(response)
			c.egress <- data
		}

		// Can be extended with typing, read receipts, etc.
	}
}

// writeMessages pumps messages from the manager to the websocket connection
func (c *Client) writeMessages() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.connection.Close()
	}()

	for {
		select {
		case message, ok := <-c.egress:
			c.connection.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Manager closed the channel
				c.connection.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			if err := c.connection.WriteMessage(websocket.TextMessage, message); err != nil {
				c.log.Error("Failed to write message", "error", err, "user_id", c.userID)
				return
			}

			c.log.Debug("Message sent to client", "user_id", c.userID, "room_id", c.roomID)

		case <-ticker.C:
			c.connection.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.connection.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
