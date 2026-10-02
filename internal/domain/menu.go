package domain

import (
	"fmt"
	"time"
)

// MenuCategory represents an owner-ordered grouping header for menu items.
type MenuCategory struct {
	ID         CategoryID `json:"id"`
	BusinessID BusinessID `json:"business_id"`
	Name       string     `json:"name"`
	SortOrder  int        `json:"sort_order"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
	DeletedAt  *time.Time `json:"deleted_at,omitempty"`
}

// MenuItem represents an individual food or beverage offering with exact minor-unit pricing.
type MenuItem struct {
	ID            ItemID     `json:"id"`
	CategoryID    CategoryID `json:"category_id"`
	BusinessID    BusinessID `json:"business_id"`
	Name          string     `json:"name"`
	Description   string     `json:"description"`
	PriceETBMinor int64      `json:"price_etb_minor"` // ETB Santims (1 ETB = 100 Santims)
	SortOrder     int        `json:"sort_order"`
	IsAvailable   bool       `json:"is_available"`
	Image         *string    `json:"image,omitempty"`
	ImageMediaID  *MediaID   `json:"image_media_id,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	DeletedAt     *time.Time `json:"deleted_at,omitempty"`
}

// PriceFormatted returns human-readable ETB notation for display.
func (i *MenuItem) PriceFormatted() string {
	if i.PriceETBMinor%100 == 0 {
		return fmt.Sprintf("%d ETB", i.PriceETBMinor/100)
	}
	return fmt.Sprintf("%.2f ETB", float64(i.PriceETBMinor)/100.0)
}

// CategoryWithItems combines a category with its contained menu items.
type CategoryWithItems struct {
	Category MenuCategory `json:"category"`
	Items    []MenuItem   `json:"items"`
}

// BusinessMenu aggregates all categories and items for a venue.
type BusinessMenu struct {
	BusinessID BusinessID          `json:"business_id"`
	Categories []CategoryWithItems `json:"categories"`
}
