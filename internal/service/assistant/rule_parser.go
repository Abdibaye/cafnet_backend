package assistant

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type ruleParser struct{}

// NewRuleParser returns a deterministic fallback parser that extracts search criteria
// from English and Amharic natural language food queries.
func NewRuleParser() AIParser {
	return &ruleParser{}
}

var (
	priceUnderRegex = regexp.MustCompile(`(?i)(?:under|below|less\s+than|max|up\s+to|within|budget\s+of|budget)\s*(?:etb|birr|ብር)?\s*(\d+(?:\.\d+)?)`)
	priceWithUnitRegex = regexp.MustCompile(`(?i)(\d+(?:\.\d+)?)\s*(?:etb|birr|santim|ብር)`)
	priceAmharicUnderRegex = regexp.MustCompile(`(?i)(?:ከ\s*)?(\d+(?:\.\d+)?)\s*(?:ብር)?\s*በታች`)
	priceOverRegex = regexp.MustCompile(`(?i)(?:over|above|more\s+than|min|at\s+least)\s*(?:etb|birr|ብር)?\s*(\d+(?:\.\d+)?)`)
	ratingRegex = regexp.MustCompile(`(?i)\b(best|top|top-rated|recommended|highest\s+rated|popular|great|ምርጥ|ጥሩ)\b`)
)

type dishMatch struct {
	canonical string
	aliases   []string
}

var knownDishes = []dishMatch{
	{canonical: "Tibs", aliases: []string{"tibs", "tibse", "tibbs", "tibis", "siga tibs", "beef tibs", "shekla tibs", "ጥብስ"}},
	{canonical: "Kitfo", aliases: []string{"kitfo", "ketfo", "kitfow", "special kitfo", "ክትፎ"}},
	{canonical: "Shiro", aliases: []string{"shiro", "shuro", "shiro tegabino", "tegabino", "shiro wot", "ሺሮ", "ሽሮ"}},
	{canonical: "Doro Wot", aliases: []string{"doro wot", "doro wat", "doro wet", "chicken wot", "ዶሮ ወጥ", "ዶሮ"}},
	{canonical: "Firfir", aliases: []string{"firfir", "fitfit", "fir fir", "fit fit", "ፍርፍር"}},
	{canonical: "Chechebsa", aliases: []string{"chechebsa", "kita firfir", "ጨጨብሳ"}},
	{canonical: "Beyaynetu", aliases: []string{"beyaynetu", "fasting platter", "yetsom beyaynetu", "veggie combo", "በያይነቱ", "የጾም"}},
	{canonical: "Gomen", aliases: []string{"gomen", "collard greens", "ye'gomen wot", "ጎመን"}},
	{canonical: "Misir", aliases: []string{"misir", "misir wot", "red lentil", "ምስር"}},
	{canonical: "Alicha Wot", aliases: []string{"alicha", "alicha wot", "አልጫ"}},
	{canonical: "Kik Wot", aliases: []string{"kik wot", "kik", "ክክ"}},
	{canonical: "Dulet", aliases: []string{"dulet", "ዱለት"}},
	{canonical: "Bozena Shiro", aliases: []string{"bozena shiro", "bozena", "ቦዘና"}},
	{canonical: "Fuul", aliases: []string{"fuul", "foul", "foul mudammas", "ፉል"}},
	{canonical: "Burger", aliases: []string{"burger", "cheeseburger", "beef burger", "hamburger", "በርገር"}},
	{canonical: "Pizza", aliases: []string{"pizza", "pizzas", "ፒዛ"}},
	{canonical: "Pasta", aliases: []string{"pasta", "spaghetti", "macaroni", "ፓስታ"}},
	{canonical: "Sandwich", aliases: []string{"sandwich", "sandwiches", "club sandwich", "ሳንድዊች"}},
	{canonical: "Fried Chicken", aliases: []string{"fried chicken", "crispy chicken", "broasted chicken"}},
	{canonical: "Coffee", aliases: []string{"coffee", "macchiato", "buna", "ቡና", "ማኪያቶ"}},
}

