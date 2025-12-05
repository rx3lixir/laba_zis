package websocket

import "github.com/google/uuid"

type EventType string

const (
	EventNewVoiceMessage EventType = "new_voice_message"
	EventUserJoined      EventType = "user_joined"
	EventUserLeft        EventType = "user_left"
)

type Event struct {
	Type EventType `json:"type"`
	Data any       `json:"data"`
}

type NewVoiceMessageEvent struct {
	MessageID uuid.UUID `json:"message_id"`
	RoomID    uuid.UUID `json:"room_id"`
	SenderID  uuid.UUID `json:"sender_id"`
	Duration  int       `json:"duration_seconds"`
	URL       string    `json:"url"`
	CreatedAt int64     `json:"created_at"`
}
