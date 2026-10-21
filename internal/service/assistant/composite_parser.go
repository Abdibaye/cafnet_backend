package assistant

import (
	"context"
	"log/slog"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type compositeParser struct {
	aiClient   AIParser
	ruleParser AIParser
	hasAPIKey  bool
}

// NewCompositeParser returns an AIParser that attempts to use the AI provider (Gemini/OpenAI)
// if configured, and transparently falls back to the deterministic rule-based extractor
// upon any network timeout, provider error, or missing credentials.
func NewCompositeParser(cfg config.AIConfig) AIParser {
	return &compositeParser{
		aiClient:   NewGeminiClient(cfg),
		ruleParser: NewRuleParser(),
		hasAPIKey:  cfg.HasAPIKey(),
	}
}

func (p *compositeParser) Interpret(ctx context.Context, history []domain.AssistantMessage, userMessage string, userLocation string) (*InterpretedQuery, error) {
	if p.hasAPIKey {
		query, err := p.aiClient.Interpret(ctx, history, userMessage, userLocation)
		if err == nil && query != nil {
			return query, nil
		}
		slog.Warn("AI provider request failed; engaging deterministic fallback", "error", err)
	}

	// Graceful deterministic fallback
	return p.ruleParser.Interpret(ctx, history, userMessage, userLocation)
}
