package voice

import (
	"time"

	"github.com/google/uuid"
)

// VoiceMessage represents a voice message record in the database
type VoiceMessage struct {
	ID              uuid.UUID `json:"id"`
	RoomID          uuid.UUID `json:"room_id"`
	SenderID        uuid.UUID `json:"sender_id"`
	S3Key           string    `json:"s3_key"`
	DurationSeconds int       `json:"duration_seconds"`
	CreatedAt       time.Time `json:"created_at"`
}

// UploadVoiceMessageRequest is the metadata for uploading a voice message
// The actual audio file comes as multipart form data
type UploadVoiceMessageRequest struct {
	RoomID          uuid.UUID `json:"room_id"`
	DurationSeconds int       `json:"duration_seconds"`
}

// UploadVoiceMessageResponse returns info about the uploaded voice message
type UploadVoiceMessageResponse struct {
	Message VoiceMessage `json:"message"`
	URL     string       `json:"url"` // Presigned URL for playback
}

// GetRoomMessagesResponse returns voice messages for a room
type GetRoomMessagesResponse struct {
	Messages []VoiceMessageWithURL `json:"messages"`
	Count    int                   `json:"count"`
}

// VoiceMessageWithURL includes the message and a presigned URL
type VoiceMessageWithURL struct {
	VoiceMessage
	URL string `json:"url"`
}
