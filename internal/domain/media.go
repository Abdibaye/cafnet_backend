package domain

import (
	"time"
)

// Strongly-typed identifier for media assets.
type MediaID string

// MediaType classifies business or menu imagery.
type MediaType string

const (
	MediaTypeBusinessCover   MediaType = "BUSINESS_COVER"
	MediaTypeBusinessGallery MediaType = "BUSINESS_GALLERY"
	MediaTypeMenuItem        MediaType = "MENU_ITEM"
)

// MediaStatus tracks the upload lifecycle state machine.
type MediaStatus string

const (
	MediaStatusPending MediaStatus = "PENDING"
	MediaStatusReady   MediaStatus = "READY"
	MediaStatusFailed  MediaStatus = "FAILED"
	MediaStatusDeleted MediaStatus = "DELETED"
)

// Media represents a persistent media record stored in PostgreSQL and Tigris.
type Media struct {
	ID              MediaID       `json:"id"`
	CreatedByUserID UserID        `json:"created_by_user_id"`
	BusinessID      *BusinessID   `json:"business_id,omitempty"`
	MenuItemID      *ItemID       `json:"menu_item_id,omitempty"`
	SubmissionID    *SubmissionID `json:"submission_id,omitempty"`
	MediaType       MediaType     `json:"media_type"`
	ObjectKey       string        `json:"object_key"`
	ContentType     string        `json:"content_type"`
	FileSize        int64         `json:"file_size"`
	Status          MediaStatus   `json:"status"`
	SortOrder       int           `json:"sort_order"`
	URL             string        `json:"url"` // Derived delivery URL
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
}

// BusinessMedia is the normalized, public view model exposed across APIs.
type BusinessMedia struct {
	ID        MediaID   `json:"id"`
	URL       string    `json:"url"`
	Type      MediaType `json:"type"`
	SortOrder int       `json:"sort_order"`
}

// TargetType defines the entity destination for an impending upload.
type TargetType string

const (
	TargetTypeSubmission      TargetType = "submission"
	TargetTypeBusinessCover   TargetType = "business_cover"
	TargetTypeBusinessGallery TargetType = "business_gallery"
	TargetTypeMenuItem        TargetType = "menu_item"
)

// CreateUploadInput carries request payload for generating a presigned upload URL.
type CreateUploadInput struct {
	TargetType  TargetType `json:"target_type"`
	TargetID    string     `json:"target_id,omitempty"` // submission_id, business_id, or menu_item_id
	ContentType string     `json:"content_type"`
	FileSize    int64      `json:"file_size"`
}

// PresignedUploadResponse is returned to the browser for direct Tigris PUT upload.
type PresignedUploadResponse struct {
	MediaID   MediaID   `json:"media_id"`
	UploadURL string    `json:"upload_url"`
	ObjectKey string    `json:"object_key"`
	ExpiresAt time.Time `json:"expires_at"`
}

// CompleteUploadResponse is returned after Tigris upload verification.
type CompleteUploadResponse struct {
	Media Media `json:"media"`
}

// ReorderMediaInput specifies the new sequence of media IDs for a business gallery.
type ReorderMediaInput struct {
	MediaIDs []MediaID `json:"media_ids"`
}
