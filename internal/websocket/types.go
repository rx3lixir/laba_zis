package websocket

import (
	"encoding/json"

	"github.com/google/uuid"
)

// MessageType defines the type of message
type MessageType string

const (
	// Client -> Server
	TypePing        MessageType = "ping"
	TypeTyping      MessageType = "typing"
	TypeReadReceipt MessageType = "read_receipt"

	// Server -> Client
	TypePong            MessageType = "pong"
	TypeNewVoiceMessage MessageType = "new_voice_message"
	TypeUserJoined      MessageType = "user_joined"
	TypeUserLeft        MessageType = "user_left"
	TypeError           MessageType = "error"
	TypeConnectionAck   MessageType = "connection_ack"
)

// ClientMessage represents any message from client
type ClientMessage struct {
	Type MessageType     `json:"type"`
	Data json.RawMessage `json:"data,omitempty"`
}

// ServerMessage represents any message to client
type ServerMessage struct {
	Type      MessageType `json:"type"`
	Data      any         `json:"data,omitempty"`
	Timestamp int64       `json:"timestamp"`
}

// VoiceMessageData is the payload for new voice messages
type VoiceMessageData struct {
	MessageID uuid.UUID `json:"message_id"`
	SenderID  uuid.UUID `json:"sender_id"`
	Duration  int       `json:"duration"`
	URL       string    `json:"url"`
}
