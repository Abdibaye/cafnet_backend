package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/handler"
	"github.com/Abdibaye/cafnet_backend/internal/http/router"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

type httpTestHarness struct {
	server        *httptest.Server
	client        *http.Client
	cfg           *config.ServerConfig
	userRepo      repository.UserRepository
	businessRepo  repository.BusinessRepository
	ownershipRepo repository.OwnershipRepository
	claimRepo     repository.ClaimRepository
	subRepo       repository.SubmissionRepository
	menuRepo      repository.MenuRepository
	updateRepo    repository.UpdateRepository
	resRepo       repository.ReservationRepository
	reviewRepo    repository.ReviewRepository
	reportRepo    repository.ReportRepository
	custRepo      repository.CustomerListRepository
	adminRepo     repository.AdminActionRepository
	authService   service.AuthService
}

func setupHTTPTest(t *testing.T) *httpTestHarness {
	t.Helper()

	pool, _ := setupTestDB(t)

	cfg := &config.ServerConfig{
		HTTPAddr:       ":8080",
		Environment:    "test",
		AllowedOrigins: []string{"http://localhost:3000", "http://127.0.0.1:3000"},
		MaxBodyBytes:   1024 * 1024,
		Auth: config.AuthConfig{
			OTPTTL:             5 * time.Minute,
			OTPMaxAttempts:     3,
			OTPRequestCooldown: 10 * time.Millisecond,
			SessionTTL:         24 * time.Hour,
			SessionCookieName:  "cafnet_session",
			AllowedOrigins:     []string{"http://localhost:3000", "http://127.0.0.1:3000"},
			DevMode:            true,
			DeterministicOTP:   "123456",
			Environment:        "test",
		},
	}

	txMgr := repository.NewTxManager(pool)
	userRepo := repository.NewUserRepository(pool)
	businessRepo := repository.NewBusinessRepository(pool)
	ownershipRepo := repository.NewOwnershipRepository(pool)
	claimRepo := repository.NewClaimRepository(pool)
	subRepo := repository.NewSubmissionRepository(pool)
	menuRepo := repository.NewMenuRepository(pool)
	updateRepo := repository.NewUpdateRepository(pool)
	reservationRepo := repository.NewReservationRepository(pool)
	reviewRepo := repository.NewReviewRepository(pool)
	reportRepo := repository.NewReportRepository(pool)
	customerListRepo := repository.NewCustomerListRepository(pool)
	adminActionRepo := repository.NewAdminActionRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)

	authzService := service.NewAuthorizationService(ownershipRepo, businessRepo)
	ownershipWorkflow := service.NewOwnershipWorkflowService(
		authzService,
		claimRepo,
		subRepo,
		businessRepo,
		ownershipRepo,
		adminActionRepo,
		txMgr,
	)

	authService := service.NewAuthService(&cfg.Auth, userRepo, sessionRepo, otpRepo, txMgr, service.NewMockSMSProvider())
	discoveryService := service.NewDiscoveryService(businessRepo, menuRepo, updateRepo, reviewRepo)
	customerService := service.NewCustomerService(customerListRepo, businessRepo, claimRepo, subRepo)
	businessService := service.NewBusinessService(businessRepo, ownershipRepo, authzService, txMgr)
	menuService := service.NewMenuService(menuRepo, authzService, txMgr)
	updatesService := service.NewUpdatesService(updateRepo, authzService, customerListRepo)
	reservationService := service.NewReservationService(reservationRepo, businessRepo, authzService, txMgr)
	reviewService := service.NewReviewService(reviewRepo, reservationRepo, authzService, txMgr)
	reportService := service.NewReportService(reportRepo, reviewRepo, businessRepo)
	adminService := service.NewAdminService(
		ownershipWorkflow,
		authzService,
		subRepo,
		claimRepo,
		businessRepo,
		reviewRepo,
		reportRepo,
		adminActionRepo,
		txMgr,
	)

	handlers := &router.Handlers{
		Health:      handler.NewHealthHandler(pool),
		Auth:        handler.NewAuthHandler(authService, &cfg.Auth),
		Discovery:   handler.NewDiscoveryHandler(discoveryService),
		Customer:    handler.NewCustomerHandler(customerService),
		Business:    handler.NewBusinessHandler(businessService),
		Menu:        handler.NewMenuHandler(menuService),
		Updates:     handler.NewUpdatesHandler(updatesService),
		Reservation: handler.NewReservationHandler(reservationService),
		Review:      handler.NewReviewHandler(reviewService),
		Report:      handler.NewReportHandler(reportService),
		Admin:       handler.NewAdminHandler(adminService),
	}

	routerHandler := router.NewRouter(cfg, authService, handlers)
	ts := httptest.NewServer(routerHandler)
	t.Cleanup(ts.Close)

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 60 * time.Second}

	return &httpTestHarness{
		server:        ts,
		client:        client,
		cfg:           cfg,
		userRepo:      userRepo,
		businessRepo:  businessRepo,
		ownershipRepo: ownershipRepo,
		claimRepo:     claimRepo,
		subRepo:       subRepo,
		menuRepo:      menuRepo,
		updateRepo:    updateRepo,
		resRepo:       reservationRepo,
		reviewRepo:    reviewRepo,
		reportRepo:    reportRepo,
		custRepo:      customerListRepo,
		adminRepo:     adminActionRepo,
		authService:   authService,
	}
}

