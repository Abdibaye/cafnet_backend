package assistant

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// RestaurantProfileBundle contains all raw relational entities needed to compile RAG documents.
type RestaurantProfileBundle struct {
	Business    domain.Business
	Hours       []domain.BusinessHours
	MenuItems   []domain.MenuItem
	Reviews     []domain.Review
	Rating      float64
	ReviewCount int
}

// CompileRestaurantDocuments transforms relational restaurant data into rich text documents for RAG.
func CompileRestaurantDocuments(bundle RestaurantProfileBundle) []domain.RestaurantDocument {
	b := bundle.Business
	area := b.Neighborhood
	if area == "" {
		area = b.LocationName
	}
	if area == "" {
		area = "Addis Ababa"
	}

	var minPriceETB, maxPriceETB float64
	firstPrice := true
	for _, item := range bundle.MenuItems {
		if !item.IsAvailable {
			continue
		}
		p := float64(item.PriceETBMinor) / 100.0
		if firstPrice {
			minPriceETB = p
			maxPriceETB = p
			firstPrice = false
		} else {
			if p < minPriceETB {
				minPriceETB = p
			}
			if p > maxPriceETB {
				maxPriceETB = p
			}
		}
	}

	commonMetadata := map[string]interface{}{
		"business_id":           b.ID,
		"name":                  b.Name,
		"category":              b.Category,
		"cuisine":               b.Cuisine,
		"neighborhood":          b.Neighborhood,
		"location_name":         b.LocationName,
		"price_range":           b.PriceRange,
		"min_price_etb":         minPriceETB,
		"max_price_etb":         maxPriceETB,
		"rating":                bundle.Rating,
		"review_count":          bundle.ReviewCount,
		"reservation_available": b.ReservationAvailable,
	}

	var docs []domain.RestaurantDocument

	// 1. Profile Document
	profileContent := buildProfileContent(b, bundle.Hours, bundle.Rating, bundle.ReviewCount, minPriceETB, maxPriceETB)
	profileDoc := domain.RestaurantDocument{
		BusinessID:  int64(b.ID),
		DocType:     "profile",
		Title:       fmt.Sprintf("%s - %s (%s, %s)", b.Name, b.Cuisine, b.Category, area),
		Content:     profileContent,
		Metadata:    commonMetadata,
		ContentHash: computeContentHash(profileContent),
	}
	docs = append(docs, profileDoc)

	// 2. Menu Document (if menu items exist)
	if len(bundle.MenuItems) > 0 {
		menuContent := buildMenuContent(b, bundle.MenuItems, area)
		menuDoc := domain.RestaurantDocument{
			BusinessID:  int64(b.ID),
			DocType:     "menu",
			Title:       fmt.Sprintf("%s Menu & Dish Offerings (%s)", b.Name, area),
			Content:     menuContent,
			Metadata:    commonMetadata,
			ContentHash: computeContentHash(menuContent),
		}
		docs = append(docs, menuDoc)
	}

	// 3. FAQs & Practical Dining Info Document
	faqContent := buildFAQContent(b, bundle.Hours, bundle.MenuItems, bundle.Rating, bundle.ReviewCount, minPriceETB, maxPriceETB, area)
	faqDoc := domain.RestaurantDocument{
		BusinessID:  int64(b.ID),
		DocType:     "faq",
		Title:       fmt.Sprintf("%s Dining FAQs & Details (%s)", b.Name, area),
		Content:     faqContent,
		Metadata:    commonMetadata,
		ContentHash: computeContentHash(faqContent),
	}
	docs = append(docs, faqDoc)

	// 4. Customer Reviews & Ratings Document
	if bundle.ReviewCount > 0 || len(bundle.Reviews) > 0 {
		reviewContent := buildReviewsContent(b, bundle.Reviews, bundle.Rating, bundle.ReviewCount, area)
		reviewDoc := domain.RestaurantDocument{
			BusinessID:  int64(b.ID),
			DocType:     "review",
			Title:       fmt.Sprintf("%s Customer Reviews & Ratings (%s)", b.Name, area),
			Content:     reviewContent,
			Metadata:    commonMetadata,
			ContentHash: computeContentHash(reviewContent),
		}
		docs = append(docs, reviewDoc)
	}

	return docs
}

