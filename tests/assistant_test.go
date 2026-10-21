package tests

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/handler"
	"github.com/Abdibaye/cafnet_backend/internal/http/router"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
	"github.com/Abdibaye/cafnet_backend/internal/service/assistant"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

type mockFailingAIParser struct{}

func (m *mockFailingAIParser) Interpret(ctx context.Context, history []domain.AssistantMessage, userMsg string, userLoc string) (*assistant.InterpretedQuery, error) {
	return nil, errors.New("simulated AI provider timeout or 503 unavailable")
}

func TestAssistant_RuleParserNLP(t *testing.T) {
	parser := assistant.NewRuleParser()
	ctx := context.Background()

	t.Run("Extract Dish, Budget, and Area in English", func(t *testing.T) {
		res, err := parser.Interpret(ctx, nil, "Where can I find kitfo in Bole under 350 ETB?", "")
		if err != nil {
			t.Fatalf("Parser failed: %v", err)
		}
		if res.DishOrFood != "Kitfo" {
			t.Errorf("Expected dish 'Kitfo', got %q", res.DishOrFood)
		}
		if res.Neighborhood != "Bole" {
			t.Errorf("Expected neighborhood 'Bole', got %q", res.Neighborhood)
		}
		if res.MaxPriceETB == nil || *res.MaxPriceETB != 350 {
			t.Errorf("Expected max price 350, got %v", res.MaxPriceETB)
		}
		if !res.FallbackUsed {
			t.Errorf("Expected FallbackUsed to be true for rule parser")
		}
		if len(res.FollowUpSuggestions) == 0 {
			t.Errorf("Expected follow up suggestions")
		}
	})

	t.Run("Extract Multilingual Amharic with Rating and Budget", func(t *testing.T) {
		res, err := parser.Interpret(ctx, nil, "ምርጥ ጥብስ በ 250 ብር በታች በቦሌ", "")
		if err != nil {
			t.Fatalf("Parser failed: %v", err)
		}
		if res.DishOrFood != "Tibs" {
			t.Errorf("Expected dish 'Tibs', got %q", res.DishOrFood)
		}
		if res.Neighborhood != "Bole" {
			t.Errorf("Expected neighborhood 'Bole', got %q", res.Neighborhood)
		}
		if res.MaxPriceETB == nil || *res.MaxPriceETB != 250 {
			t.Errorf("Expected max price 250, got %v", res.MaxPriceETB)
		}
		if res.MinRating == nil || *res.MinRating < 4.0 {
			t.Errorf("Expected min rating >= 4.0 for 'ምርጥ', got %v", res.MinRating)
		}
	})

	t.Run("Extract Area and Budget without Dish Name", func(t *testing.T) {
		res, err := parser.Interpret(ctx, nil, "cheap food in Kazanchis", "")
		if err != nil {
			t.Fatalf("Parser failed: %v", err)
		}
		if res.Neighborhood != "Kazanchis" {
			t.Errorf("Expected neighborhood 'Kazanchis', got %q", res.Neighborhood)
		}
		if res.MaxPriceETB == nil || *res.MaxPriceETB <= 0 {
			t.Errorf("Expected max price for 'cheap', got %v", res.MaxPriceETB)
		}
	})
}

func TestAssistant_GracefulFallbackOnProviderFailure(t *testing.T) {
	// Configure an AIConfig that fails or points to an invalid mock
	cfg := config.AIConfig{
		OpenAIAPIKey: "invalid_key",
		BaseURL:      "http://127.0.0.1:54321/nonexistent",
		Timeout:      100 * time.Millisecond,
	}

	composite := assistant.NewCompositeParser(cfg)
	res, err := composite.Interpret(context.Background(), nil, "Find burger in Bole under 200 birr", "")
	if err != nil {
		t.Fatalf("Expected fallback to succeed transparently without error, got %v", err)
	}

	if !res.FallbackUsed {
		t.Errorf("Expected FallbackUsed to be true upon provider failure")
	}
	if res.DishOrFood != "Burger" {
		t.Errorf("Expected 'Burger', got %q", res.DishOrFood)
	}
	if res.Neighborhood != "Bole" {
		t.Errorf("Expected 'Bole', got %q", res.Neighborhood)
	}
	if res.MaxPriceETB == nil || *res.MaxPriceETB != 200 {
		t.Errorf("Expected 200 ETB, got %v", res.MaxPriceETB)
	}
}