// loginClient performs OTP request and verify, returning an authenticated client and user.
func (h *httpTestHarness) loginClient(t *testing.T, phone string, role domain.UserRole) (*http.Client, *domain.User) {
	t.Helper()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar, Timeout: 60 * time.Second}

	// 1. Request OTP
	reqBody, _ := json.Marshal(map[string]string{"phone": phone})
	resp, err := client.Post(h.server.URL+"/api/v1/auth/otp/request", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("failed to request OTP: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("request OTP failed (%d): %s", resp.StatusCode, string(body))
	}

	var reqResult struct {
		Status string `json:"status"`
		Data   struct {
			ChallengeID string `json:"challenge_id"`
			DevOTP      string `json:"dev_otp"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&reqResult); err != nil {
		t.Fatalf("failed to decode OTP request response: %v", err)
	}

	otpCode := reqResult.Data.DevOTP
	if otpCode == "" {
		otpCode = "123456"
	}

	// 2. Verify OTP
	verifyBody, _ := json.Marshal(map[string]string{
		"challenge_id": reqResult.Data.ChallengeID,
		"phone":        phone,
		"otp_code":     otpCode,
	})
	vResp, err := client.Post(h.server.URL+"/api/v1/auth/otp/verify", "application/json", bytes.NewReader(verifyBody))
	if err != nil {
		t.Fatalf("failed to verify OTP: %v", err)
	}
	defer vResp.Body.Close()
	if vResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(vResp.Body)
		t.Fatalf("verify OTP failed (%d): %s", vResp.StatusCode, string(body))
	}

	var verifyResult struct {
		Status string `json:"status"`
		Data   struct {
			User *domain.User `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(vResp.Body).Decode(&verifyResult); err != nil {
		t.Fatalf("failed to decode verify response: %v", err)
	}

	user := verifyResult.Data.User
	if role != "" && role != user.Role {
		user.Role = role
		if err := h.userRepo.Update(context.Background(), user); err != nil {
			t.Fatalf("failed to update user role to %s: %v", role, err)
		}
	}

	return client, user
}

func (h *httpTestHarness) createBusiness(t *testing.T, name string, pubStatus domain.PublicationStatus) *domain.Business {
	t.Helper()
	b := &domain.Business{
		Name:                 name,
		Slug:                 fmt.Sprintf("%s-%d", strings.ToLower(strings.ReplaceAll(name, " ", "-")), time.Now().UnixNano()%100000),
		Category:             domain.CategoryCafe,
		Cuisine:              "Coffee & Pastries",
		Description:          "Delicious artisan coffee",
		ShortDescription:     "Coffee shop",
		LocationName:         "Bole",
		Neighborhood:         fmt.Sprintf("Bole-%d", time.Now().UnixNano()%1000000),
		Phone:                fmt.Sprintf("+251911%06d", time.Now().UnixNano()%1000000),
		PriceRange:           domain.PriceRangeModerate,
		Tags:                 []string{"Coffee", "WiFi"},
		PaymentMethods:       []string{"Cash", "Telebirr"},
		ReservationAvailable: true,
		PublicationStatus:    pubStatus,
		VerificationStatus:   domain.VerificationStatusVerified,
		ClaimStatus:          domain.ClaimStatusUnclaimed,
	}
	if err := h.businessRepo.Create(context.Background(), b); err != nil {
		t.Fatalf("failed to create test business: %v", err)
	}
	return b
}

func TestHTTP_HealthAndReadiness(t *testing.T) {
	h := setupHTTPTest(t)

	// GET /health
	resp, err := h.client.Get(h.server.URL + "/health")
	if err != nil {
		t.Fatalf("GET /health failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /health, got %d", resp.StatusCode)
	}

	var healthRes struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&healthRes); err != nil {
		t.Fatalf("failed to decode /health JSON: %v", err)
	}
	if healthRes.Data["status"] != "ok" {
		t.Fatalf("expected status 'ok', got %v", healthRes.Data["status"])
	}

	// Security Headers verification
	if nosniff := resp.Header.Get("X-Content-Type-Options"); nosniff != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff, got %q", nosniff)
	}
	if reqID := resp.Header.Get("X-Request-ID"); reqID == "" {
		t.Errorf("expected X-Request-ID header to be present")
	}

	// GET /ready
	rResp, err := h.client.Get(h.server.URL + "/ready")
	if err != nil {
		t.Fatalf("GET /ready failed: %v", err)
	}
	defer rResp.Body.Close()

	if rResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /ready, got %d", rResp.StatusCode)
	}
	var readyRes struct {
		Data map[string]string `json:"data"`
	}
	if err := json.NewDecoder(rResp.Body).Decode(&readyRes); err != nil {
		t.Fatalf("failed to decode /ready JSON: %v", err)
	}
	if readyRes.Data["status"] != "ready" || readyRes.Data["database"] != "connected" {
		t.Fatalf("unexpected /ready response: %v", readyRes.Data)
	}
}

