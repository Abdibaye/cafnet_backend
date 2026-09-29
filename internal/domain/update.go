package domain

import (
	"time"
)

// BusinessUpdate represents a time-sensitive broadcast published by an authorized manager.
type BusinessUpdate struct {
	ID           UpdateID     `json:"id"`
	BusinessID   BusinessID   `json:"business_id"`
	AuthorUserID UserID       `json:"author_user_id"`
	UpdateType   UpdateType   `json:"update_type"`
	Title        string       `json:"title"`
	Description  string       `json:"description"`
	Status       UpdateStatus `json:"status"`
	EventDate    *string      `json:"event_date,omitempty"`
	EventTime    *string      `json:"event_time,omitempty"`
	PriceText    *string      `json:"price_text,omitempty"`
	OfferDetails *string      `json:"offer_details,omitempty"`
	ValidUntil   *string      `json:"valid_until,omitempty"`
	ExpiresAt    *time.Time   `json:"expires_at,omitempty"`
	PublishedAt  time.Time    `json:"published_at"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
	DeletedAt    *time.Time   `json:"deleted_at,omitempty"`
}

// IsActive returns true if the update is published, not soft-deleted, and has not elapsed.
func (u *BusinessUpdate) IsActive() bool {
	if u.Status != UpdateStatusPublished || u.DeletedAt != nil {
		return false
	}
	if u.ExpiresAt != nil && time.Now().UTC().After(*u.ExpiresAt) {
		return false
	}
	return true
}
