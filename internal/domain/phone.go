package domain

import (
	"regexp"
	"strings"
)

var (
	// Clean non-digit and non-plus characters
	phoneSanitizeRegex = regexp.MustCompile(`[^\d+]`)
	// Strictly validate 9-digit Ethiopian mobile body: starts with 9 or 7, followed by 8 digits
	ethiopianSubscriberRegex = regexp.MustCompile(`^[97]\d{8}$`)
)

// NormalizePhone converts any standard Ethiopian phone representation into canonical E.164 (+251...).
// Supported input patterns:
//   - "+251911223344" / "+251711223344"
//   - "251911223344"  / "251711223344"
//   - "0911223344"    / "0711223344"
//   - "911223344"     / "711223344"
//
// Returns domain.ErrInvalidPhone if the input cannot be resolved deterministically.
func NormalizePhone(input string) (string, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return "", ErrInvalidPhone
	}

	// Remove common punctuation: spaces, hyphens, parentheses, dots
	cleaned := phoneSanitizeRegex.ReplaceAllString(trimmed, "")

	var subscriber string

	if strings.HasPrefix(cleaned, "+251") {
		subscriber = cleaned[4:]
	} else if strings.HasPrefix(cleaned, "251") {
		subscriber = cleaned[3:]
	} else if strings.HasPrefix(cleaned, "0") {
		subscriber = cleaned[1:]
	} else {
		subscriber = cleaned
	}

	if !ethiopianSubscriberRegex.MatchString(subscriber) {
		return "", ErrInvalidPhone
	}

	return "+251" + subscriber, nil
}
