package service

import (
	"context"
	"strings"

	"github.com/google/uuid"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service/assistant"
)

// AssistantService orchestrates conversational food discovery interactions,
// grounding natural language requests with real venue taxonomy and offerings.
type AssistantService interface {
	Chat(ctx context.Context, userID *domain.UserID, req domain.AssistantChatRequest) (*domain.AssistantChatResponse, error)
	GetSession(ctx context.Context, sessionID domain.AssistantSessionID) (*domain.AssistantSession, error)
	CreateSession(ctx context.Context, userID *domain.UserID, title string) (*domain.AssistantSession, error)
}

type assistantService struct {
	assistantRepo    repository.AssistantRepository
	discoveryService DiscoveryService
	parser           assistant.AIParser
}

// NewAssistantService constructs an AssistantService instance.
func NewAssistantService(
	assistantRepo repository.AssistantRepository,
	discoveryService DiscoveryService,
	aiCfg config.AIConfig,
) AssistantService {
	return &assistantService{
		assistantRepo:    assistantRepo,
		discoveryService: discoveryService,
		parser:           assistant.NewCompositeParser(aiCfg),
	}
}

// NewAssistantServiceWithParser constructs an AssistantService with a custom AIParser (used in testing).
func NewAssistantServiceWithParser(
	assistantRepo repository.AssistantRepository,
	discoveryService DiscoveryService,
	parser assistant.AIParser,
) AssistantService {
	return &assistantService{
		assistantRepo:    assistantRepo,
		discoveryService: discoveryService,
		parser:           parser,
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
	history, err := s.assistantRepo.ListMessages(ctx, session.ID, 10)
	if err != nil {
		history = nil
	}

	// 3. Save incoming user message
	userMsg := &domain.AssistantMessage{
		ID:        domain.AssistantMessageID(uuid.NewString()),
		SessionID: session.ID,
		Role:      "user",
		Content:   msgContent,
		Intent:    "food_search",
	}
	_ = s.assistantRepo.CreateMessage(ctx, userMsg)

	// 4. Interpret message into validated structured search parameters
	query, err := s.parser.Interpret(ctx, history, msgContent, req.Location)
	if err != nil || query == nil {
		query = &assistant.InterpretedQuery{
			DishOrFood:          msgContent,
			Intent:              "food_search",
			ReplyText:           "Searching for dishes matching your request:",
			FollowUpSuggestions: []string{"Show top rated", "Show under 300 ETB", "Show in Bole"},
			FallbackUsed:        true,
		}
	}

	// 5. Construct domain search filter
	filter := domain.FoodSearchFilter{
		Query:  query.DishOrFood,
		Limit:  20,
		Offset: 0,
	}

	if query.MaxPriceETB != nil && *query.MaxPriceETB > 0 {
		minor := int64(*query.MaxPriceETB * 100)
		filter.MaxPriceETBMinor = &minor
	}
	if query.MinPriceETB != nil && *query.MinPriceETB > 0 {
		minor := int64(*query.MinPriceETB * 100)
		filter.MinPriceETBMinor = &minor
	}
	if query.MinRating != nil && *query.MinRating > 0 {
		filter.MinRating = query.MinRating
	}
	if query.Neighborhood != "" {
		n := strings.TrimSpace(query.Neighborhood)
		filter.Neighborhood = &n
	}

	// 6. Query real PostgreSQL database through DiscoveryService
	searchResp, err := s.discoveryService.SearchFoodWithFilter(ctx, filter)
	if err != nil {
		searchResp = &domain.FoodSearchResponse{
			Results: []domain.FoodSearchResult{},
			Items:   []domain.FoodSearchResult{},
		}
	}

	// If no results with strict filters, try relaxed search without price or area
	if len(searchResp.Results) == 0 && (filter.MaxPriceETBMinor != nil || filter.Neighborhood != nil) && filter.Query != "" {
		relaxedFilter := domain.FoodSearchFilter{
			Query: filter.Query,
			Limit: 10,
		}
		if relaxedResp, rErr := s.discoveryService.SearchFoodWithFilter(ctx, relaxedFilter); rErr == nil && len(relaxedResp.Results) > 0 {
			query.ReplyText = query.ReplyText + " (We found matches elsewhere across Addis Ababa):"
			searchResp = relaxedResp
		}
	}

	// 7. Save assistant message with search context
	var matchedDishID *domain.DishID
	if searchResp.CanonicalDish != nil {
		matchedDishID = &searchResp.CanonicalDish.ID
	}

	assistantMsg := &domain.AssistantMessage{
		ID:                  domain.AssistantMessageID(uuid.NewString()),
		SessionID:           session.ID,
		Role:                "assistant",
		Content:             query.ReplyText,
		Intent:              query.Intent,
		SearchFilter:        &filter,
		MatchedDishID:       matchedDishID,
		ResultsCount:        len(searchResp.Results),
		FollowUpSuggestions: query.FollowUpSuggestions,
	}
	_ = s.assistantRepo.CreateMessage(ctx, assistantMsg)

	return &domain.AssistantChatResponse{
		SessionID:           string(session.ID),
		Message:             query.ReplyText,
		Intent:              query.Intent,
		SearchFilter:        &filter,
		CanonicalDish:       searchResp.CanonicalDish,
		Results:             searchResp.Results,
		Items:               searchResp.Results,
		FollowUpSuggestions: query.FollowUpSuggestions,
		FallbackUsed:        query.FallbackUsed,
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
