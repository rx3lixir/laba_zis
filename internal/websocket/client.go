package websocket

import (
	"github.com/gorilla/websocket"
	"github.com/rx3lixir/laba_zis/pkg/logger"
)

type ClientList map[*Client]bool

// Client is responsible for single connection
type Client struct {
	connection *websocket.Conn
	manager    *Manager
	egress     chan []byte
	log        logger.Logger
}

func NewClient(conn *websocket.Conn, manager *Manager, log logger.Logger) *Client {
	return &Client{
		connection: conn,
		manager:    manager,
		egress:     make(chan []byte),
		log:        log,
	}
}

func (c *Client) ReadMessages() {
	for {
		messageType, payload, err := c.connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				c.log.Error("Error reading message: %v", err)
			}
			break
		}

		for wsclient := range c.manager.clients {
			wsclient.egress <- payload
		}
	}
}

func (c *Client) WriteMessages() {
	defer func() {
		c.manager.RemoveClient(c)
	}()

	for {
		select {
		case message, ok := <-c.egress:
			if !ok {
				if err := c.connection.WriteMessage(websocket.CloseMessage, nil); err != nil {
					c.log.Info("Connection closed", err)
				}
				if err := c.connection.WriteMessage(websocket.TextMessage, message); err != nil {
					c.log.Error("Failed to send message", message)
				}
				c.log.Info("Message sent")
			}
		}
	}
}
