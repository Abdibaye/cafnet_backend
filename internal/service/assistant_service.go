package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service/assistant"
)

// AssistantService orchestrates conversational dining discovery using Gemini-powered RAG.
type AssistantService interface {
	Chat(ctx context.Context, userID *domain.UserID, req domain.AssistantChatRequest) (*domain.AssistantChatResponse, error)
	GetSession(ctx context.Context, sessionID domain.AssistantSessionID) (*domain.AssistantSession, error)
	CreateSession(ctx context.Context, userID *domain.UserID, title string) (*domain.AssistantSession, error)
}

type assistantService struct {
	assistantRepo repository.AssistantRepository
	docRepo       repository.RestaurantDocumentRepository
	ragClient     assistant.GeminiRAGClient
}

// NewAssistantService constructs a Gemini RAG AssistantService.
func NewAssistantService(
	assistantRepo repository.AssistantRepository,
	docRepo repository.RestaurantDocumentRepository,
	ragClient assistant.GeminiRAGClient,
) AssistantService {
	return &assistantService{
		assistantRepo: assistantRepo,
		docRepo:       docRepo,
		ragClient:     ragClient,
	}
}

func (s *assistantService) CreateSession(ctx context.Context, userID *domain.UserID, title string) (*domain.AssistantSession, error) {
	if strings.TrimSpace(title) == "" {
		title = "Food Discovery"
	}

	session := &domain.AssistantSession{
		ID:     domain.AssistantSessionID(uuid.NewString()),
		UserID: userID,
		Title:  title,
	}

	if err := s.assistantRepo.CreateSession(ctx, session); err != nil {
		return nil, err
	}
	return session, nil
}

func (s *assistantService) GetSession(ctx context.Context, sessionID domain.AssistantSessionID) (*domain.AssistantSession, error) {
	return s.assistantRepo.GetSessionByID(ctx, sessionID)
}

func (s *assistantService) Chat(ctx context.Context, userID *domain.UserID, req domain.AssistantChatRequest) (*domain.AssistantChatResponse, error) {
	msgContent := strings.TrimSpace(req.Message)
	if msgContent == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "message cannot be empty", nil)
	}
	if len(msgContent) > 1000 {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "message exceeds maximum length of 1000 characters", nil)
	}

	// 1. Resolve or create conversational session
	var session *domain.AssistantSession
	if req.SessionID != "" {
		sID := domain.AssistantSessionID(strings.TrimSpace(req.SessionID))
		existing, err := s.assistantRepo.GetSessionByID(ctx, sID)
		if err == nil && existing != nil {
			session = existing
		}
	}

	if session == nil {
		title := generateSessionTitle(msgContent)
		newSess := &domain.AssistantSession{
			ID:     domain.AssistantSessionID(uuid.NewString()),
			UserID: userID,
			Title:  title,
		}
		if err := s.assistantRepo.CreateSession(ctx, newSess); err != nil {
			return nil, err
		}
		session = newSess
	}

	// 2. Fetch conversation history context
	history, err := s.assistantRepo.ListMessages(ctx, session.ID, 8)
	if err != nil {
		history = nil
	}

	// 3. Save incoming user message
	userMsg := &domain.AssistantMessage{
		ID:        domain.AssistantMessageID(uuid.NewString()),
		SessionID: session.ID,
		Role:      "user",
		Content:   msgContent,
		Intent:    "food_rag",
	}
	_ = s.assistantRepo.CreateMessage(ctx, userMsg)

	// 4. Compute vector embedding of user prompt via Gemini
	queryEmbedding, err := s.ragClient.Embed(ctx, msgContent)
	if err != nil {
		queryEmbedding = nil
	}

	// 5. Construct hybrid search filter
	filter := domain.DocumentSearchFilter{
		Query: msgContent,
		Limit: 6,
	}

	if req.Location != "" {
		loc := strings.TrimSpace(req.Location)
		filter.Neighborhood = &loc
	}

	// 6. Retrieve relevant restaurant documents from pgvector
	retrievedDocs, err := s.docRepo.SearchSimilar(ctx, queryEmbedding, filter)
	if err != nil {
		retrievedDocs = nil
	}

	// If filtered search returned zero documents (e.g. strict location), try relaxed search across all areas
	if len(retrievedDocs) == 0 && filter.Neighborhood != nil {
		relaxedFilter := domain.DocumentSearchFilter{
			Query: msgContent,
			Limit: 6,
		}
		if relaxed, rErr := s.docRepo.SearchSimilar(ctx, queryEmbedding, relaxedFilter); rErr == nil && len(relaxed) > 0 {
			retrievedDocs = relaxed
		}
	}

	// 7. Synthesize grounded answer via Gemini RAG
	answer, followUps, err := s.ragClient.GenerateGroundedAnswer(ctx, history, msgContent, retrievedDocs)
	if err != nil || answer == "" {
		answer = "I'm having trouble retrieving verified dining options right now. Please try again in a moment."
		followUps = []string{"Show popular places in Bole", "Show traditional kitfo", "Show budget options"}
	}

	// 8. Save assistant message with turn history
	assistantMsg := &domain.AssistantMessage{
		ID:                  domain.AssistantMessageID(uuid.NewString()),
		SessionID:           session.ID,
		Role:                "assistant",
		Content:             answer,
		Intent:              "food_rag",
		ResultsCount:        len(retrievedDocs),
		FollowUpSuggestions: followUps,
	}
	_ = s.assistantRepo.CreateMessage(ctx, assistantMsg)

	return &domain.AssistantChatResponse{
		SessionID:           string(session.ID),
		Message:             answer,
		Intent:              "food_rag",
		Results:             []domain.FoodSearchResult{},
		Items:               []domain.FoodSearchResult{},
		FollowUpSuggestions: followUps,
		FallbackUsed:        false,
	}, nil
}

func generateSessionTitle(msg string) string {
	words := strings.Fields(msg)
	if len(words) == 0 {
		return "Food Discovery"
	}
	if len(words) <= 5 {
		return msg
	}
	return strings.Join(words[:5], " ") + "..."
}
