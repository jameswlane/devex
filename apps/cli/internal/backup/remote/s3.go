package remote

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

// S3Config selects an S3-compatible bucket (AWS S3, Backblaze B2, Cloudflare
// R2, MinIO, ...). Credentials come from the environment, never from
// configuration files.
type S3Config struct {
	Endpoint  string
	Bucket    string
	Region    string
	Insecure  bool
	AccessKey string
	SecretKey string
}

// S3 is an S3-compatible Store.
type S3 struct {
	client *minio.Client
	bucket string
}

// NewS3 connects to an S3-compatible endpoint.
func NewS3(config S3Config) (*S3, error) {
	if strings.TrimSpace(config.Endpoint) == "" || strings.TrimSpace(config.Bucket) == "" {
		return nil, errors.New("backup.remote.endpoint and backup.remote.bucket are required")
	}
	if strings.Contains(config.Endpoint, "://") {
		return nil, fmt.Errorf("backup.remote.endpoint is a host name such as s3.amazonaws.com, not a URL; set backup.remote.insecure for plain HTTP")
	}
	if config.AccessKey == "" || config.SecretKey == "" {
		return nil, errors.New("set DEVEX_BACKUP_ACCESS_KEY_ID and DEVEX_BACKUP_SECRET_ACCESS_KEY (or AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY)")
	}
	client, err := minio.New(config.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(config.AccessKey, config.SecretKey, ""),
		Secure: !config.Insecure,
		Region: config.Region,
	})
	if err != nil {
		return nil, fmt.Errorf("configure remote storage: %w", err)
	}
	return &S3{client: client, bucket: config.Bucket}, nil
}

func (s *S3) Put(ctx context.Context, key string, data []byte) error {
	_, err := s.client.PutObject(ctx, s.bucket, key, bytes.NewReader(data), int64(len(data)), minio.PutObjectOptions{ContentType: "application/octet-stream"})
	return err
}

func (s *S3) Get(ctx context.Context, key string) ([]byte, error) {
	object, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	defer func() { _ = object.Close() }()
	data, err := io.ReadAll(io.LimitReader(object, int64(MaxObjectBytes)+1))
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if len(data) > MaxObjectBytes {
		return nil, fmt.Errorf("remote object %s exceeds %d bytes", key, MaxObjectBytes)
	}
	return data, nil
}

func (s *S3) List(ctx context.Context, prefix string) ([]Object, error) {
	var objects []Object
	for info := range s.client.ListObjects(ctx, s.bucket, minio.ListObjectsOptions{Prefix: prefix, Recursive: true}) {
		if info.Err != nil {
			return nil, info.Err
		}
		objects = append(objects, Object{Key: info.Key, Size: info.Size})
	}
	return objects, nil
}
