package storage

import (
	"context"
	"fmt"
	"io"
	"log"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinioStorage struct {
	client    *minio.Client
	bucket    string
	publicURL string
}

func NewMinioStorage(endpoint, accessKey, secretKey, bucket, publicURL string, useSSL bool) (*MinioStorage, error) {
	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure: useSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create minio client: %w", err)
	}

	s := &MinioStorage{
		client:    client,
		bucket:    bucket,
		publicURL: publicURL,
	}

	if err := s.ensureBucket(context.Background()); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *MinioStorage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("failed to check bucket: %w", err)
	}
	if !exists {
		if err := s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}

		// Set bucket policy to allow public read
		policy := fmt.Sprintf(`{
			"Version": "2012-10-17",
			"Statement": [{
				"Effect": "Allow",
				"Principal": {"AWS": ["*"]},
				"Action": ["s3:GetObject"],
				"Resource": ["arn:aws:s3:::%s/*"]
			}]
		}`, s.bucket)

		if err := s.client.SetBucketPolicy(ctx, s.bucket, policy); err != nil {
			log.Printf("WARNING: failed to set bucket policy: %v", err)
		}

		log.Printf("Bucket '%s' created", s.bucket)
	}
	return nil
}

// Upload stores a file in MinIO and returns the object key.
func (s *MinioStorage) Upload(ctx context.Context, key string, reader io.Reader, size int64, contentType string) (string, error) {
	_, err := s.client.PutObject(ctx, s.bucket, key, reader, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload: %w", err)
	}

	return s.FileURL(key), nil
}

// Delete removes a file from MinIO.
func (s *MinioStorage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

// GetPresignedURL returns a temporary download URL.
func (s *MinioStorage) GetPresignedURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
	url, err := s.client.PresignedGetObject(ctx, s.bucket, key, expiry, nil)
	if err != nil {
		return "", err
	}
	return url.String(), nil
}

// FileURL returns the public URL for a file.
func (s *MinioStorage) FileURL(key string) string {
	return fmt.Sprintf("%s/%s/%s", s.publicURL, s.bucket, key)
}