var knownNeighborhoods = []struct {
	canonical string
	patterns  []string
}{
	{canonical: "Bole", patterns: []string{"bole", "ቦሌ", "atlas", "medhanialem", "edna mall"}},
	{canonical: "Kazanchis", patterns: []string{"kazanchis", "ካዛንቺስ", "intercontinental"}},
	{canonical: "Piazza", patterns: []string{"piazza", "piassa", "ፒያሳ"}},
	{canonical: "Sarbet", patterns: []string{"sarbet", "sar bet", "ሳርቤት", "ሳር ቤት"}},
	{canonical: "Megenagna", patterns: []string{"megenagna", "መገናኛ"}},
	{canonical: "4 Kilo", patterns: []string{"4 kilo", "arat kilo", "አራት ኪሎ"}},
	{canonical: "6 Kilo", patterns: []string{"6 kilo", "sidist kilo", "ስድስት ኪሎ"}},
	{canonical: "CMC", patterns: []string{"cmc", "ሲኤምሲ"}},
	{canonical: "Gerji", patterns: []string{"gerji", "ገርጂ"}},
	{canonical: "Old Airport", patterns: []string{"old airport", "ኦልድ ኤርፖርት"}},
	{canonical: "Hayahulet", patterns: []string{"hayahulet", "haya hulet", "22", "ሀያ ሁለት"}},
	{canonical: "Mexico", patterns: []string{"mexico", "ሜክሲኮ"}},
	{canonical: "Lebu", patterns: []string{"lebu", "ለቡ"}},
	{canonical: "Summit", patterns: []string{"summit", "ሰሚት"}},
	{canonical: "Ayat", patterns: []string{"ayat", "አያት"}},
	{canonical: "Tor Hailoch", patterns: []string{"tor hailoch", "ጦር ኃይሎች"}},
	{canonical: "Gotera", patterns: []string{"gotera", "ጎተራ"}},
	{canonical: "Meskel Flower", patterns: []string{"meskel flower", "መስቀል ፍላወር"}},
	{canonical: "Olympia", patterns: []string{"olympia", "ኦሊምፒያ"}},
}