func TestAssistant_ChatFlowAndGrounding(t *testing.T) {
	pool, ctx := setupTestDB(t)

	dishRepo := repository.NewDishRepository(pool)
	bizRepo := repository.NewBusinessRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	mediaRepo := repository.NewMediaRepository(pool)
	assistantRepo := repository.NewAssistantRepository(pool)
	memStorage := storage.NewMemoryStorage("https://cdn.cafnet.et")

	discoverySvc := service.NewDiscoveryService(bizRepo, menuRepo, updateRepo, reviewRepo, mediaRepo, dishRepo, memStorage)
	aiCfg := config.AIConfig{} // Empty keys triggers deterministic rule-based engine
	assistantSvc := service.NewAssistantService(assistantRepo, discoverySvc, aiCfg)

	suffix := time.Now().UnixNano()

	// 1. Seed Real Venue & Dishes in Neon Postgres
	dishName := fmt.Sprintf("Assistant Tibs %d", suffix)
	dishSlug := fmt.Sprintf("assistant-tibs-%d", suffix)
	canonicalDish := &domain.CanonicalDish{
		Name: dishName,
		Slug: dishSlug,
	}
	if err := dishRepo.Create(ctx, canonicalDish); err != nil {
		t.Fatalf("Failed to create canonical dish: %v", err)
	}

	bizBole := &domain.Business{
		Name:              fmt.Sprintf("Bole Tibs House %d", suffix),
		Slug:              fmt.Sprintf("bole-tibs-%d", suffix),
		Category:          domain.CategoryRestaurant,
		Neighborhood:      "Bole",
		LocationName:      "Bole",
		PublicationStatus: domain.PublicationStatusPublished,
	}
	if err := bizRepo.Create(ctx, bizBole); err != nil {
		t.Fatalf("Failed to create bizBole: %v", err)
	}

	catBole := &domain.MenuCategory{BusinessID: bizBole.ID, Name: "Mains"}
	_ = menuRepo.CreateCategory(ctx, catBole)

	// Item at 260 ETB (26000 santims)
	menuItemBole := &domain.MenuItem{
		CategoryID:      catBole.ID,
		BusinessID:      bizBole.ID,
		Name:            "Authentic Sautéed Tibs",
		PriceETBMinor:   26000,
		IsAvailable:     true,
		CanonicalDishID: &canonicalDish.ID,
	}
	if err := menuRepo.CreateItem(ctx, menuItemBole); err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// 2. Execute Conversational Chat
	chatReq := domain.AssistantChatRequest{
		Message: fmt.Sprintf("Where can I eat %s in Bole under 300 ETB?", dishName),
	}

	chatResp, err := assistantSvc.Chat(ctx, nil, chatReq)
	if err != nil {
		t.Fatalf("Assistant Chat failed: %v", err)
	}

	if chatResp.SessionID == "" {
		t.Errorf("Expected non-empty session ID")
	}
	if len(chatResp.Results) == 0 {
		t.Fatalf("Expected at least 1 real grounded restaurant result, got 0")
	}

	topResult := chatResp.Results[0]
	if topResult.Business.Neighborhood != "Bole" {
		t.Errorf("Expected Bole venue, got %q", topResult.Business.Neighborhood)
	}
	if topResult.MenuItem.PriceETBMinor != 26000 {
		t.Errorf("Expected price 26000 santims (260 ETB), got %d", topResult.MenuItem.PriceETBMinor)
	}
	if topResult.MenuItem.Name != "Authentic Sautéed Tibs" {
		t.Errorf("Expected real authentic menu item name, got %q", topResult.MenuItem.Name)
	}

	// 3. Verify Session Persistence
	savedSession, err := assistantSvc.GetSession(ctx, domain.AssistantSessionID(chatResp.SessionID))
	if err != nil {
		t.Fatalf("Failed to get saved session: %v", err)
	}
	if len(savedSession.Messages) < 2 {
		t.Errorf("Expected at least 2 messages in session, got %d", len(savedSession.Messages))
	}

	// 4. Follow-up query in same session
	followUpReq := domain.AssistantChatRequest{
		SessionID: chatResp.SessionID,
		Message:   "Show places in Kazanchis",
	}
	followUpResp, err := assistantSvc.Chat(ctx, nil, followUpReq)
	if err != nil {
		t.Fatalf("Follow-up failed: %v", err)
	}
	if followUpResp.SessionID != chatResp.SessionID {
		t.Errorf("Expected same session ID %s, got %s", chatResp.SessionID, followUpResp.SessionID)
	}
}

