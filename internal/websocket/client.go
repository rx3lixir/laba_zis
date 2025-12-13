package websocket

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 8192 // 8KB for JSON messages
)

type Client struct {
	hub    *Hub
	conn   *websocket.Conn
	send   chan []byte
	userID uuid.UUID
	log    *slog.Logger
}

func NewClient(hub *Hub, conn *websocket.Conn, userID uuid.UUID, log *slog.Logger) *Client {
	return &Client{
		hub:    hub,
		conn:   conn,
		send:   make(chan []byte, 256),
		userID: userID,
		log:    log,
	}
}

func (c *Client) SendMessage(msg ServerMessage) {
	data, err := json.Marshal(msg)
	if err != nil {
		c.log.Error("failed to marshal message", "error", err)
		return
	}

	select {
	case c.send <- data:
	default:
		c.log.Warn("client send buffer full", "user_id", c.userID)
	}
}

// readPump pumps messages from WebSocket to hub
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(appData string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure) {
				c.log.Error("webSocket error", "error", err, "user_id", c.userID)
			}
			break
		}
		// Parse and handle client message
		var clientMsg ClientMessage
		if err := json.Unmarshal(message, &clientMsg); err != nil {
			c.log.Warn("invalid message format", "error", err, "user_id", c.userID)
			c.sendError("invalid message format")
			continue
		}
		c.handleClientMessage(clientMsg)
	}
}

// writePump pumps messages from hub to WebSocket
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// Add queued messages to current websocket frame (optimization)
			n := len(c.send)
			for i := 0; i < n; i++ {
				w.Write([]byte{'\n'})
				w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) handleClientMessage(msg ClientMessage) {
	switch msg.Type {
	case TypePing:
		c.SendMessage(ServerMessage{Type: TypePong})

	case TypeTyping:
		// Could broadcast typing indicators
		c.log.Debug("user typing", "user_id", c.userID)

	case TypeReadReceipt:
		// Handle read receipts
		c.log.Debug("read receipt", "user_id", c.userID)

	default:
		c.log.Warn("unknown message type", "type", msg.Type, "user_id", c.userID)
	}
}

func (c *Client) sendError(message string) {
	c.SendMessage(ServerMessage{
		Type: TypeError,
		Data: map[string]string{"error": message},
	})
}
