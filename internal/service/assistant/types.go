package assistant

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// InterpretedQuery represents structured search parameters and conversational metadata
// extracted from natural language by the AI or fallback engine.
type InterpretedQuery struct {
	DishOrFood          string   `json:"dish_or_food"`
	MaxPriceETB         *float64 `json:"max_price_etb,omitempty"`
	MinPriceETB         *float64 `json:"min_price_etb,omitempty"`
	MinRating           *float64 `json:"min_rating,omitempty"`
	Neighborhood        string   `json:"neighborhood,omitempty"`
	Intent              string   `json:"intent"` // "food_search", "clarification", "general_qa"
	ReplyText           string   `json:"reply_text"`
	FollowUpSuggestions []string `json:"follow_up_suggestions"`
	FallbackUsed        bool     `json:"fallback_used"`
}

// AIParser defines the contract for interpreting conversational food requests.
type AIParser interface {
	Interpret(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error)
}
