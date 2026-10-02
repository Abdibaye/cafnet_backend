package storage

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrObjectNotFound     = errors.New("storage object not found")
	ErrUnsupportedMimeType = errors.New("unsupported image mime type")
	ErrFileTooLarge        = errors.New("file exceeds maximum allowed size")
	ErrInvalidObjectKey    = errors.New("invalid storage object key")
)

// ObjectMetadata captures object storage metadata retrieved from Tigris/S3.
type ObjectMetadata struct {
	Key           string
	ContentType   string
	ContentLength int64
	ETag          string
	LastModified  time.Time
}

// ObjectStorage is the portable abstraction for S3-compatible Tigris storage.
type ObjectStorage interface {
	CreatePresignedUpload(ctx context.Context, key string, contentType string, contentLength int64, lifetime time.Duration) (string, error)
	CreatePresignedDownload(ctx context.Context, key string, lifetime time.Duration) (string, error)
	HeadObject(ctx context.Context, key string) (*ObjectMetadata, error)
	ObjectExists(ctx context.Context, key string) (bool, error)
	DeleteObject(ctx context.Context, key string) error
	CopyObject(ctx context.Context, srcKey, destKey string) error
	PublicURL(key string) string
}

// MimeToExtension safely maps supported MIME types to strict, safe file extensions.
func MimeToExtension(mime string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mime)) {
	case "image/jpeg", "image/jpg":
		return ".jpg", nil
	case "image/png":
		return ".png", nil
	case "image/webp":
		return ".webp", nil
	default:
		return "", fmt.Errorf("%w: %s (supported: image/jpeg, image/png, image/webp)", ErrUnsupportedMimeType, mime)
	}
}

// ValidateMimeType validates that the supplied MIME type is in the allowed list.
func ValidateMimeType(mime string, allowed []string) error {
	normalized := strings.ToLower(strings.TrimSpace(mime))
	for _, a := range allowed {
		if strings.ToLower(strings.TrimSpace(a)) == normalized {
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrUnsupportedMimeType, mime)
}

// ValidateFileSize ensures the uploaded size is positive and within configured bounds.
func ValidateFileSize(size int64, maxSize int64) error {
	if size <= 0 {
		return errors.New("file size must be greater than zero")
	}
	if size > maxSize {
		return fmt.Errorf("%w: size %d bytes exceeds maximum %d bytes", ErrFileTooLarge, size, maxSize)
	}
	return nil
}

// Server-generated, deterministic object key generators.
// Prevent client-supplied paths and directory traversal attacks.

// SubmissionCoverKey generates a deterministic object key for a submission image.
func SubmissionCoverKey(submissionID string, mediaID string, ext string) string {
	ext = cleanExt(ext)
	return fmt.Sprintf("submissions/%s/cover/%s%s", sanitizeSegment(submissionID), sanitizeSegment(mediaID), ext)
}

// BusinessCoverKey generates a deterministic object key for an approved business cover image.
func BusinessCoverKey(businessID int64, mediaID string, ext string) string {
	ext = cleanExt(ext)
	return fmt.Sprintf("businesses/%d/cover/%s%s", businessID, sanitizeSegment(mediaID), ext)
}

// BusinessGalleryKey generates a deterministic object key for a business gallery image.
func BusinessGalleryKey(businessID int64, mediaID string, ext string) string {
	ext = cleanExt(ext)
	return fmt.Sprintf("businesses/%d/gallery/%s%s", businessID, sanitizeSegment(mediaID), ext)
}

// MenuItemKey generates a deterministic object key for a menu item image.
func MenuItemKey(businessID int64, itemID string, mediaID string, ext string) string {
	ext = cleanExt(ext)
	return fmt.Sprintf("businesses/%d/menu-items/%s/%s%s", businessID, sanitizeSegment(itemID), sanitizeSegment(mediaID), ext)
}

func sanitizeSegment(s string) string {
	s = strings.ReplaceAll(s, "/", "")
	s = strings.ReplaceAll(s, "\\", "")
	s = strings.ReplaceAll(s, "..", "")
	return strings.TrimSpace(s)
}

func cleanExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext != "" && !strings.HasPrefix(ext, ".") {
		ext = "." + ext
	}
	return ext
}