func (p *ruleParser) Interpret(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error) {
	lowerMsg := strings.ToLower(userMessage)

	// 1. Extract Price bounds
	var maxPrice *float64
	var minPrice *float64

	if m := priceUnderRegex.FindStringSubmatch(lowerMsg); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
			maxPrice = &val
		}
	} else if m := priceAmharicUnderRegex.FindStringSubmatch(userMessage); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
			maxPrice = &val
		}
	} else if m := priceWithUnitRegex.FindStringSubmatch(lowerMsg); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
			// If message says "under/around" or simple "300 birr", treat as target max
			maxPrice = &val
		}
	} else if strings.Contains(lowerMsg, "cheap") || strings.Contains(lowerMsg, "budget") || strings.Contains(lowerMsg, "affordable") || strings.Contains(lowerMsg, "ርካሽ") {
		v := 250.0
		maxPrice = &v
	}

	if m := priceOverRegex.FindStringSubmatch(lowerMsg); len(m) > 1 {
		if val, err := strconv.ParseFloat(m[1], 64); err == nil && val > 0 {
			minPrice = &val
		}
	}

	// 2. Extract Neighborhood / Area
	var detectedArea string
	for _, n := range knownNeighborhoods {
		for _, pat := range n.patterns {
			if strings.Contains(lowerMsg, pat) {
				detectedArea = n.canonical
				break
			}
		}
		if detectedArea != "" {
			break
		}
	}
	if detectedArea == "" && userLocation != "" {
		detectedArea = strings.TrimSpace(userLocation)
	}

	// 3. Extract Rating
	var minRating *float64
	if ratingRegex.MatchString(lowerMsg) || strings.Contains(userMessage, "ምርጥ") || strings.Contains(userMessage, "ጥሩ") {
		r := 4.0
		minRating = &r
	}

	// 4. Extract Dish / Food
	var matchedDish string
	for _, d := range knownDishes {
		for _, a := range d.aliases {
			if strings.Contains(lowerMsg, a) {
				matchedDish = d.canonical
				break
			}
		}
		if matchedDish != "" {
			break
		}
	}

	// If no known dish matched, but message contains a query term (e.g. "salad", "breakfast")
	if matchedDish == "" {
		words := strings.Fields(userMessage)
		// Try to pick food noun if not purely stop words
		for _, w := range words {
			wLow := strings.ToLower(strings.Trim(w, "?!.,;:\"'"))
			if len(wLow) >= 3 && !isStopWord(wLow) {
				matchedDish = wLow
				break
			}
		}
	}

	// 5. Construct intent and reply text
	intent := "food_search"
	var replyText string
	var suggestions []string

	dishDisplay := matchedDish
	if dishDisplay == "" {
		dishDisplay = "popular dishes"
	}

	switch {
	case matchedDish != "" && detectedArea != "" && maxPrice != nil:
		replyText = fmt.Sprintf("Here are the best spots for %s in %s within your budget of %.0f ETB:", dishDisplay, detectedArea, *maxPrice)
		suggestions = append(suggestions, fmt.Sprintf("Top rated %s in %s", dishDisplay, detectedArea), fmt.Sprintf("Other dishes in %s", detectedArea), "Show all price ranges")
	case matchedDish != "" && detectedArea != "":
		replyText = fmt.Sprintf("Here are popular places for %s in %s:", dishDisplay, detectedArea)
		suggestions = append(suggestions, fmt.Sprintf("Under 300 ETB in %s", detectedArea), fmt.Sprintf("Top-rated %s", dishDisplay))
	case matchedDish != "" && maxPrice != nil:
		replyText = fmt.Sprintf("Here are great places serving %s under %.0f ETB:", dishDisplay, *maxPrice)
		suggestions = append(suggestions, fmt.Sprintf("Show in Bole"), fmt.Sprintf("Show in Kazanchis"))
	case matchedDish != "":
		replyText = fmt.Sprintf("Here is where you can enjoy authentic %s in Addis Ababa:", dishDisplay)
		suggestions = append(suggestions, fmt.Sprintf("In Bole"), fmt.Sprintf("Under 300 ETB"), "Top rated only")
	case detectedArea != "" && maxPrice != nil:
		replyText = fmt.Sprintf("Here are recommended food offerings in %s under %.0f ETB:", detectedArea, *maxPrice)
		suggestions = append(suggestions, "Show Tibs", "Show Kitfo", "Show Shiro")
	case detectedArea != "":
		replyText = fmt.Sprintf("Here are popular dining options in %s:", detectedArea)
		suggestions = append(suggestions, fmt.Sprintf("Tibs in %s", detectedArea), fmt.Sprintf("Under 250 ETB in %s", detectedArea))
	default:
		replyText = "Here are top-rated dishes and restaurants across Addis Ababa:"
		suggestions = append(suggestions, "Where to eat Tibs in Bole?", "Best Kitfo under 400 ETB", "Affordable Shiro in Kazanchis")
	}

	return &InterpretedQuery{
		DishOrFood:          matchedDish,
		MaxPriceETB:         maxPrice,
		MinPriceETB:         minPrice,
		MinRating:           minRating,
		Neighborhood:        detectedArea,
		Intent:              intent,
		ReplyText:           replyText,
		FollowUpSuggestions: suggestions,
		FallbackUsed:        true,
	}, nil
}

func isStopWord(w string) bool {
	stopWords := map[string]bool{
		"where": true, "what": true, "when": true, "which": true, "who": true, "how": true,
		"can": true, "could": true, "would": true, "should": true, "will": true,
		"eat": true, "find": true, "get": true, "have": true, "want": true, "need": true,
		"the": true, "this": true, "that": true, "these": true, "those": true,
		"for": true, "with": true, "from": true, "near": true, "about": true,
		"any": true, "some": true, "good": true, "best": true, "place": true, "places": true,
		"spot": true, "spots": true, "restaurant": true, "restaurants": true, "cafe": true,
		"cheap": true, "under": true, "below": true, "birr": true, "etb": true,
	}
	return stopWords[w]
}