func TestHTTP_CORS_And_SecurityHeaders(t *testing.T) {
	h := setupHTTPTest(t)

	// Preflight OPTIONS request with allowed origin
	req, err := http.NewRequest(http.MethodOptions, h.server.URL+"/api/v1/businesses", nil)
	if err != nil {
		t.Fatalf("failed to create OPTIONS request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:3000")
	req.Header.Set("Access-Control-Request-Method", "GET")

	resp, err := h.client.Do(req)
	if err != nil {
		t.Fatalf("OPTIONS request failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204 No Content for preflight, got %d", resp.StatusCode)
	}

	if allowOrigin := resp.Header.Get("Access-Control-Allow-Origin"); allowOrigin != "http://localhost:3000" {
		t.Errorf("expected Access-Control-Allow-Origin: http://localhost:3000, got %q", allowOrigin)
	}
	if allowCreds := resp.Header.Get("Access-Control-Allow-Credentials"); allowCreds != "true" {
		t.Errorf("expected Access-Control-Allow-Credentials: true, got %q", allowCreds)
	}

	// Request with disallowed origin
	reqBad, _ := http.NewRequest(http.MethodOptions, h.server.URL+"/api/v1/businesses", nil)
	reqBad.Header.Set("Origin", "http://evil-attacker.com")
	respBad, err := h.client.Do(reqBad)
	if err != nil {
		t.Fatalf("bad origin request failed: %v", err)
	}
	defer respBad.Body.Close()

	if respBad.Header.Get("Access-Control-Allow-Origin") != "" {
		t.Errorf("expected no CORS allow header for disallowed origin, got %q", respBad.Header.Get("Access-Control-Allow-Origin"))
	}
}

func TestHTTP_AuthenticationLifecycle(t *testing.T) {
	h := setupHTTPTest(t)

	phone := fmt.Sprintf("+251911%06d", time.Now().UnixNano()%1000000)

	// 1. Request OTP with invalid phone
	badReq, _ := json.Marshal(map[string]string{"phone": "invalid-phone"})
	badResp, err := h.client.Post(h.server.URL+"/api/v1/auth/otp/request", "application/json", bytes.NewReader(badReq))
	if err != nil {
		t.Fatalf("failed to send bad phone request: %v", err)
	}
	defer badResp.Body.Close()
	if badResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid phone, got %d", badResp.StatusCode)
	}

	// 2. Request OTP with valid phone
	validReq, _ := json.Marshal(map[string]string{"phone": phone})
	otpResp, err := h.client.Post(h.server.URL+"/api/v1/auth/otp/request", "application/json", bytes.NewReader(validReq))
	if err != nil {
		t.Fatalf("failed to request OTP: %v", err)
	}
	defer otpResp.Body.Close()
	if otpResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for OTP request, got %d", otpResp.StatusCode)
	}

	var reqData struct {
		Status string `json:"status"`
		Data   struct {
			ChallengeID string `json:"challenge_id"`
			DevOTP      string `json:"dev_otp"`
		} `json:"data"`
	}
	if err := json.NewDecoder(otpResp.Body).Decode(&reqData); err != nil {
		t.Fatalf("failed to parse OTP request response: %v", err)
	}

	challengeID := reqData.Data.ChallengeID
	if challengeID == "" {
		t.Fatal("expected challenge_id to be non-empty")
	}

	// 3. Verify OTP with incorrect code
	wrongVerify, _ := json.Marshal(map[string]string{
		"challenge_id": challengeID,
		"phone":        phone,
		"otp_code":     "000000",
	})
	wrongResp, err := h.client.Post(h.server.URL+"/api/v1/auth/otp/verify", "application/json", bytes.NewReader(wrongVerify))
	if err != nil {
		t.Fatalf("failed to post wrong OTP: %v", err)
	}
	defer wrongResp.Body.Close()
	if wrongResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for wrong OTP, got %d", wrongResp.StatusCode)
	}

	// 4. Verify OTP with valid code ("123456")
	validVerify, _ := json.Marshal(map[string]string{
		"challenge_id": challengeID,
		"phone":        phone,
		"otp_code":     "123456",
	})

	jar, _ := cookiejar.New(nil)
	userClient := &http.Client{Jar: jar, Timeout: 60 * time.Second}

	vResp, err := userClient.Post(h.server.URL+"/api/v1/auth/otp/verify", "application/json", bytes.NewReader(validVerify))
	if err != nil {
		t.Fatalf("failed to post valid OTP: %v", err)
	}
	defer vResp.Body.Close()

	if vResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(vResp.Body)
		t.Fatalf("expected 200 OK for valid OTP verification, got %d: %s", vResp.StatusCode, string(body))
	}

	rawBody, _ := io.ReadAll(vResp.Body)
	// INVARIANT: Response body MUST NOT contain raw session token or OTP secrets
	if strings.Contains(string(rawBody), "raw_token") || strings.Contains(string(rawBody), "token_hash") {
		t.Errorf("response body leaks session token or hash: %s", string(rawBody))
	}

	// Verify Set-Cookie header exists and has HttpOnly
	cookies := vResp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == h.cfg.Auth.SessionCookieName {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil {
		t.Fatalf("expected %q cookie in Set-Cookie header", h.cfg.Auth.SessionCookieName)
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("expected session cookie to have HttpOnly flag")
	}

	// 5. Test GET /api/v1/auth/me WITH session cookie
	meResp, err := userClient.Get(h.server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /auth/me with cookie, got %d", meResp.StatusCode)
	}

	var meResult struct {
		Data struct {
			Phone string `json:"phone"`
		} `json:"data"`
	}
	if err := json.NewDecoder(meResp.Body).Decode(&meResult); err != nil {
		t.Fatalf("failed to parse /auth/me response: %v", err)
	}
	if meResult.Data.Phone != phone {
		t.Errorf("expected phone %s in /auth/me, got %s", phone, meResult.Data.Phone)
	}

	// 6. Test GET /api/v1/auth/me WITHOUT session cookie -> 401
	unauthResp, err := h.client.Get(h.server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("unauthenticated GET /auth/me failed: %v", err)
	}
	defer unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauthenticated /auth/me, got %d", unauthResp.StatusCode)
	}

	// 7. Test Logout: POST /api/v1/auth/logout
	logoutResp, err := userClient.Post(h.server.URL+"/api/v1/auth/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /auth/logout failed: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for logout, got %d", logoutResp.StatusCode)
	}

	// 8. Test GET /api/v1/auth/me AFTER logout -> 401
	postLogoutMe, err := userClient.Get(h.server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("post-logout /auth/me failed: %v", err)
	}
	defer postLogoutMe.Body.Close()
	if postLogoutMe.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized after logout, got %d", postLogoutMe.StatusCode)
	}
}