func buildProfileContent(
	b domain.Business,
	hours []domain.BusinessHours,
	rating float64,
	reviewCount int,
	minPrice, maxPrice float64,
) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Restaurant: %s\n", b.Name))
	sb.WriteString(fmt.Sprintf("Category: %s\n", b.Category))
	sb.WriteString(fmt.Sprintf("Cuisine: %s\n", b.Cuisine))
	sb.WriteString(fmt.Sprintf("Neighborhood: %s\n", b.Neighborhood))
	if b.LocationName != "" && b.LocationName != b.Neighborhood {
		sb.WriteString(fmt.Sprintf("Area: %s\n", b.LocationName))
	}
	if b.AddressLine != nil && *b.AddressLine != "" {
		sb.WriteString(fmt.Sprintf("Address: %s\n", *b.AddressLine))
	}
	if b.Phone != "" {
		sb.WriteString(fmt.Sprintf("Phone: %s\n", b.Phone))
	}
	sb.WriteString(fmt.Sprintf("Price Level: %s", b.PriceRange))
	if maxPrice > 0 {
		sb.WriteString(fmt.Sprintf(" (Typical dish prices: %.0f ETB to %.0f ETB)", minPrice, maxPrice))
	}
	sb.WriteString("\n")

	if rating > 0 {
		sb.WriteString(fmt.Sprintf("Customer Rating: %.1f stars (%d verified reviews)\n", rating, reviewCount))
	}

	if len(b.Tags) > 0 {
		sb.WriteString(fmt.Sprintf("Tags & Highlights: %s\n", strings.Join(b.Tags, ", ")))
	}

	if len(b.PaymentMethods) > 0 {
		sb.WriteString(fmt.Sprintf("Payment Methods: %s\n", strings.Join(b.PaymentMethods, ", ")))
	}

	if b.ReservationAvailable {
		sb.WriteString("Table Reservations: Available on Cafnet\n")
	} else {
		sb.WriteString("Table Reservations: Walk-in only\n")
	}

	if b.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", b.Description))
	}

	if len(hours) > 0 {
		sb.WriteString("Operating Hours:\n")
		for _, h := range hours {
			sb.WriteString(fmt.Sprintf("  - %s: %s\n", formatDayOfWeek(int(h.DayOfWeek)), formatSchedule(h)))
		}
	}

	return strings.TrimSpace(sb.String())
}

func buildMenuContent(b domain.Business, items []domain.MenuItem, area string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Menu and Dish Offerings at %s (%s, %s):\n", b.Name, b.Cuisine, area))
	for _, item := range items {
		if !item.IsAvailable {
			continue
		}
		priceETB := float64(item.PriceETBMinor) / 100.0
		sb.WriteString(fmt.Sprintf("- %s: %.0f ETB", item.Name, priceETB))
		if item.Description != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", item.Description))
		}
		sb.WriteString("\n")
	}
	return strings.TrimSpace(sb.String())
}

