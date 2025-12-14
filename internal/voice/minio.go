package voice

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type MinIOVoiceStore struct {
	client     *minio.Client
	bucketName string
}

func NewMinIOVoiceStore(client *minio.Client, bucketName string) *MinIOVoiceStore {
	return &MinIOVoiceStore{
		client:     client,
		bucketName: bucketName,
	}
}

// generateObjectName creates a consistent S3 key for voice messages
func (m *MinIOVoiceStore) generateObjectName(messageID uuid.UUID, audioFormat string) string {
	now := time.Now()

	// FIXED: Your original code had day/month swapped!
	return fmt.Sprintf(
		"messages/%d/%02d/%02d/%s.%s",
		now.Year(),
		now.Month(),
		now.Day(),
		messageID.String(),
		audioFormat,
	)
}

// UploadVoiceMessage uplads a voice message to MinIO
func (m *MinIOVoiceStore) UploadVoiceMessage(
	ctx context.Context,
	messageID uuid.UUID,
	reader io.Reader,
	size int64,
	audioFormat string,
) (string, error) {
	objectName := m.generateObjectName(messageID, audioFormat)

	contentType := getContentType(audioFormat)

	_, err := m.client.PutObject(
		ctx,
		m.bucketName,
		objectName,
		reader,
		size,
		minio.PutObjectOptions{
			ContentType: contentType,
			UserMetadata: map[string]string{
				"message-id": messageID.String(),
				"uploaded":   time.Now().Format(time.RFC3339),
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload to minio: %w", err)
	}

	return objectName, nil
}

// DownloadVoiceMessage downloads a voice message from MinIO
func (m *MinIOVoiceStore) DownloadVoiceMessage(ctx context.Context, objectName string) ([]byte, error) {
	object, err := m.client.GetObject(ctx, m.bucketName, objectName, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object: %w", err)
	}
	defer object.Close()

	data, err := io.ReadAll(object)
	if err != nil {
		return nil, fmt.Errorf("failed to read object: %w", err)
	}

	return data, nil
}

// DeleteVoiceMessage deletes a voice message from MinIO
func (m *MinIOVoiceStore) DeleteVoiceMessage(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// DeleteVoiceMessage deletes a voice message from MinIO
func (m *MinIOVoiceStore) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}
	return url.String(), nil
}

// GetObjectInfo retrieves metadata about a stored object
func (m *MinIOVoiceStore) GetObjectInfo(ctx context.Context, objectName string) (*minio.ObjectInfo, error) {
	info, err := m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}
	return &info, nil
}

// getContentType maps audio format to MIME type
func getContentType(audioFormat string) string {
	switch audioFormat {
	case "webm":
		return "audio/webm"
	case "m4a", "mp4":
		return "audio/mp4"
	case "mp3":
		return "audio/mpeg"
	case "ogg", "opus":
		return "audio/ogg"
	case "wav":
		return "audio/wav"
	default:
		return "application/octet-stream"
	}
}
