package voice

import (
	"context"
	"io"
	"time"

	"github.com/google/uuid"
)

// VoiceMessageStore handles S3 operations for voice files
type VoiceMessageStore interface {
	UploadVoiceMessage(ctx context.Context, messageID uuid.UUID, reader io.Reader, size int64, audioFormat string) (string, error)
	DownloadVoiceMessage(ctx context.Context, objectName string) ([]byte, error)
	DeleteVoiceMessage(ctx context.Context, objectName string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}

// VoiceMessageDBStore handles database operations for voice message metadata
type VoiceMessageDBStore interface {
	CreateVoiceMessage(ctx context.Context, message *VoiceMessage) error
	GetVoiceMessageByID(ctx context.Context, messageID uuid.UUID) (*VoiceMessage, error)
	GetRoomMessages(ctx context.Context, roomID uuid.UUID, limit, offset int) ([]*VoiceMessage, error)
	DeleteVoiceMessage(ctx context.Context, messageID uuid.UUID) error
	GetMessagesBySender(ctx context.Context, senderID uuid.UUID, limit, offset int) ([]*VoiceMessage, error)
}
