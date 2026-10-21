package tests

import (
	"encoding/json"
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

func TestAssistant_DocumentCompilation(t *testing.T) {
	suffix := time.Now().UnixNano()
	bundle := assistant.RestaurantProfileBundle{
		Business: domain.Business{
			ID:                   domain.BusinessID(1000 + (suffix % 5000)),
			Name:                 "Kategna Restaurant",
			Category:             domain.CategoryRestaurant,
			Cuisine:              "Ethiopian Traditional",
			Neighborhood:         "Bole",
			LocationName:         "Bole Medhanialem",
			PriceRange:           "$$",
			Tags:                 []string{"traditional", "family friendly"},
			PaymentMethods:       []string{"Cash", "Telebirr"},
			ReservationAvailable: true,
			Description:          "Renowned authentic Ethiopian dining experience.",
		},
		Hours: []domain.BusinessHours{
			{DayOfWeek: 1, OpensAt: strPtr("08:00:00"), ClosesAt: strPtr("22:00:00"), IsClosed: false},
		},
		MenuItems: []domain.MenuItem{
			{Name: "Special Kitfo", PriceETBMinor: 38000, IsAvailable: true, Description: "With ayib and gomen"},
			{Name: "Derek Tibs", PriceETBMinor: 32000, IsAvailable: true},
		},
		Rating:      4.7,
		ReviewCount: 85,
	}

	docs := assistant.CompileRestaurantDocuments(bundle)
	if len(docs) != 4 {
		t.Fatalf("Expected 4 documents (profile, menu, faq, review), got %d", len(docs))
	}

	// Verify Profile doc
	profileDoc := docs[0]
	if profileDoc.DocType != "profile" {
		t.Errorf("Expected doc_type 'profile', got %s", profileDoc.DocType)
	}
	if !strings.Contains(profileDoc.Content, "Kategna Restaurant") {
		t.Errorf("Expected profile content to mention venue name")
	}
	if !strings.Contains(profileDoc.Content, "Bole") {
		t.Errorf("Expected profile content to mention neighborhood")
	}

	// Verify Menu doc
	menuDoc := docs[1]
	if menuDoc.DocType != "menu" {
		t.Errorf("Expected doc_type 'menu', got %s", menuDoc.DocType)
	}
	if !strings.Contains(menuDoc.Content, "Special Kitfo: 380 ETB") {
		t.Errorf("Expected menu content to format ETB price, got: %s", menuDoc.Content)
	}

	// Verify FAQ doc
	faqDoc := docs[2]
	if faqDoc.DocType != "faq" {
		t.Errorf("Expected doc_type 'faq', got %s", faqDoc.DocType)
	}
	if !strings.Contains(faqDoc.Content, "Where is Kategna Restaurant located") {
		t.Errorf("Expected FAQ content to contain location question")
	}

	// Verify Review doc
	reviewDoc := docs[3]
	if reviewDoc.DocType != "review" {
		t.Errorf("Expected doc_type 'review', got %s", reviewDoc.DocType)
	}
	if !strings.Contains(reviewDoc.Content, "Customer Reviews and Ratings") {
		t.Errorf("Expected review content to mention customer reviews, got: %s", reviewDoc.Content)
	}
}

func TestAssistant_ChatFlowAndRAGGrounding(t *testing.T) {
	pool, ctx := setupTestDB(t)

	dishRepo := repository.NewDishRepository(pool)
	bizRepo := repository.NewBusinessRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	docRepo := repository.NewPostgresDocumentRepository(pool.Pool)
	assistantRepo := repository.NewAssistantRepository(pool)

	aiCfg := config.AIConfig{} // Empty keys triggers deterministic RAG embedding & local synthesis
	ragClient := assistant.NewGeminiRAGClient(aiCfg)
	assistantSvc := service.NewAssistantService(assistantRepo, docRepo, ragClient)
	ingestionSvc := assistant.NewIngestionService(pool.Pool, docRepo, ragClient)

	suffix := time.Now().UnixNano()

	// 1. Seed Real Venue & Dishes in Neon Postgres
	dishName := fmt.Sprintf("Assistant Kitfo %d", suffix)
	dishSlug := fmt.Sprintf("assistant-kitfo-%d", suffix)
	canonicalDish := &domain.CanonicalDish{
		Name: dishName,
		Slug: dishSlug,
	}
	if err := dishRepo.Create(ctx, canonicalDish); err != nil {
		t.Fatalf("Failed to create canonical dish: %v", err)
	}

	bizBole := &domain.Business{
		Name:              fmt.Sprintf("Bole Kitfo Palace %d", suffix),
		Slug:              fmt.Sprintf("bole-kitfo-%d", suffix),
		Category:          domain.CategoryRestaurant,
		Neighborhood:      "Bole",
		LocationName:      "Bole",
		PriceRange:        "$$",
		PublicationStatus: domain.PublicationStatusPublished,
	}
	if err := bizRepo.Create(ctx, bizBole); err != nil {
		t.Fatalf("Failed to create bizBole: %v", err)
	}

	catBole := &domain.MenuCategory{BusinessID: bizBole.ID, Name: "Mains"}
	_ = menuRepo.CreateCategory(ctx, catBole)

	// Item at 280 ETB (28000 santims)
	menuItemBole := &domain.MenuItem{
		CategoryID:      catBole.ID,
		BusinessID:      bizBole.ID,
		Name:            "Authentic Gurage Kitfo",
		PriceETBMinor:   28000,
		IsAvailable:     true,
		CanonicalDishID: &canonicalDish.ID,
	}
	if err := menuRepo.CreateItem(ctx, menuItemBole); err != nil {
		t.Fatalf("Failed to create item: %v", err)
	}

	// 2. Synchronize restaurant into RAG documents & vector table
	indexedCount, err := ingestionSvc.SyncBusiness(ctx, int64(bizBole.ID))
	if err != nil {
		t.Fatalf("Failed to sync business to RAG documents: %v", err)
	}
	if indexedCount == 0 {
		t.Fatalf("Expected at least 1 document indexed into pgvector")
	}

	// 3. Execute Conversational Chat Query
	chatReq := domain.AssistantChatRequest{
		Message:  fmt.Sprintf("Where can I eat %s in Bole?", dishName),
		Location: "Bole",
	}

	chatResp, err := assistantSvc.Chat(ctx, nil, chatReq)
	if err != nil {
		t.Fatalf("Assistant Chat failed: %v", err)
	}

	if chatResp.SessionID == "" {
		t.Errorf("Expected non-empty session ID")
	}
	if chatResp.Message == "" {
		t.Errorf("Expected grounded natural language answer, got empty string")
	}
	if len(chatResp.FollowUpSuggestions) == 0 {
		t.Errorf("Expected conversational follow-up suggestions")
	}

	// 4. Verify Session Persistence
	savedSession, err := assistantSvc.GetSession(ctx, domain.AssistantSessionID(chatResp.SessionID))
	if err != nil {
		t.Fatalf("Failed to get saved session: %v", err)
	}
	if len(savedSession.Messages) < 2 {
		t.Errorf("Expected at least 2 messages (user + assistant) in session, got %d", len(savedSession.Messages))
	}

	// 5. Multi-turn Follow-up query in same session
	followUpReq := domain.AssistantChatRequest{
		SessionID: chatResp.SessionID,
		Message:   "What are the opening hours?",
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
	docRepo := repository.NewPostgresDocumentRepository(pool.Pool)
	assistantRepo := repository.NewAssistantRepository(pool)
	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	txMgr := repository.NewTxManager(pool)
	memStorage := storage.NewMemoryStorage("https://cdn.cafnet.et")

	discoverySvc := service.NewDiscoveryService(bizRepo, menuRepo, updateRepo, reviewRepo, mediaRepo, dishRepo, memStorage)
	ragClient := assistant.NewGeminiRAGClient(config.AIConfig{})
	assistantSvc := service.NewAssistantService(assistantRepo, docRepo, ragClient)

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
		reqBody := `{"message": "Where can I eat kitfo in Bole under 400 ETB?"}`
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
