package rooms

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

type MinIOStore struct {
	client     *minio.Client
	bucketName string
}

func NewMinIOStore(client *minio.Client, bucketName string) *MinIOStore {
	return &MinIOStore{client: client, bucketName: bucketName}
}

func (m *MinIOStore) UploadVoiceMessage(
	ctx context.Context,
	messageID uuid.UUID,
	data []byte,
	audioFormat string,
) (string, error) {
	now := time.Now()

	objectName := fmt.Sprintf(
		"messages/%d/%02d/%02d/%s.%s",
		now.Year(),
		now.Day(),
		now.Month(),
		messageID.String(),
		audioFormat,
	)

	contentType := "audio/opus"

	reader := bytes.NewReader(data)

	_, err := m.client.PutObject(
		ctx,
		m.bucketName,
		objectName,
		reader,
		int64(len(data)),
		minio.PutObjectOptions{
			ContentType: contentType,
		},
	)
	if err != nil {
		return "", fmt.Errorf("failed to upload to minio: %w", err)
	}

	return objectName, nil
}

// DownloadVoiceMessage downloads a voice message from MinIO
func (m *MinIOStore) DownloadVoiceMessage(ctx context.Context, objectName string) ([]byte, error) {
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
func (m *MinIOStore) DeleteVoiceMessage(ctx context.Context, objectName string) error {
	err := m.client.RemoveObject(ctx, m.bucketName, objectName, minio.RemoveObjectOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete object: %w", err)
	}
	return nil
}

// DeleteVoiceMessage deletes a voice message from MinIO
func (m *MinIOStore) GetPresignedURL(ctx context.Context, objectName string, expiry time.Duration) (string, error) {
	url, err := m.client.PresignedGetObject(ctx, m.bucketName, objectName, expiry, nil)
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned url: %w", err)
	}
	return url.String(), nil
}

// GetObjectInfo retrieves metadata about a stored object
func (m *MinIOStore) GetObjectInfo(ctx context.Context, objectName string) (*minio.ObjectInfo, error) {
	info, err := m.client.StatObject(ctx, m.bucketName, objectName, minio.StatObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get object info: %w", err)
	}
	return &info, nil
}
