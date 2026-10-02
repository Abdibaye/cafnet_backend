package domain

import (
	"time"
)

// Business represents a dining venue, cafe, or restaurant listing.
type Business struct {
	ID                   BusinessID         `json:"id"`
	Name                 string             `json:"name"`
	Slug                 string             `json:"slug"`
	Category             BusinessCategory   `json:"category"`
	Cuisine              string             `json:"cuisine"`
	Description          string             `json:"description"`
	ShortDescription     string             `json:"short_description"`
	LocationName         string             `json:"location_name"`
	Neighborhood         string             `json:"neighborhood"`
	AddressLine          *string            `json:"address_line,omitempty"`
	Latitude             *float64           `json:"latitude,omitempty"`
	Longitude            *float64           `json:"longitude,omitempty"`
	Phone                string             `json:"phone"`
	Website              *string            `json:"website,omitempty"`
	PriceRange           PriceRange         `json:"price_range"`
	Tags                 []string           `json:"tags"`
	PaymentMethods       []string           `json:"payment_methods"`
	ReservationAvailable bool               `json:"reservation_available"`
	PublicationStatus    PublicationStatus  `json:"publication_status"`
	VerificationStatus   VerificationStatus `json:"verification_status"`
	ClaimStatus          ClaimStatus        `json:"claim_status"`
	IsFeatured           bool               `json:"is_featured"`
	CoverImage           *string            `json:"cover_image,omitempty"`
	GalleryCount         int                `json:"gallery_count"`
	CreatedByUserID      *UserID            `json:"created_by_user_id,omitempty"`
	CreatedAt            time.Time          `json:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at"`
	DeletedAt            *time.Time         `json:"deleted_at,omitempty"`
}

// IsDiscoverable returns true if the business is published and not soft-deleted.
func (b *Business) IsDiscoverable() bool {
	return b.PublicationStatus == PublicationStatusPublished && b.DeletedAt == nil
}

// IsVerified returns true if the business holds an official verified authenticity badge.
func (b *Business) IsVerified() bool {
	return b.VerificationStatus == VerificationStatusVerified
}

// IsClaimed returns true if the business has an approved, active manager relationship.
func (b *Business) IsClaimed() bool {
	return b.ClaimStatus == ClaimStatusClaimed
}

// BusinessHours specifies the opening/closing schedule for a single day of the week.
type BusinessHours struct {
	ID         string     `json:"id"`
	BusinessID BusinessID `json:"business_id"`
	DayOfWeek  int16      `json:"day_of_week"`         // 0=Sunday, 1=Monday, ..., 6=Saturday
	OpensAt    *string    `json:"opens_at,omitempty"`  // Format: "HH:MM:SS"
	ClosesAt   *string    `json:"closes_at,omitempty"` // Format: "HH:MM:SS"
	IsClosed   bool       `json:"is_closed"`
}

// BusinessImage represents a gallery photo for a business listing.
type BusinessImage struct {
	ID         string     `json:"id"`
	BusinessID BusinessID `json:"business_id"`
	ImageURL   string     `json:"image_url"`
	AltText    *string    `json:"alt_text,omitempty"`
	SortOrder  int        `json:"sort_order"`
	CreatedAt  time.Time  `json:"created_at"`
}

// BusinessFilter defines criteria for querying directory businesses.
type BusinessFilter struct {
	Category           *BusinessCategory
	Neighborhood       *string
	PublicationStatus  *PublicationStatus
	VerificationStatus *VerificationStatus
	ClaimStatus        *ClaimStatus
	IncludeDeleted     bool
	Limit              int
	Offset             int
}

// SearchResult wraps a business with search relevance rank.
type SearchResult struct {
	Business Business `json:"business"`
	Rank     float64  `json:"rank"`
}
