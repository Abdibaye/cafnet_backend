package domain

import (
	"time"
)

// RestaurantDocument represents a vectorized document chunk for RAG.
type RestaurantDocument struct {
	ID          string                 `json:"id"`
	BusinessID  int64                  `json:"business_id"`
	DocType     string                 `json:"doc_type"` // "profile", "menu", "faq", "reviews"
	Title       string                 `json:"title"`
	Content     string                 `json:"content"`
	Metadata    map[string]interface{} `json:"metadata"`
	Embedding   []float32              `json:"embedding,omitempty"`
	ContentHash string                 `json:"content_hash"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
}

// RetrievedDocument represents a matched document with its vector similarity score.
type RetrievedDocument struct {
	Document   RestaurantDocument `json:"document"`
	Similarity float64            `json:"similarity"`
}

// DocumentSearchFilter allows hybrid filtering on retrieved document metadata.
type DocumentSearchFilter struct {
	Query        string   `json:"query"`
	Neighborhood *string  `json:"neighborhood,omitempty"`
	MaxPriceETB  *float64 `json:"max_price_etb,omitempty"`
	MinRating    *float64 `json:"min_rating,omitempty"`
	DocType      *string  `json:"doc_type,omitempty"`
	Limit        int      `json:"limit"`
}