func TestHTTP_DiscoveryEndpoints(t *testing.T) {
	h := setupHTTPTest(t)

	pubName := fmt.Sprintf("Public Cafe %d", time.Now().UnixNano()%100000)
	draftName := fmt.Sprintf("Draft Cafe %d", time.Now().UnixNano()%100000)

	pubBiz := h.createBusiness(t, pubName, domain.PublicationStatusPublished)
	_ = h.createBusiness(t, draftName, domain.PublicationStatusUnpublished)

	// 1. GET /api/v1/businesses?neighborhood=...
	resp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses?neighborhood=%s", h.server.URL, pubBiz.Neighborhood))
	if err != nil {
		t.Fatalf("failed to query /businesses: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /businesses, got %d", resp.StatusCode)
	}

	var listResult struct {
		Data       []domain.Business `json:"data"`
		Pagination struct {
			Limit   int  `json:"limit"`
			Offset  int  `json:"offset"`
			HasMore bool `json:"has_more"`
		} `json:"pagination"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&listResult); err != nil {
		t.Fatalf("failed to decode /businesses JSON: %v", err)
	}

	foundPub := false
	for _, b := range listResult.Data {
		if b.ID == pubBiz.ID {
			foundPub = true
		}
		if b.PublicationStatus != domain.PublicationStatusPublished {
			t.Errorf("non-published business found in discovery: %s", b.Name)
		}
	}
	if !foundPub {
		t.Errorf("expected published business %s in list results", pubName)
	}

	// 2. GET /api/v1/businesses/{id}
	detailResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, pubBiz.ID))
	if err != nil {
		t.Fatalf("failed to get business profile: %v", err)
	}
	defer detailResp.Body.Close()
	if detailResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /businesses/{id}, got %d", detailResp.StatusCode)
	}

	var profileResult struct {
		Data struct {
			Business domain.Business `json:"business"`
		} `json:"data"`
	}
	if err := json.NewDecoder(detailResp.Body).Decode(&profileResult); err != nil {
		t.Fatalf("failed to decode business profile JSON: %v", err)
	}
	if profileResult.Data.Business.ID != pubBiz.ID {
		t.Errorf("expected business ID %d, got %d", pubBiz.ID, profileResult.Data.Business.ID)
	}

	// 3. GET /api/v1/slugs/{slug} and GET /api/v1/businesses/{slug}
	slugResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/slugs/%s", h.server.URL, pubBiz.Slug))
	if err != nil {
		t.Fatalf("failed to get business by slug: %v", err)
	}
	defer slugResp.Body.Close()
	if slugResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /slugs/{slug}, got %d", slugResp.StatusCode)
	}

	slugResp2, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%s", h.server.URL, pubBiz.Slug))
	if err != nil {
		t.Fatalf("failed to get business by slug via /businesses/{slug}: %v", err)
	}
	defer slugResp2.Body.Close()
	if slugResp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /businesses/{slug}, got %d", slugResp2.StatusCode)
	}

	// 4. GET /api/v1/search?q=...
	searchResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/search?q=%s", h.server.URL, url.QueryEscape(pubBiz.Name)))
	if err != nil {
		t.Fatalf("failed to search businesses: %v", err)
	}
	defer searchResp.Body.Close()
	if searchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /search, got %d", searchResp.StatusCode)
	}
}

func TestHTTP_CustomerInteractions(t *testing.T) {
	h := setupHTTPTest(t)

	phone := fmt.Sprintf("+251922%06d", time.Now().UnixNano()%1000000)
	client, _ := h.loginClient(t, phone, domain.RoleCustomer)

	biz := h.createBusiness(t, "Customer Target Cafe", domain.PublicationStatusPublished)

	// 1. Save business: POST /api/v1/saved/{businessID}
	saveResp, err := client.Post(fmt.Sprintf("%s/api/v1/saved/%d", h.server.URL, biz.ID), "application/json", nil)
	if err != nil {
		t.Fatalf("failed to save business: %v", err)
	}
	defer saveResp.Body.Close()
	if saveResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for save business, got %d", saveResp.StatusCode)
	}

	// 2. List saved: GET /api/v1/saved
	savedListResp, err := client.Get(h.server.URL + "/api/v1/saved")
	if err != nil {
		t.Fatalf("failed to get saved places: %v", err)
	}
	defer savedListResp.Body.Close()
	if savedListResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /saved, got %d", savedListResp.StatusCode)
	}

	var savedData struct {
		Data []domain.Business `json:"data"`
	}
	if err := json.NewDecoder(savedListResp.Body).Decode(&savedData); err != nil {
		t.Fatalf("failed to parse saved list: %v", err)
	}
	if len(savedData.Data) != 1 || savedData.Data[0].ID != biz.ID {
		t.Fatalf("expected 1 saved business with ID %d, got %v", biz.ID, savedData.Data)
	}

	// 3. Unsave: DELETE /api/v1/saved/{businessID}
	delReq, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/saved/%d", h.server.URL, biz.ID), nil)
	delResp, err := client.Do(delReq)
	if err != nil {
		t.Fatalf("failed to delete saved business: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for DELETE /saved/{id}, got %d", delResp.StatusCode)
	}

	// 4. Follow business: POST /api/v1/following/{businessID}
	followResp, err := client.Post(fmt.Sprintf("%s/api/v1/following/%d", h.server.URL, biz.ID), "application/json", nil)
	if err != nil {
		t.Fatalf("failed to follow business: %v", err)
	}
	defer followResp.Body.Close()
	if followResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for follow business, got %d", followResp.StatusCode)
	}

	// 5. Submit venue claim: POST /api/v1/claims
	claimReq, _ := json.Marshal(map[string]any{
		"business_id": biz.ID,
	})
	claimResp, err := client.Post(h.server.URL+"/api/v1/claims", "application/json", bytes.NewReader(claimReq))
	if err != nil {
		t.Fatalf("failed to submit claim: %v", err)
	}
	defer claimResp.Body.Close()
	if claimResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(claimResp.Body)
		t.Fatalf("expected 201 Created for claim submission, got %d: %s", claimResp.StatusCode, string(body))
	}

	// 6. Submit venue suggestion: POST /api/v1/submissions
	subReq, _ := json.Marshal(map[string]any{
		"business_name": "Discovered Coffee Roasters",
		"category":      "cafe",
		"area":          "Kazanchis",
		"payload":       map[string]any{"phone": "+251911999999"},
	})
	subResp, err := client.Post(h.server.URL+"/api/v1/submissions", "application/json", bytes.NewReader(subReq))
	if err != nil {
		t.Fatalf("failed to submit suggestion: %v", err)
	}
	defer subResp.Body.Close()
	if subResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(subResp.Body)
		t.Fatalf("expected 201 Created for venue suggestion, got %d: %s", subResp.StatusCode, string(body))
	}
}

func TestHTTP_BusinessManagementAndIDORProtection(t *testing.T) {
	h := setupHTTPTest(t)

	// Create 2 managers and 2 businesses
	phoneA := fmt.Sprintf("+251933%06d", time.Now().UnixNano()%1000000)
	clientA, userA := h.loginClient(t, phoneA, domain.RoleCustomer)

	phoneB := fmt.Sprintf("+251944%06d", time.Now().UnixNano()%1000000)
	clientB, _ := h.loginClient(t, phoneB, domain.RoleCustomer)

	bizA := h.createBusiness(t, "Manager A Venue", domain.PublicationStatusPublished)
	bizB := h.createBusiness(t, "Manager B Venue", domain.PublicationStatusPublished)

	// Assign Manager A to Venue A, and Manager B to Venue B
	ctx := context.Background()
	_ = h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizA.ID,
		UserID:      userA.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})

	// 1a. Manager A lists managed venues -> returns Venue A
	listReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/businesses/managed", h.server.URL), nil)
	listResp, err := clientA.Do(listReq)
	if err != nil {
		t.Fatalf("failed to list managed businesses: %v", err)
	}
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for list managed businesses, got %d", listResp.StatusCode)
	}
	var envelope struct {
		Data []domain.Business `json:"data"`
	}
	_ = json.NewDecoder(listResp.Body).Decode(&envelope)
	foundA := false
	for _, mb := range envelope.Data {
		if mb.ID == bizA.ID {
			foundA = true
			break
		}
	}
	if !foundA {
		t.Fatalf("expected to find Venue A in Manager A's managed list")
	}

	// 1b. Manager A gets management details for Venue A -> 200 OK
	mgmtReq, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/businesses/%d/manage", h.server.URL, bizA.ID), nil)
	mgmtResp, err := clientA.Do(mgmtReq)
	if err != nil {
		t.Fatalf("failed to get managed business: %v", err)
	}
	defer mgmtResp.Body.Close()
	if mgmtResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get managed business, got %d", mgmtResp.StatusCode)
	}

	// 1c. Manager A updates Venue A with Category and Cuisine -> 200 OK
	patchBody, _ := json.Marshal(map[string]any{
		"description": "Updated description by authorized Manager A",
		"category":    "cafe",
		"cuisine":     "Ethiopian Coffee",
	})
	patchReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, bizA.ID), bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	patchResp, err := clientA.Do(patchReq)
	if err != nil {
		t.Fatalf("Manager A update request failed: %v", err)
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(patchResp.Body)
		t.Fatalf("expected 200 OK for Manager A update, got %d: %s", patchResp.StatusCode, string(body))
	}

	// 1d. Invalid category update returns 400 Bad Request
	invalidPatchBody, _ := json.Marshal(map[string]any{
		"category": "invalid_cat",
	})
	invalidReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, bizA.ID), bytes.NewReader(invalidPatchBody))
	invalidReq.Header.Set("Content-Type", "application/json")
	invalidResp, err := clientA.Do(invalidReq)
	if err != nil {
		t.Fatalf("Manager A invalid update failed: %v", err)
	}
	defer invalidResp.Body.Close()
	if invalidResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid category, got %d", invalidResp.StatusCode)
	}

	// 2. IDOR PROTECTION: Manager B attempts to update Venue A -> 403 Forbidden
	patchReqIDOR, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, bizA.ID), bytes.NewReader(patchBody))
	patchReqIDOR.Header.Set("Content-Type", "application/json")
	respIDOR, err := clientB.Do(patchReqIDOR)
	if err != nil {
		t.Fatalf("Manager B IDOR request failed: %v", err)
	}
	defer respIDOR.Body.Close()
	if respIDOR.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B mutating Venue A, got %d", respIDOR.StatusCode)
	}

	// 2b. IDOR PROTECTION: Manager B attempts to get management profile for Venue A -> 403 Forbidden
	mgmtReqIDOR, _ := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v1/businesses/%d/manage", h.server.URL, bizA.ID), nil)
	mgmtRespIDOR, err := clientB.Do(mgmtReqIDOR)
	if err != nil {
		t.Fatalf("Manager B IDOR get failed: %v", err)
	}
	defer mgmtRespIDOR.Body.Close()
	if mgmtRespIDOR.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B viewing Venue A management, got %d", mgmtRespIDOR.StatusCode)
	}

	// 3. Manager A adds menu category to Venue A -> 201 Created
	catBody, _ := json.Marshal(map[string]any{
		"name": "Hot Beverages",
	})
	catResp, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizA.ID), "application/json", bytes.NewReader(catBody))
	if err != nil {
		t.Fatalf("failed to create menu category: %v", err)
	}
	defer catResp.Body.Close()
	if catResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(catResp.Body)
		t.Fatalf("expected 201 Created for menu category, got %d: %s", catResp.StatusCode, string(body))
	}

	// 4. IDOR PROTECTION: Manager B attempts to add menu category to Venue A -> 403 Forbidden
	catRespIDOR, err := clientB.Post(fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizA.ID), "application/json", bytes.NewReader(catBody))
	if err != nil {
		t.Fatalf("Manager B category IDOR failed: %v", err)
	}
	defer catRespIDOR.Body.Close()
	if catRespIDOR.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B adding category to Venue A, got %d", catRespIDOR.StatusCode)
	}

	_ = bizB
}

func TestHTTP_ReservationAndReviewWorkflow(t *testing.T) {
	h := setupHTTPTest(t)

	phoneCust := fmt.Sprintf("+251955%06d", time.Now().UnixNano()%1000000)
	clientCust, _ := h.loginClient(t, phoneCust, domain.RoleCustomer)

	phoneMgr := fmt.Sprintf("+251966%06d", time.Now().UnixNano()%1000000)
	clientMgr, userMgr := h.loginClient(t, phoneMgr, domain.RoleCustomer)

	biz := h.createBusiness(t, "Dining & Reviews Venue", domain.PublicationStatusPublished)

	ctx := context.Background()
	_ = h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  biz.ID,
		UserID:      userMgr.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	})

	// 1. Customer creates reservation: POST /api/v1/reservations
	resDate := time.Now().Add(24 * time.Hour)
	resBody, _ := json.Marshal(map[string]any{
		"business_id":      biz.ID,
		"customer_name":    "Test Customer",
		"customer_phone":   phoneCust,
		"reservation_date": resDate.Format("2006-01-02"),
		"reservation_time": "19:00:00",
		"party_size":       2,
	})
	resResp, err := clientCust.Post(h.server.URL+"/api/v1/reservations", "application/json", bytes.NewReader(resBody))
	if err != nil {
		t.Fatalf("failed to create reservation: %v", err)
	}
	defer resResp.Body.Close()
	if resResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resResp.Body)
		t.Fatalf("expected 201 Created for reservation, got %d: %s", resResp.StatusCode, string(body))
	}

	var createResResult struct {
		Data struct {
			ID domain.ReservationID `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resResp.Body).Decode(&createResResult); err != nil {
		t.Fatalf("failed to parse reservation response: %v", err)
	}
	reservationID := createResResult.Data.ID

	// 2. INVARIANT CHECK: Customer attempts review BEFORE completion -> 409 or 403 Forbidden
	reviewBody, _ := json.Marshal(map[string]any{
		"reservation_id": reservationID,
		"rating":         5,
		"review_text":    "Premature review attempt",
	})
	prematureResp, err := clientCust.Post(h.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(reviewBody))
	if err != nil {
		t.Fatalf("premature review request failed: %v", err)
	}
	defer prematureResp.Body.Close()
	if prematureResp.StatusCode == http.StatusCreated {
		t.Fatalf("expected review to fail before reservation is completed, but got 201 Created")
	}

	// 3. Manager confirms reservation: POST /api/v1/businesses/{id}/reservations/{resID}/confirm
	confirmResp, err := clientMgr.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/confirm", h.server.URL, biz.ID, reservationID), "application/json", nil)
	if err != nil {
		t.Fatalf("manager confirm reservation failed: %v", err)
	}
	defer confirmResp.Body.Close()
	if confirmResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(confirmResp.Body)
		t.Fatalf("expected 200 OK for confirming reservation, got %d: %s", confirmResp.StatusCode, string(body))
	}

	// 4. Manager completes reservation: POST /api/v1/businesses/{id}/reservations/{resID}/complete
	compResp, err := clientMgr.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/complete", h.server.URL, biz.ID, reservationID), "application/json", nil)
	if err != nil {
		t.Fatalf("manager complete reservation failed: %v", err)
	}
	defer compResp.Body.Close()
	if compResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(compResp.Body)
		t.Fatalf("expected 200 OK for completing reservation, got %d: %s", compResp.StatusCode, string(body))
	}

	// 5. Customer creates verified dining review: POST /api/v1/reviews -> 201 Created!
	validReviewBody, _ := json.Marshal(map[string]any{
		"reservation_id": reservationID,
		"rating":         5,
		"review_text":    "Outstanding hospitality and wonderful specialty coffee!",
	})
	revResp, err := clientCust.Post(h.server.URL+"/api/v1/reviews", "application/json", bytes.NewReader(validReviewBody))
	if err != nil {
		t.Fatalf("verified review creation failed: %v", err)
	}
	defer revResp.Body.Close()
	if revResp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(revResp.Body)
		t.Fatalf("expected 201 Created for verified review, got %d: %s", revResp.StatusCode, string(body))
	}

	var revResult struct {
		Data struct {
			ID domain.ReviewID `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(revResp.Body).Decode(&revResult); err != nil {
		t.Fatalf("failed to decode review response: %v", err)
	}

	// 6. Helpful vote on review: POST /api/v1/reviews/{id}/helpful
	voteResp, err := clientMgr.Post(fmt.Sprintf("%s/api/v1/reviews/%s/helpful", h.server.URL, revResult.Data.ID), "application/json", nil)
	if err != nil {
		t.Fatalf("failed to vote helpful on review: %v", err)
	}
	defer voteResp.Body.Close()
	if voteResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for helpful vote, got %d", voteResp.StatusCode)
	}
}

func TestHTTP_AdminModerationAndRBAC(t *testing.T) {
	h := setupHTTPTest(t)

	phoneCust := fmt.Sprintf("+251977%06d", time.Now().UnixNano()%1000000)
	clientCust, _ := h.loginClient(t, phoneCust, domain.RoleCustomer)

	phoneAdmin := fmt.Sprintf("+251988%06d", time.Now().UnixNano()%1000000)
	clientAdmin, _ := h.loginClient(t, phoneAdmin, domain.RoleAdmin)

	// 1. Regular customer attempts admin endpoint -> 403 Forbidden
	custSubResp, err := clientCust.Get(h.server.URL + "/api/v1/admin/submissions")
	if err != nil {
		t.Fatalf("customer access to admin failed: %v", err)
	}
	defer custSubResp.Body.Close()
	if custSubResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer calling admin API, got %d", custSubResp.StatusCode)
	}

	// 2. Admin successfully lists pending submissions -> 200 OK
	adminSubResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/submissions")
	if err != nil {
		t.Fatalf("admin listing submissions failed: %v", err)
	}
	defer adminSubResp.Body.Close()
	if adminSubResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for admin listing submissions, got %d", adminSubResp.StatusCode)
	}

	// 3. Admin lists audit log actions -> 200 OK
	actionsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/actions")
	if err != nil {
		t.Fatalf("admin listing actions failed: %v", err)
	}
	defer actionsResp.Body.Close()
	if actionsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for admin listing audit actions, got %d", actionsResp.StatusCode)
	}
}

func TestHTTP_ErrorEnvelopeAndMalformedRequests(t *testing.T) {
	h := setupHTTPTest(t)

	// 1. Malformed JSON payload to /api/v1/auth/otp/request
	resp, err := h.client.Post(h.server.URL+"/api/v1/auth/otp/request", "application/json", strings.NewReader("{invalid-json-structure"))
	if err != nil {
		t.Fatalf("malformed POST failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for malformed JSON, got %d", resp.StatusCode)
	}

	var errPayload struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&errPayload); err != nil {
		t.Fatalf("failed to decode error JSON envelope: %v", err)
	}
	if errPayload.Error.Code == "" || errPayload.Error.Message == "" {
		t.Fatalf("expected populated error code and message, got %+v", errPayload)
	}

	// 2. Nonexistent route -> 404
	notfoundResp, err := h.client.Get(h.server.URL + "/api/v1/nonexistent/endpoint")
	if err != nil {
		t.Fatalf("nonexistent endpoint GET failed: %v", err)
	}
	defer notfoundResp.Body.Close()
	if notfoundResp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404 Not Found, got %d", notfoundResp.StatusCode)
	}
}

func TestHTTP_UpdatesFeedAndBusinessReports(t *testing.T) {
	h := setupHTTPTest(t)
	custPhone := fmt.Sprintf("+251977%06d", time.Now().UnixNano()%1000000)
	custClient, _ := h.loginClient(t, custPhone, domain.RoleCustomer)
	biz := h.createBusiness(t, "Updates Venue", domain.PublicationStatusPublished)

	// 1. Public GET /api/v1/updates returns 200 OK
	resp, err := h.client.Get(h.server.URL + "/api/v1/updates")
	if err != nil {
		t.Fatalf("GET /api/v1/updates failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK from /api/v1/updates, got %d", resp.StatusCode)
	}

	// 2. Following updates unauthenticated -> 401
	unauthResp, err := h.client.Get(h.server.URL + "/api/v1/following/updates")
	if err != nil {
		t.Fatalf("GET /api/v1/following/updates failed: %v", err)
	}
	defer unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized, got %d", unauthResp.StatusCode)
	}

	// 3. Following updates authenticated -> 200 OK
	authResp, err := custClient.Get(h.server.URL + "/api/v1/following/updates")
	if err != nil {
		t.Fatalf("GET /api/v1/following/updates for customer failed: %v", err)
	}
	defer authResp.Body.Close()
	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", authResp.StatusCode)
	}

	// 4. Report business unauthenticated -> 401
	repPayload := map[string]string{
		"reason":  "WRONG_HOURS",
		"details": "They actually close at 9pm on Sundays.",
	}
	repJSON, _ := json.Marshal(repPayload)
	unauthRepResp, err := h.client.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/reports", h.server.URL, biz.ID),
		"application/json",
		bytes.NewReader(repJSON),
	)
	if err != nil {
		t.Fatalf("unauth POST report failed: %v", err)
	}
	defer unauthRepResp.Body.Close()
	if unauthRepResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 for unauth business report, got %d", unauthRepResp.StatusCode)
	}

	// 5. Report business authenticated -> 201 Created
	authRepResp, err := custClient.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/reports", h.server.URL, biz.ID),
		"application/json",
		bytes.NewReader(repJSON),
	)
	if err != nil {
		t.Fatalf("auth POST report failed: %v", err)
	}
	defer authRepResp.Body.Close()
	if authRepResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for customer business report, got %d", authRepResp.StatusCode)
	}
}

func TestMenuManagementHTTP(t *testing.T) {
	h := setupHTTPTest(t)
	ctx := context.Background()

	// Setup: Two businesses (Biz A and Biz B)
	bizA := h.createBusiness(t, "Menu Bistro A", domain.PublicationStatusPublished)
	bizB := h.createBusiness(t, "Menu Bistro B", domain.PublicationStatusPublished)

	// Users: Manager A for Biz A, Customer C with no management rights
	mgrPhone := fmt.Sprintf("+251977%06d", time.Now().UnixNano()%1000000)
	mgrClientA, mgrUserA := h.loginClient(t, mgrPhone, domain.RoleCustomer)

	custPhone := fmt.Sprintf("+251988%06d", time.Now().UnixNano()%1000000)
	custClient, _ := h.loginClient(t, custPhone, domain.RoleCustomer)

	// Assign Manager A to Biz A
	if err := h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizA.ID,
		UserID:      mgrUserA.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("failed to assign manager A to biz A: %v", err)
	}

	// 1. Public Menu Read (unauthenticated) -> 200 OK
	pubResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%d/menu", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("public GET menu failed: %v", err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for public menu, got %d", pubResp.StatusCode)
	}

	// 2. Scenario 1: Unauthorized customer cannot mutate Biz A menu (403 Forbidden)
	catPayload, _ := json.Marshal(map[string]any{"name": "Breakfast", "sort_order": 0})
	unauthPostCatResp, err := custClient.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(catPayload),
	)
	if err != nil {
		t.Fatalf("customer POST category failed: %v", err)
	}
	defer unauthPostCatResp.Body.Close()
	if unauthPostCatResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for unauthorized customer, got %d", unauthPostCatResp.StatusCode)
	}

	// 3. Scenario 2: Cross-Business Isolation: Manager of Biz A cannot mutate Biz B menu (403 Forbidden)
	crossBizResp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizB.ID),
		"application/json",
		bytes.NewReader(catPayload),
	)
	if err != nil {
		t.Fatalf("cross-business POST category failed: %v", err)
	}
	defer crossBizResp.Body.Close()
	if crossBizResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for manager touching unmanaged business, got %d", crossBizResp.StatusCode)
	}

	// 4. Scenario 3: Category Lifecycle on Biz A
	// 4a. Create Category 1: "Breakfast"
	postCat1Resp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(catPayload),
	)
	if err != nil {
		t.Fatalf("POST category 1 failed: %v", err)
	}
	defer postCat1Resp.Body.Close()
	if postCat1Resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(postCat1Resp.Body)
		t.Fatalf("expected 201 Created for category 1, got %d: %s", postCat1Resp.StatusCode, string(body))
	}
	var cat1Result struct {
		Data domain.MenuCategory `json:"data"`
	}
	if err := json.NewDecoder(postCat1Resp.Body).Decode(&cat1Result); err != nil {
		t.Fatalf("decode cat1 failed: %v", err)
	}
	cat1ID := cat1Result.Data.ID

	// 4b. Create Category 2: "Drinks"
	cat2Payload, _ := json.Marshal(map[string]any{"name": "Drinks", "sort_order": 1})
	postCat2Resp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(cat2Payload),
	)
	if err != nil {
		t.Fatalf("POST category 2 failed: %v", err)
	}
	defer postCat2Resp.Body.Close()
	if postCat2Resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for category 2, got %d", postCat2Resp.StatusCode)
	}
	var cat2Result struct {
		Data domain.MenuCategory `json:"data"`
	}
	if err := json.NewDecoder(postCat2Resp.Body).Decode(&cat2Result); err != nil {
		t.Fatalf("decode cat2 failed: %v", err)
	}
	cat2ID := cat2Result.Data.ID

	// 4c. Update Category 1: Rename to "Morning Specials"
	upCatPayload, _ := json.Marshal(map[string]any{"name": "Morning Specials"})
	reqUpCat, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories/%s", h.server.URL, bizA.ID, cat1ID),
		bytes.NewReader(upCatPayload),
	)
	reqUpCat.Header.Set("Content-Type", "application/json")
	upCatResp, err := mgrClientA.Do(reqUpCat)
	if err != nil {
		t.Fatalf("PATCH category failed: %v", err)
	}
	defer upCatResp.Body.Close()
	if upCatResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for update category, got %d", upCatResp.StatusCode)
	}

	// 4d. Reorder Categories: [cat2, cat1]
	reorderCatsPayload, _ := json.Marshal(map[string]any{
		"category_ids": []string{string(cat2ID), string(cat1ID)},
	})
	reqReorderCats, _ := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories/reorder", h.server.URL, bizA.ID),
		bytes.NewReader(reorderCatsPayload),
	)
	reqReorderCats.Header.Set("Content-Type", "application/json")
	reorderCatsResp, err := mgrClientA.Do(reqReorderCats)
	if err != nil {
		t.Fatalf("PUT reorder categories failed: %v", err)
	}
	defer reorderCatsResp.Body.Close()
	if reorderCatsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for reorder categories, got %d", reorderCatsResp.StatusCode)
	}

	// 5. Scenario 4 & 5: Item Lifecycle & Money Precision (Integer Santims)
	// 5a. Create Item 1: "Buna", 4500 Santims (ETB 45.00)
	item1Payload, _ := json.Marshal(map[string]any{
		"category_id":     cat2ID,
		"name":            "Buna",
		"description":     "Freshly roasted organic beans",
		"price_etb_minor": 4500,
		"is_available":    true,
		"sort_order":      0,
	})
	postItem1Resp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/items", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(item1Payload),
	)
	if err != nil {
		t.Fatalf("POST item 1 failed: %v", err)
	}
	defer postItem1Resp.Body.Close()
	if postItem1Resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(postItem1Resp.Body)
		t.Fatalf("expected 201 Created for item 1, got %d: %s", postItem1Resp.StatusCode, string(body))
	}
	var item1Result struct {
		Data domain.MenuItem `json:"data"`
	}
	if err := json.NewDecoder(postItem1Resp.Body).Decode(&item1Result); err != nil {
		t.Fatalf("decode item 1 failed: %v", err)
	}
	item1ID := item1Result.Data.ID
	if item1Result.Data.PriceETBMinor != 4500 {
		t.Fatalf("expected PriceETBMinor=4500, got %d", item1Result.Data.PriceETBMinor)
	}

	// 5b. Create Item 2: "Special Tea", 3550 Santims (ETB 35.50)
	item2Payload, _ := json.Marshal(map[string]any{
		"category_id":     cat2ID,
		"name":            "Special Tea",
		"description":     "Spiced herbal infusion",
		"price_etb_minor": 3550,
		"is_available":    true,
		"sort_order":      1,
	})
	postItem2Resp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/items", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(item2Payload),
	)
	if err != nil {
		t.Fatalf("POST item 2 failed: %v", err)
	}
	defer postItem2Resp.Body.Close()
	if postItem2Resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for item 2, got %d", postItem2Resp.StatusCode)
	}
	var item2Result struct {
		Data domain.MenuItem `json:"data"`
	}
	if err := json.NewDecoder(postItem2Resp.Body).Decode(&item2Result); err != nil {
		t.Fatalf("decode item 2 failed: %v", err)
	}
	item2ID := item2Result.Data.ID

	// 5c. Validation: Negative price rejected
	badPricePayload, _ := json.Marshal(map[string]any{
		"category_id":     cat2ID,
		"name":            "Invalid Item",
		"price_etb_minor": -500,
	})
	badPriceResp, err := mgrClientA.Post(
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/items", h.server.URL, bizA.ID),
		"application/json",
		bytes.NewReader(badPricePayload),
	)
	if err != nil {
		t.Fatalf("POST bad price item failed: %v", err)
	}
	defer badPriceResp.Body.Close()
	if badPriceResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for negative price, got %d", badPriceResp.StatusCode)
	}

	// 5d. Update Item 1: price change to 5000 Santims (ETB 50.00)
	upItemPayload, _ := json.Marshal(map[string]any{
		"price_etb_minor": 5000,
		"description":     "Single origin Yirgacheffe",
	})
	reqUpItem, _ := http.NewRequest(
		http.MethodPatch,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/items/%s", h.server.URL, bizA.ID, item1ID),
		bytes.NewReader(upItemPayload),
	)
	reqUpItem.Header.Set("Content-Type", "application/json")
	upItemResp, err := mgrClientA.Do(reqUpItem)
	if err != nil {
		t.Fatalf("PATCH item failed: %v", err)
	}
	defer upItemResp.Body.Close()
	if upItemResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for update item, got %d", upItemResp.StatusCode)
	}

	// 5e. Reorder Items in Category 2: [item2, item1]
	reorderItemsPayload, _ := json.Marshal(map[string]any{
		"item_ids": []string{string(item2ID), string(item1ID)},
	})
	reqReorderItems, _ := http.NewRequest(
		http.MethodPut,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories/%s/items/reorder", h.server.URL, bizA.ID, cat2ID),
		bytes.NewReader(reorderItemsPayload),
	)
	reqReorderItems.Header.Set("Content-Type", "application/json")
	reorderItemsResp, err := mgrClientA.Do(reqReorderItems)
	if err != nil {
		t.Fatalf("PUT reorder items failed: %v", err)
	}
	defer reorderItemsResp.Body.Close()
	if reorderItemsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for reorder items, got %d", reorderItemsResp.StatusCode)
	}

	// 6. Delete Item 2
	reqDelItem, _ := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/items/%s", h.server.URL, bizA.ID, item2ID),
		nil,
	)
	delItemResp, err := mgrClientA.Do(reqDelItem)
	if err != nil {
		t.Fatalf("DELETE item failed: %v", err)
	}
	defer delItemResp.Body.Close()
	if delItemResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete item, got %d", delItemResp.StatusCode)
	}

	// 7. Delete Category 1
	reqDelCat, _ := http.NewRequest(
		http.MethodDelete,
		fmt.Sprintf("%s/api/v1/businesses/%d/menu/categories/%s", h.server.URL, bizA.ID, cat1ID),
		nil,
	)
	delCatResp, err := mgrClientA.Do(reqDelCat)
	if err != nil {
		t.Fatalf("DELETE category failed: %v", err)
	}
	defer delCatResp.Body.Close()
	if delCatResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete category, got %d", delCatResp.StatusCode)
	}

	// 8. Verify Final Menu State via GET
	finalResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%d/menu", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("final GET menu failed: %v", err)
	}
	defer finalResp.Body.Close()
	if finalResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for final menu, got %d", finalResp.StatusCode)
	}
	var finalMenuResult struct {
		Data domain.BusinessMenu `json:"data"`
	}
	if err := json.NewDecoder(finalResp.Body).Decode(&finalMenuResult); err != nil {
		t.Fatalf("decode final menu failed: %v", err)
	}
	// Category 1 deleted, only Category 2 remains
	if len(finalMenuResult.Data.Categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(finalMenuResult.Data.Categories))
	}
	catRemaining := finalMenuResult.Data.Categories[0]
	if catRemaining.Category.ID != cat2ID {
		t.Fatalf("expected category %s, got %s", cat2ID, catRemaining.Category.ID)
	}
	// Item 2 deleted, only Item 1 remains
	if len(catRemaining.Items) != 1 {
		t.Fatalf("expected 1 item, got %d", len(catRemaining.Items))
	}
	itemRemaining := catRemaining.Items[0]
	if itemRemaining.ID != item1ID || itemRemaining.PriceETBMinor != 5000 {
		t.Fatalf("expected item %s with price 5000, got %s with price %d", item1ID, itemRemaining.ID, itemRemaining.PriceETBMinor)
	}
}

