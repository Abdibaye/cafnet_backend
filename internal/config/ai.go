package config

import (
	"os"
	"time"
)

// AIConfig controls connection and timeout parameters for the AI food assistant.
type AIConfig struct {
	GeminiAPIKey string
	OpenAIAPIKey string
	BaseURL      string
	Model        string
	Timeout      time.Duration
}

// LoadAIConfig populates AI settings from environment variables with sensible defaults.
func LoadAIConfig() AIConfig {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	openaiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("AI_BASE_URL")
	model := os.Getenv("AI_MODEL")

	if model == "" {
		if geminiKey != "" {
			model = "gemini-1.5-flash"
		} else {
			model = "gpt-4o-mini"
		}
	}

	timeout := parseDurationEnv("AI_TIMEOUT", 8*time.Second)

	return AIConfig{
		GeminiAPIKey: geminiKey,
		OpenAIAPIKey: openaiKey,
		BaseURL:      baseURL,
		Model:        model,
		Timeout:      timeout,
	}
}

// HasAPIKey returns true if either a Gemini or OpenAI key is present.
func (c AIConfig) HasAPIKey() bool {
	return c.GeminiAPIKey != "" || c.OpenAIAPIKey != ""
}
