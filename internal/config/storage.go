package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// StorageConfig encapsulates runtime parameters for S3-compatible Tigris object storage.
type StorageConfig struct {
	Endpoint               string        `json:"endpoint"`
	Region                 string        `json:"region"`
	Bucket                 string        `json:"bucket"`
	AccessKey              string        `json:"-"` // Never expose in JSON/logs
	SecretKey              string        `json:"-"` // Never expose in JSON/logs
	PublicURL              string        `json:"public_url"`
	PresignedURLExpiration time.Duration `json:"presigned_url_expiration"`
	MaxUploadSizeBytes     int64         `json:"max_upload_size_bytes"`
	AllowedMimeTypes       []string      `json:"allowed_mime_types"`
}

// LoadStorageConfig reads storage configuration from environment variables with safe production defaults.
func LoadStorageConfig() StorageConfig {
	endpoint := os.Getenv("TIGRIS_ENDPOINT")
	if endpoint == "" {
		endpoint = os.Getenv("AWS_ENDPOINT_URL_S3")
	}
	if endpoint == "" {
		endpoint = "https://fly.storage.tigris.dev"
	}
	endpoint = strings.TrimRight(endpoint, "/")

	region := os.Getenv("TIGRIS_REGION")
	if region == "" {
		region = os.Getenv("AWS_REGION")
	}
	if region == "" {
		region = "auto"
	}

	bucket := os.Getenv("TIGRIS_BUCKET")
	if bucket == "" {
		bucket = os.Getenv("AWS_S3_BUCKET")
	}
	if bucket == "" {
		bucket = "cafnet-media"
	}

	accessKey := os.Getenv("TIGRIS_ACCESS_KEY")
	if accessKey == "" {
		accessKey = os.Getenv("AWS_ACCESS_KEY_ID")
	}

	secretKey := os.Getenv("TIGRIS_SECRET_KEY")
	if secretKey == "" {
		secretKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}

	publicURL := os.Getenv("TIGRIS_PUBLIC_URL")
	if publicURL == "" {
		publicURL = os.Getenv("MEDIA_PUBLIC_URL")
	}
	if publicURL == "" {
		// Default public domain for Tigris buckets
		if strings.Contains(endpoint, "fly.storage.tigris.dev") {
			publicURL = fmt.Sprintf("https://%s.fly.storage.tigris.dev", bucket)
		} else {
			publicURL = fmt.Sprintf("%s/%s", endpoint, bucket)
		}
	}
	publicURL = strings.TrimRight(publicURL, "/")

	presignedExpiration := parseDurationEnv("STORAGE_PRESIGNED_EXPIRATION", 10*time.Minute)
	maxUploadSize := int64(parseIntEnv("STORAGE_MAX_UPLOAD_SIZE_BYTES", 10*1024*1024)) // 10 MB

	return StorageConfig{
		Endpoint:               endpoint,
		Region:                 region,
		Bucket:                 bucket,
		AccessKey:              accessKey,
		SecretKey:              secretKey,
		PublicURL:              publicURL,
		PresignedURLExpiration: presignedExpiration,
		MaxUploadSizeBytes:     maxUploadSize,
		AllowedMimeTypes:       []string{"image/jpeg", "image/png", "image/webp"},
	}
}

// IsConfigured returns true if real S3 credentials are provided.
func (c *StorageConfig) IsConfigured() bool {
	return c.AccessKey != "" && c.SecretKey != ""
}

// Validate checks the storage configuration.
func (c *StorageConfig) Validate() error {
	if c.Bucket == "" {
		return fmt.Errorf("storage bucket cannot be empty")
	}
	if c.PresignedURLExpiration < 1*time.Minute {
		return fmt.Errorf("STORAGE_PRESIGNED_EXPIRATION must be >= 1m")
	}
	if c.MaxUploadSizeBytes < 1024 {
		return fmt.Errorf("STORAGE_MAX_UPLOAD_SIZE_BYTES must be >= 1024 bytes")
	}
	return nil
}
