package storage

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/Abdibaye/cafnet_backend/internal/config"
)

// TigrisStorage implements ObjectStorage for Tigris Object Storage via the standard AWS S3 SDK.
type TigrisStorage struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicURL     string
}

// NewTigrisStorage initializes a configured S3 client targeting Tigris.
func NewTigrisStorage(cfg config.StorageConfig) (*TigrisStorage, error) {
	customResolver := aws.EndpointResolverWithOptionsFunc(
		func(service, region string, options ...interface{}) (aws.Endpoint, error) {
			return aws.Endpoint{
				URL:               cfg.Endpoint,
				SigningRegion:     cfg.Region,
				HostnameImmutable: true,
			}, nil
		},
	)

	awsCfg, err := awsconfig.LoadDefaultConfig(
		context.Background(),
		awsconfig.WithRegion(cfg.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, "")),
		awsconfig.WithEndpointResolverWithOptions(customResolver),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration for Tigris: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		if cfg.Endpoint != "" {
			o.BaseEndpoint = aws.String(cfg.Endpoint)
		}
	})

	presignClient := s3.NewPresignClient(client)

	return &TigrisStorage{
		client:        client,
		presignClient: presignClient,
		bucket:        cfg.Bucket,
		publicURL:     cfg.PublicURL,
	}, nil
}

// CreatePresignedUpload generates a secure, short-lived presigned PUT URL for direct browser uploads.
func (s *TigrisStorage) CreatePresignedUpload(ctx context.Context, key string, contentType string, contentLength int64, lifetime time.Duration) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	}
	if contentLength > 0 {
		input.ContentLength = aws.Int64(contentLength)
	}

	req, err := s.presignClient.PresignPutObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = lifetime
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned upload URL: %w", err)
	}

	return req.URL, nil
}

// CreatePresignedDownload generates a temporary signed GET URL if needed.
func (s *TigrisStorage) CreatePresignedDownload(ctx context.Context, key string, lifetime time.Duration) (string, error) {
	input := &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	}

	req, err := s.presignClient.PresignGetObject(ctx, input, func(opts *s3.PresignOptions) {
		opts.Expires = lifetime
	})
	if err != nil {
		return "", fmt.Errorf("failed to generate presigned download URL: %w", err)
	}

	return req.URL, nil
}

// HeadObject retrieves metadata for an existing object in Tigris.
func (s *TigrisStorage) HeadObject(ctx context.Context, key string) (*ObjectMetadata, error) {
	resp, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var notFound *s3types.NotFound
		if errors.As(err, &notFound) || strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "NotFound") {
			return nil, ErrObjectNotFound
		}
		return nil, fmt.Errorf("head object failed: %w", err)
	}

	var contentType string
	if resp.ContentType != nil {
		contentType = *resp.ContentType
	}
	var contentLength int64
	if resp.ContentLength != nil {
		contentLength = *resp.ContentLength
	}
	var etag string
	if resp.ETag != nil {
		etag = *resp.ETag
	}
	var lastModified time.Time
	if resp.LastModified != nil {
		lastModified = *resp.LastModified
	}

	return &ObjectMetadata{
		Key:           key,
		ContentType:   contentType,
		ContentLength: contentLength,
		ETag:          etag,
		LastModified:  lastModified,
	}, nil
}

// ObjectExists checks whether an object exists in the Tigris bucket.
func (s *TigrisStorage) ObjectExists(ctx context.Context, key string) (bool, error) {
	_, err := s.HeadObject(ctx, key)
	if err != nil {
		if errors.Is(err, ErrObjectNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// DeleteObject deletes an object from Tigris.
func (s *TigrisStorage) DeleteObject(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("failed to delete object from storage: %w", err)
	}
	return nil
}

// CopyObject copies an object within Tigris (e.g. from submissions to businesses).
func (s *TigrisStorage) CopyObject(ctx context.Context, srcKey, destKey string) error {
	copySource := url.PathEscape(fmt.Sprintf("%s/%s", s.bucket, srcKey))
	_, err := s.client.CopyObject(ctx, &s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(copySource),
		Key:        aws.String(destKey),
	})
	if err != nil {
		return fmt.Errorf("failed to copy object in storage: %w", err)
	}
	return nil
}

// PublicURL returns the stable, customer-facing delivery URL for an object key.
func (s *TigrisStorage) PublicURL(key string) string {
	cleanKey := strings.TrimLeft(key, "/")
	if s.publicURL != "" {
		return fmt.Sprintf("%s/%s", s.publicURL, cleanKey)
	}
	return fmt.Sprintf("https://%s.fly.storage.tigris.dev/%s", s.bucket, cleanKey)
}