func buildFAQContent(
	b domain.Business,
	hours []domain.BusinessHours,
	items []domain.MenuItem,
	rating float64,
	reviewCount int,
	minPrice, maxPrice float64,
	area string,
) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Frequently Asked Questions about %s:\n\n", b.Name))

	sb.WriteString(fmt.Sprintf("Q: What kind of food does %s serve?\n", b.Name))
	sb.WriteString(fmt.Sprintf("A: %s serves %s cuisine (%s).", b.Name, b.Cuisine, b.Category))
	if b.ShortDescription != "" {
		sb.WriteString(fmt.Sprintf(" %s", b.ShortDescription))
	}
	sb.WriteString("\n\n")

	sb.WriteString(fmt.Sprintf("Q: Where is %s located in Addis Ababa?\n", b.Name))
	sb.WriteString(fmt.Sprintf("A: %s is located in %s", b.Name, area))
	if b.AddressLine != nil && *b.AddressLine != "" {
		sb.WriteString(fmt.Sprintf(" at %s", *b.AddressLine))
	}
	sb.WriteString(".\n\n")

	sb.WriteString(fmt.Sprintf("Q: How much does it cost to eat at %s?\n", b.Name))
	if maxPrice > 0 {
		sb.WriteString(fmt.Sprintf("A: It is rated as %s price level. Menu items range from %.0f ETB to %.0f ETB.\n\n", b.PriceRange, minPrice, maxPrice))
	} else {
		sb.WriteString(fmt.Sprintf("A: It is classified in the %s price category.\n\n", b.PriceRange))
	}

	sb.WriteString(fmt.Sprintf("Q: Can I book or reserve a table at %s?\n", b.Name))
	if b.ReservationAvailable {
		sb.WriteString("A: Yes, table reservations can be booked directly through Cafnet.\n\n")
	} else {
		sb.WriteString("A: No advance table reservations are listed. Walk-ins are welcomed.\n\n")
	}

	sb.WriteString(fmt.Sprintf("Q: What payment options are accepted at %s?\n", b.Name))
	if len(b.PaymentMethods) > 0 {
		sb.WriteString(fmt.Sprintf("A: Accepted payment methods include %s.\n\n", strings.Join(b.PaymentMethods, ", ")))
	} else {
		sb.WriteString("A: Cash and standard local payment methods (such as Telebirr) are commonly used.\n\n")
	}

	if len(hours) > 0 {
		sb.WriteString(fmt.Sprintf("Q: When is %s open?\n", b.Name))
		var schedules []string
		for _, h := range hours {
			schedules = append(schedules, fmt.Sprintf("%s: %s", formatDayOfWeek(int(h.DayOfWeek)), formatSchedule(h)))
		}
		sb.WriteString(fmt.Sprintf("A: %s.\n\n", strings.Join(schedules, "; ")))
	}

	return strings.TrimSpace(sb.String())
}

func buildReviewsContent(
	b domain.Business,
	reviews []domain.Review,
	rating float64,
	reviewCount int,
	area string,
) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Customer Reviews and Ratings for %s (%s, %s):\n", b.Name, b.Cuisine, area))
	sb.WriteString(fmt.Sprintf("Overall Score: %.1f / 5.0 stars based on %d verified customer reviews.\n\n", rating, reviewCount))

	if len(reviews) > 0 {
		sb.WriteString("Verified Diner Feedback:\n")
		for _, r := range reviews {
			text := strings.TrimSpace(r.ReviewText)
			if text != "" {
				sb.WriteString(fmt.Sprintf("- Rated %d/5 stars: \"%s\"\n", r.Rating, text))
			} else {
				sb.WriteString(fmt.Sprintf("- Rated %d/5 stars by a verified diner\n", r.Rating))
			}
		}
	} else if rating > 0 {
		sb.WriteString(fmt.Sprintf("Diners on Cafnet have consistently rated %s with an average score of %.1f stars across %d visits.\n", b.Name, rating, reviewCount))
	}

	return strings.TrimSpace(sb.String())
}

func computeContentHash(content string) string {
	hasher := sha256.New()
	hasher.Write([]byte(content))
	return hex.EncodeToString(hasher.Sum(nil))
}

func formatDayOfWeek(dow int) string {
	days := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
	if dow >= 0 && dow < len(days) {
		return days[dow]
	}
	return "Day"
}

func formatSchedule(h domain.BusinessHours) string {
	if h.IsClosed {
		return "Closed"
	}
	if h.OpensAt != nil && h.ClosesAt != nil {
		return fmt.Sprintf("%s - %s", *h.OpensAt, *h.ClosesAt)
	}
	return "Open"
}
