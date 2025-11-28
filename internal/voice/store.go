package voice

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type VoiceMessageStore interface {
	UploadVoiceMessage(ctx context.Context, messageID uuid.UUID, data []byte, audioFormat string) (string, error)
	DownloadVoiceMessage(ctx context.Context, objectName string) ([]byte, error)
	DeleteVoiceMessage(ctx context.Context, objectName string) error
	GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error)
}
