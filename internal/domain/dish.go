package domain

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

// CanonicalDish represents a Cafnet-controlled normalized food concept.
type CanonicalDish struct {
	ID          DishID      `json:"id"`
	Name        string      `json:"name"`
	Slug        string      `json:"slug"`
	Description string      `json:"description"`
	Cuisine     string      `json:"cuisine,omitempty"`
	Aliases     []DishAlias `json:"aliases,omitempty"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	DeletedAt   *time.Time  `json:"deleted_at,omitempty"`
}

// DishAlias represents an alternative name or spelling that resolves to a canonical dish.
type DishAlias struct {
	ID              AliasID   `json:"id"`
	CanonicalDishID DishID    `json:"canonical_dish_id"`
	Alias           string    `json:"alias"`
	NormalizedAlias string    `json:"normalized_alias"`
	Language        *string   `json:"language,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

// FoodSearchResult represents a restaurant offering a matched menu item.
type FoodSearchResult struct {
	Business      Business       `json:"business"`
	MenuItem      MenuItem       `json:"menu_item"`
	CanonicalDish *CanonicalDish `json:"canonical_dish,omitempty"`
	Rating        float64        `json:"rating"`
	ReviewCount   int            `json:"review_count"`
	Rank          float64        `json:"rank"`
}

// FoodSearchFilter defines multi-criteria filtering for menu offerings.
type FoodSearchFilter struct {
	Query            string   `json:"query"`
	MaxPriceETBMinor *int64   `json:"max_price_etb_minor,omitempty"` // in santims (1 ETB = 100 santims)
	MinPriceETBMinor *int64   `json:"min_price_etb_minor,omitempty"`
	MinRating        *float64 `json:"min_rating,omitempty"`
	Neighborhood     *string  `json:"neighborhood,omitempty"` // matches neighborhood or location_name
	Limit            int      `json:"limit"`
	Offset           int      `json:"offset"`
}

// FoodSearchResponse wraps food discovery search results with any matched canonical dish taxonomy.
type FoodSearchResponse struct {
	CanonicalDish *CanonicalDish     `json:"canonical_dish,omitempty"`
	Results       []FoodSearchResult `json:"results"`
	Items         []FoodSearchResult `json:"items,omitempty"`
}

var multiSpaceRegex = regexp.MustCompile(`\s+`)

// NormalizeAlias applies deterministic normalization:
// - Unicode lowercase
// - Trim leading/trailing whitespace and punctuation
// - Collapse internal consecutive whitespace to a single space
// - Preserves non-Latin scripts (e.g. Ethiopic/Amharic: ጥብስ)
func NormalizeAlias(alias string) string {
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return ""
	}

	lower := strings.ToLower(trimmed)
	collapsed := multiSpaceRegex.ReplaceAllString(lower, " ")

	// Trim leading and trailing punctuation (e.g. quotes, dashes, dots)
	start := 0
	end := len(collapsed)

	runes := []rune(collapsed)
	firstNonPunct := 0
	for firstNonPunct < len(runes) && (unicode.IsPunct(runes[firstNonPunct]) || unicode.IsSpace(runes[firstNonPunct])) {
		firstNonPunct++
	}

	lastNonPunct := len(runes)
	for lastNonPunct > firstNonPunct && (unicode.IsPunct(runes[lastNonPunct-1]) || unicode.IsSpace(runes[lastNonPunct-1])) {
		lastNonPunct--
	}

	if firstNonPunct >= lastNonPunct {
		return ""
	}

	result := string(runes[firstNonPunct:lastNonPunct])
	_ = start
	_ = end
	return result
}
