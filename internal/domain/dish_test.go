package domain_test

import (
	"testing"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

func TestNormalizeAlias(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Tibs", "tibs"},
		{"  TIBSE  ", "tibse"},
		{"Special   Beef    Tibs", "special beef tibs"},
		{"\"Kitfo\"", "kitfo"},
		{"ጥብስ", "ጥብስ"},
		{"  ጥብስ  ", "ጥብስ"},
		{"(Shiro)", "shiro"},
		{"", ""},
		{"   ", ""},
		{"---", ""},
		{"Doro Wot!", "doro wot"},
		{"ዶሮ ወጥ", "ዶሮ ወጥ"},
	}

	for _, tc := range tests {
		got := domain.NormalizeAlias(tc.input)
		if got != tc.expected {
			t.Errorf("NormalizeAlias(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}
