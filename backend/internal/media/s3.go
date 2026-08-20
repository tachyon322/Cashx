// Package media uploads files to an S3-compatible store (MinIO in dev).
package media

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	"cashx/internal/platform"
)

// Store wraps the MinIO/S3 client.
type Store struct {
	client *minio.Client
	bucket string
	useSSL bool
	endpoint string
}

// New creates the S3 client; bucket existence is verified lazily by EnsureBucket.
func New(cfg platform.Config) (*Store, error) {
	client, err := minio.New(cfg.S3Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.S3AccessKey, cfg.S3SecretKey, ""),
		Secure: cfg.S3UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("create s3 client: %w", err)
	}
	return &Store{client: client, bucket: cfg.S3Bucket, useSSL: cfg.S3UseSSL, endpoint: cfg.S3Endpoint}, nil
}

// EnsureBucket creates the bucket if missing.
func (s *Store) EnsureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err != nil {
		return err
	}
	if !exists {
		return s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	}
	return nil
}

// Upload stores one object and returns its key.
func (s *Store) Upload(ctx context.Context, r io.Reader, size int64, contentType string) (string, error) {
	key := "media/" + uuid.NewString()
	_, err := s.client.PutObject(ctx, s.bucket, key, r, size, minio.PutObjectOptions{ContentType: contentType})
	if err != nil {
		return "", err
	}
	return key, nil
}

// PublicURL builds the public URL for an object key.
func (s *Store) PublicURL(key string) string {
	scheme := "http"
	if s.useSSL {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/%s/%s", scheme, s.endpoint, s.bucket, key)
}

// ContentTypeFromName maps a filename to a content type for uploads.
func ContentTypeFromName(name string) string {
	name = strings.ToLower(name)
	switch {
	case strings.HasSuffix(name, ".png"):
		return "image/png"
	case strings.HasSuffix(name, ".jpg"), strings.HasSuffix(name, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(name, ".webp"):
		return "image/webp"
	case strings.HasSuffix(name, ".svg"):
		return "image/svg+xml"
	default:
		return "application/octet-stream"
	}
}

// Now is a tiny indirection so tests can pin time.
var Now = func() time.Time { return time.Now() }
