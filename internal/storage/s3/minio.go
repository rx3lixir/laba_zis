package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// MinIOClient wraps the MinIO client for voice message storage
type MinIOClient struct {
	client     *minio.Client
	bucketName string
}

// NewMinIOClient creates a new MinIO client and ensures bucket exists
func NewMinIOClient(endpoint, accessKey, secretKey, bucketName string, useSSL bool) (*MinIOClient, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	mc := &MinIOClient{
		client:     client,
		bucketName: bucketName,
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	if err := mc.ensureBucket(ctx); err != nil {
		return nil, err
	}

	return mc, nil
}

// ensureBucket creates the bucket if it doesn't exist
func (m *MinIOClient) ensureBucket(ctx context.Context) error {
	exist, err := m.client.BucketExists(ctx, m.bucketName)
	if err != nil {
		return fmt.Errorf("failed to check whether bucket exist: %w", err)
	}

	if !exist {
		err = m.client.MakeBucket(ctx, m.bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}
