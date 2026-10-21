package domain

import (
	"time"
)

type AssistantSessionID string
type AssistantMessageID string

// AssistantSession represents an interactive food discovery chat session.
type AssistantSession struct {
	ID        AssistantSessionID `json:"id"`
	UserID    *UserID            `json:"user_id,omitempty"`
	Title     string             `json:"title"`
	CreatedAt time.Time          `json:"created_at"`
	UpdatedAt time.Time          `json:"updated_at"`
	Messages  []AssistantMessage `json:"messages,omitempty"`
}

// AssistantMessage represents a single conversational turn.
type AssistantMessage struct {
	ID                  AssistantMessageID `json:"id"`
	SessionID           AssistantSessionID `json:"session_id"`
	Role                string             `json:"role"` // "user", "assistant", "system"
	Content             string             `json:"content"`
	Intent              string             `json:"intent"` // "food_search", "clarification", "general_qa"
	SearchFilter        *FoodSearchFilter  `json:"search_filter,omitempty"`
	MatchedDishID       *DishID            `json:"matched_dish_id,omitempty"`
	ResultsCount        int                `json:"results_count"`
	FollowUpSuggestions []string           `json:"follow_up_suggestions,omitempty"`
	CreatedAt           time.Time          `json:"created_at"`
}

// AssistantChatRequest carries inbound conversation payload from user.
type AssistantChatRequest struct {
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
	Location  string `json:"location,omitempty"` // User's detected area, e.g. "Bole"
}

// AssistantChatResponse returns assistant reply, grounded real data, and suggestions.
type AssistantChatResponse struct {
	SessionID           string             `json:"session_id"`
	Message             string             `json:"message"`
	Intent              string             `json:"intent"`
	SearchFilter        *FoodSearchFilter  `json:"search_filter,omitempty"`
	CanonicalDish       *CanonicalDish     `json:"canonical_dish,omitempty"`
	Results             []FoodSearchResult `json:"results"`
	Items               []FoodSearchResult `json:"items"`
	FollowUpSuggestions []string           `json:"follow_up_suggestions"`
	FallbackUsed        bool               `json:"fallback_used"`
}
