package s3

import (
	"context"
	"fmt"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const (
	initTimeout = 5 * time.Second
)

// NewMinIOClient creates a new MinIO client
func NewClient(endpoint, accessKey, secretKey string, useSSL bool) (*minio.Client, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	return client, nil
}

// EnsureBucket makes sure a bucket exists
func EnsureBucket(parentCtx context.Context, client *minio.Client, bucketName string) error {
	ctx, cancel := context.WithTimeout(parentCtx, initTimeout)
	defer cancel()

	exist, err := client.BucketExists(ctx, bucketName)
	if err != nil {
		return fmt.Errorf("failed to check whether bucket exist: %w", err)
	}

	if !exist {
		err = client.MakeBucket(ctx, bucketName, minio.MakeBucketOptions{})
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
	}

	return nil
}
