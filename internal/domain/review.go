package domain

import (
	"time"
)

// Review represents customer ratings and feedback tied to a verified reservation visit.
type Review struct {
	ID              ReviewID      `json:"id"`
	BusinessID      BusinessID    `json:"business_id"`
	CustomerUserID  UserID        `json:"customer_user_id"`
	ReservationID   ReservationID `json:"reservation_id"`
	Rating          int16         `json:"rating"` // 1 to 5
	ReviewText      string        `json:"review_text"`
	IsVerifiedVisit bool          `json:"is_verified_visit"`
	Status          ReviewStatus  `json:"status"`
	HelpfulCount    int           `json:"helpful_count"`
	CreatedAt       time.Time     `json:"created_at"`
	UpdatedAt       time.Time     `json:"updated_at"`
	DeletedAt       *time.Time    `json:"deleted_at,omitempty"`
}

// IsVisible returns true if the review is published and not soft-deleted or removed by moderation.
func (r *Review) IsVisible() bool {
	return r.Status == ReviewStatusVisible && r.DeletedAt == nil
}

// ReviewHelpfulVote represents an upvote cast by a customer marking a review as helpful.
type ReviewHelpfulVote struct {
	ReviewID       ReviewID  `json:"review_id"`
	CustomerUserID UserID    `json:"customer_user_id"`
	CreatedAt      time.Time `json:"created_at"`
}

// ReviewSummary encapsulates aggregated rating statistics for a business.
type ReviewSummary struct {
	BusinessID   BusinessID `json:"business_id"`
	Count        int        `json:"count"`
	Average      float64    `json:"average"`      // e.g. 4.8
	Distribution [5]int     `json:"distribution"` // counts for [5-star, 4-star, 3-star, 2-star, 1-star]
}
