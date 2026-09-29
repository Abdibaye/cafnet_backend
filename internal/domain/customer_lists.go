package domain

import (
	"time"
)

// SavedBusiness represents a customer bookmark for a venue.
type SavedBusiness struct {
	UserID     UserID     `json:"user_id"`
	BusinessID BusinessID `json:"business_id"`
	CreatedAt  time.Time  `json:"created_at"`
}

// FollowedBusiness represents a customer subscription to venue updates.
type FollowedBusiness struct {
	UserID     UserID     `json:"user_id"`
	BusinessID BusinessID `json:"business_id"`
	CreatedAt  time.Time  `json:"created_at"`
}
