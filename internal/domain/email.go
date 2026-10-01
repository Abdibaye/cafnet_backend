package domain

import (
	"net/mail"
	"strings"
)

// NormalizeEmail trims whitespace, converts to lowercase, and validates email syntax.
func NormalizeEmail(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" || len(trimmed) > 254 {
		return "", ErrInvalidEmail
	}

	// Reject if contains whitespace or control characters
	if strings.ContainsAny(trimmed, " \t\r\n<>") {
		return "", ErrInvalidEmail
	}

	addr, err := mail.ParseAddress(trimmed)
	if err != nil || addr.Address != trimmed {
		return "", ErrInvalidEmail
	}

	parts := strings.Split(trimmed, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", ErrInvalidEmail
	}

	domainPart := parts[1]
	if !strings.Contains(domainPart, ".") || strings.HasPrefix(domainPart, ".") || strings.HasSuffix(domainPart, ".") {
		return "", ErrInvalidEmail
	}

	return strings.ToLower(trimmed), nil
}
