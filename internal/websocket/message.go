package websocket

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
)

// MessageType defines the type of WebSocket messgae
type MessageType string

const (
	MessageTypeVoiceMessage MessageType = "voice_message"
	MessageTypeUserJoined   MessageType = "user_joined"
	MessageTypeUserLeft     MessageType = "user_left"
	MessageTypeConnected    MessageType = "connected"
	MessageTypeError        MessageType = "error"
)

// Message is the base structure for all WebSocket messages
type Message struct {
	Type MessageType `json:"type"`
	Data any         `json:"data"`
}

// VoiceMessageData represents a voice message being broadcast
type VoiceMessageData struct {
	ID             uuid.UUID `json:"id"`
	RoomID         uuid.UUID `json:"room_id"`
	SenderID       uuid.UUID `json:"sender_id"`
	URL            string    `json:"url"`
	DuratonSeconds int       `json:"duration_seconds"`
	CreatedAt      time.Time `json:"created_at"`
}

// UserJoinedData represents a user joining th eroom
type UserJoinedData struct {
	UserID   uuid.UUID `json:"user_id"`
	Username string    `json:"username"`
}

// UserLeftData represents a user leaving the room
type UserLeftData struct {
	UserID uuid.UUID `json:"user_id"`
}

// ConnectedData confirms successful connection
type ConnectedData struct {
	RoomID uuid.UUID `json:"room_id"`
	UserID uuid.UUID `json:"user_id"`
}

// ErrorData represents an error message
type ErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// NewVoiceMessage creates a voice message
func NewVoiceMessage(data VoiceMessageData) *Message {
	return &Message{
		Type: MessageTypeVoiceMessage,
		Data: data,
	}
}

// NewUserJoined creates a user joined message
func NewUserJoined(userID uuid.UUID, username string) *Message {
	return &Message{
		Type: MessageTypeUserJoined,
		Data: UserJoinedData{
			UserID:   userID,
			Username: username,
		},
	}
}

// NewUserLeft creates a user left message
func NewUserLeft(userID uuid.UUID) *Message {
	return &Message{
		Type: MessageTypeUserLeft,
		Data: UserLeftData{
			UserID: userID,
		},
	}
}

// NewConnected creates a connection confirmation message
func NewConnected(roomID, userID uuid.UUID) *Message {
	return &Message{
		Type: MessageTypeConnected,
		Data: ConnectedData{
			RoomID: roomID,
			UserID: userID,
		},
	}
}

// NewError creates an error message
func NewError(code, message string) *Message {
	return &Message{
		Type: MessageTypeError,
		Data: ErrorData{
			Code:    code,
			Message: message,
		},
	}
}

// ToJSON converts a message to JSON bytes
func (m *Message) ToJSON() ([]byte, error) {
	return json.Marshal(m)
}