func TestAssistant_HTTPEndpoints(t *testing.T) {
	pool, _ := setupTestDB(t)

	dishRepo := repository.NewDishRepository(pool)
	bizRepo := repository.NewBusinessRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	mediaRepo := repository.NewMediaRepository(pool)
	assistantRepo := repository.NewAssistantRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	txMgr := repository.NewTxManager(pool)
	memStorage := storage.NewMemoryStorage("https://cdn.cafnet.et")

	discoverySvc := service.NewDiscoveryService(bizRepo, menuRepo, updateRepo, reviewRepo, mediaRepo, dishRepo, memStorage)
	assistantSvc := service.NewAssistantService(assistantRepo, discoverySvc, config.AIConfig{})

	authCfg := &config.AuthConfig{
		SessionCookieName: "cafnet_session",
		SessionTTL:        24 * time.Hour,
		Environment:       "development",
	}
	authSvc := service.NewAuthService(authCfg, userRepo, sessionRepo, otpRepo, txMgr, nil)

	handlers := &router.Handlers{
		Health:    handler.NewHealthHandler(pool),
		Discovery: handler.NewDiscoveryHandler(discoverySvc),
		Assistant: handler.NewAssistantHandler(assistantSvc),
	}

	serverCfg := &config.ServerConfig{
		Auth:           *authCfg,
		AllowedOrigins: []string{"http://localhost:3000"},
	}

	appRouter := router.NewRouter(serverCfg, authSvc, handlers)
	ts := httptest.NewServer(appRouter)
	defer ts.Close()

	t.Run("POST /api/v1/assistant/chat with Valid Query", func(t *testing.T) {
		reqBody := `{"message": "Where can I eat tibs in Bole under 400 ETB?"}`
		res, err := http.Post(ts.URL+"/api/v1/assistant/chat", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("HTTP Post failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			b, _ := io.ReadAll(res.Body)
			t.Fatalf("Expected 200 OK, got %d: %s", res.StatusCode, string(b))
		}

		var envelope struct {
			Status string                       `json:"status"`
			Data   domain.AssistantChatResponse `json:"data"`
		}
		if err := json.NewDecoder(res.Body).Decode(&envelope); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if envelope.Data.SessionID == "" {
			t.Errorf("Expected non-empty session_id")
		}
		if envelope.Data.Message == "" {
			t.Errorf("Expected assistant message")
		}
		if envelope.Data.SearchFilter == nil {
			t.Errorf("Expected non-nil search filter")
		}
	})

	t.Run("POST /api/v1/assistant/chat Empty Message Rejection", func(t *testing.T) {
		reqBody := `{"message": "   "}`
		res, err := http.Post(ts.URL+"/api/v1/assistant/chat", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("HTTP Post failed: %v", err)
		}
		defer res.Body.Close()

		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("Expected 400 Bad Request, got %d", res.StatusCode)
		}
	})

	t.Run("GET /api/v1/assistant/sessions/{id}", func(t *testing.T) {
		// First create a session
		createReq := `{"title": "Weekend Dinner Plan"}`
		res, err := http.Post(ts.URL+"/api/v1/assistant/sessions", "application/json", strings.NewReader(createReq))
		if err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
		defer res.Body.Close()

		var createEnv struct {
			Data domain.AssistantSession `json:"data"`
		}
		_ = json.NewDecoder(res.Body).Decode(&createEnv)
		sessID := string(createEnv.Data.ID)
		if sessID == "" {
			t.Fatalf("Expected valid session ID")
		}

		// Retrieve session
		getRes, err := http.Get(fmt.Sprintf("%s/api/v1/assistant/sessions/%s", ts.URL, sessID))
		if err != nil {
			t.Fatalf("Failed to get session: %v", err)
		}
		defer getRes.Body.Close()

		if getRes.StatusCode != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", getRes.StatusCode)
		}
	})
}
