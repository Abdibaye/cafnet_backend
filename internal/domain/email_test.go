package domain_test

import (
	"testing"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

func TestNormalizeEmail(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expected  string
		expectErr bool
	}{
		{
			name:      "Valid standard email",
			input:     "user@example.com",
			expected:  "user@example.com",
			expectErr: false,
		},
		{
			name:      "Trim whitespace and lowercase",
			input:     "  User@Example.COM  ",
			expected:  "user@example.com",
			expectErr: false,
		},
		{
			name:      "Subdomain email",
			input:     "john.doe@mail.sub.cafnet.et",
			expected:  "john.doe@mail.sub.cafnet.et",
			expectErr: false,
		},
		{
			name:      "Empty string",
			input:     "",
			expectErr: true,
		},
		{
			name:      "Whitespace only",
			input:     "   ",
			expectErr: true,
		},
		{
			name:      "Missing at sign",
			input:     "userexample.com",
			expectErr: true,
		},
		{
			name:      "Missing domain",
			input:     "user@",
			expectErr: true,
		},
		{
			name:      "Missing local part",
			input:     "@example.com",
			expectErr: true,
		},
		{
			name:      "Domain without dot",
			input:     "user@localhost",
			expectErr: true,
		},
		{
			name:      "Domain starts with dot",
			input:     "user@.com",
			expectErr: true,
		},
		{
			name:      "Domain ends with dot",
			input:     "user@com.",
			expectErr: true,
		},
		{
			name:      "Internal spaces",
			input:     "user name@example.com",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := domain.NormalizeEmail(tt.input)
			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error for input %q, got nil", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error for input %q: %v", tt.input, err)
				}
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}
