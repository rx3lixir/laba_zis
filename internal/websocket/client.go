package websocket

import (
	"context"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/google/uuid"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

const (
	// Time allowed to write a message to the peer
	writeWait = 10 * time.Second

	// Time allowed to read the next pong message from the peer
	pongWait = 60 * time.Second

	// Send pings to peer with this period (must be less than pongWait)
	pingPeriod = 30 * time.Second

	// Maximum message size allowed from peer (we don't expect clients to send much)
	maxMessageSize = 512
)

// Client represents a single WebSocket connection
type Client struct {
	// User and room info
	userID   uuid.UUID
	username string
	roomID   uuid.UUID

	// WebSocket connection
	conn *websocket.Conn

	// Hub reference
	hub *Hub

	// Buffered channel of outbound messages
	send chan *Message

	// Logger
	log logger.Logger

	// Rate limiting
	lastMessageTime time.Time
	mu              sync.Mutex
}

// NewClient creates a new client instance
func NewClient(
	userID uuid.UUID,
	username string,
	roomID uuid.UUID,
	conn *websocket.Conn,
	hub *Hub,
	log logger.Logger,
) *Client {
	return &Client{
		userID:          userID,
		username:        username,
		roomID:          roomID,
		conn:            conn,
		hub:             hub,
		send:            make(chan *Message, 256),
		log:             log,
		lastMessageTime: time.Now(),
	}
}

// readPump pumps messages from the WebSocket connection to the hub
// The application runs readPump in a per-connection goroutine
// readPump pumps messages from the WebSocket connection to the hub
func (c *Client) readPump(ctx context.Context) {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close(websocket.StatusNormalClosure, "read pump closed")
	}()

	for {
		// Create a context for this specific read operation with a reasonable timeout
		readCtx, cancel := context.WithTimeout(ctx, pongWait)

		_, _, err := c.conn.Read(readCtx)
		cancel() // Always cancel to free resources

		if err != nil {
			// Check if it's the hub shutting down
			if ctx.Err() != nil {
				c.log.Debug("Hub context cancelled, closing connection",
					"user_id", c.userID,
					"room_id", c.roomID,
				)
				return
			}

			// Check if it's a normal closure
			if websocket.CloseStatus(err) == websocket.StatusNormalClosure ||
				websocket.CloseStatus(err) == websocket.StatusGoingAway {
				c.log.Debug("Client disconnected normally",
					"user_id", c.userID,
					"room_id", c.roomID,
				)
			} else {
				c.log.Warn("WebSocket read error",
					"user_id", c.userID,
					"room_id", c.roomID,
					"error", err,
				)
			}
			return
		}

		// If we ever want to handle client messages, add logic here
	}
}

// writePump pumps messages from the hub to the WebSocket connection
// The application runs writePump in a per-connection goroutine
func (c *Client) writePump(ctx context.Context) {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close(websocket.StatusNormalClosure, "write pump closed")
	}()

	for {
		select {
		case message, ok := <-c.send:
			if !ok {
				// Hub closed the channel
				c.log.Debug("Send channel closed",
					"user_id", c.userID,
					"room_id", c.roomID,
				)
				c.conn.Close(websocket.StatusNormalClosure, "hub closed channel")
				return
			}

			// Don't use the parent context here - create a fresh one for each write
			writeCtx, cancel := context.WithTimeout(context.Background(), writeWait)
			err := c.writeMessage(writeCtx, message)
			cancel()

			if err != nil {
				c.log.Error("Failed to write message",
					"user_id", c.userID,
					"room_id", c.roomID,
					"error", err,
				)
				return
			}

		case <-ticker.C:
			// Send ping to keep connection alive
			writeCtx, cancel := context.WithTimeout(context.Background(), writeWait)
			err := c.conn.Ping(writeCtx)
			cancel()

			if err != nil {
				c.log.Warn("Failed to send ping",
					"user_id", c.userID,
					"room_id", c.roomID,
					"error", err,
				)
				return
			}

		case <-ctx.Done():
			// Only exit when the hub is shutting down
			c.log.Debug("Hub context cancelled, closing write pump",
				"user_id", c.userID,
				"room_id", c.roomID,
			)
			return
		}
	}
}

// writeMessage writes a message to the WebSocket connection
func (c *Client) writeMessage(ctx context.Context, message *Message) error {
	data, err := message.ToJSON()
	if err != nil {
		return err
	}

	return c.conn.Write(ctx, websocket.MessageText, data)
}

// canSendMessage checks rate limiting (max 1 message per second)
func (c *Client) canSendMessage() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	if now.Sub(c.lastMessageTime) < time.Second {
		return false
	}

	c.lastMessageTime = now
	return true
}
