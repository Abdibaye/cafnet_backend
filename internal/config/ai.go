package config

import (
	"os"
	"time"
)

// AIConfig controls connection and timeout parameters for the Gemini RAG food assistant.
type AIConfig struct {
	GeminiAPIKey   string
	BaseURL        string
	Model          string
	EmbeddingModel string
	Timeout        time.Duration
}

// LoadAIConfig populates AI settings from environment variables with sensible defaults.
func LoadAIConfig() AIConfig {
	geminiKey := os.Getenv("GEMINI_API_KEY")
	baseURL := os.Getenv("GEMINI_BASE_URL")
	if baseURL == "" {
		baseURL = "https://generativelanguage.googleapis.com"
	}

	model := os.Getenv("GEMINI_MODEL")
	if model == "" {
		model = os.Getenv("AI_MODEL")
	}
	if model == "" {
		model = "gemini-1.5-flash"
	}

	embeddingModel := os.Getenv("GEMINI_EMBEDDING_MODEL")
	if embeddingModel == "" {
		embeddingModel = "text-embedding-004"
	}

	timeout := parseDurationEnv("AI_TIMEOUT", 10*time.Second)

	return AIConfig{
		GeminiAPIKey:   geminiKey,
		BaseURL:        baseURL,
		Model:          model,
		EmbeddingModel: embeddingModel,
		Timeout:        timeout,
	}
}

// HasAPIKey returns true if a Gemini API key is configured.
func (c AIConfig) HasAPIKey() bool {
	return c.GeminiAPIKey != ""
}
