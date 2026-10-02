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
	"github.com/Abdibaye/cafnet_backend/internal/storage"
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
	mediaRepo     repository.MediaRepository
	storageClient *storage.MemoryStorage
	mediaService  service.MediaService
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
		Storage: config.StorageConfig{
			Endpoint:               "http://localhost:9000",
			Region:                 "auto",
			Bucket:                 "test-bucket",
			AccessKey:              "test-access-key",
			SecretKey:              "test-secret-key",
			PublicURL:              "http://localhost:9000/test-bucket",
			PresignedURLExpiration: 10 * time.Minute,
			MaxUploadSizeBytes:     10 * 1024 * 1024,
			AllowedMimeTypes:       []string{"image/jpeg", "image/png", "image/webp"},
		},
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
	mediaRepo := repository.NewMediaRepository(pool)
	storageClient := storage.NewMemoryStorage("http://localhost:9000/test-bucket")

	authzService := service.NewAuthorizationService(ownershipRepo, businessRepo)
	ownershipWorkflow := service.NewOwnershipWorkflowService(
		authzService,
		claimRepo,
		subRepo,
		businessRepo,
		ownershipRepo,
		adminActionRepo,
		mediaRepo,
		storageClient,
		txMgr,
	)

	authService := service.NewAuthService(&cfg.Auth, userRepo, sessionRepo, otpRepo, txMgr, service.NewMockSMSProvider())
	discoveryService := service.NewDiscoveryService(businessRepo, menuRepo, updateRepo, reviewRepo, mediaRepo, storageClient)
	customerService := service.NewCustomerService(customerListRepo, businessRepo, claimRepo, subRepo, mediaRepo, storageClient)
	businessService := service.NewBusinessService(businessRepo, ownershipRepo, authzService, txMgr)
	menuService := service.NewMenuService(menuRepo, authzService, mediaRepo, storageClient, txMgr)
	updatesService := service.NewUpdatesService(updateRepo, authzService, customerListRepo)
	reservationService := service.NewReservationService(reservationRepo, businessRepo, authzService, txMgr)
	reviewService := service.NewReviewService(reviewRepo, reservationRepo, authzService, txMgr)
	reportService := service.NewReportService(reportRepo, reviewRepo, businessRepo)
	mediaService := service.NewMediaService(cfg.Storage, storageClient, mediaRepo, businessRepo, menuRepo, subRepo, authzService, txMgr)
	adminService := service.NewAdminService(
		ownershipWorkflow,
		authzService,
		subRepo,
		claimRepo,
		businessRepo,
		reviewRepo,
		reportRepo,
		adminActionRepo,
		mediaRepo,
		storageClient,
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
		Media:       handler.NewMediaHandler(mediaService),
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
		mediaRepo:     mediaRepo,
		storageClient: storageClient,
		mediaService:  mediaService,
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

func TestHTTP_AccountOnboardingAndProfile(t *testing.T) {
	h := setupHTTPTest(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)
	userClient, _ := h.loginClient(t, phone, domain.RoleCustomer)

	// 1. Initial GET /api/v1/auth/me should return 200
	meResp, err := userClient.Get(h.server.URL + "/api/v1/auth/me")
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", meResp.StatusCode)
	}

	// 2. Unauthenticated PATCH /api/v1/account/profile -> 401
	unauthBody := `{"full_name":"New Name"}`
	unauthReq, _ := http.NewRequest(http.MethodPatch, h.server.URL+"/api/v1/account/profile", strings.NewReader(unauthBody))
	unauthReq.Header.Set("Content-Type", "application/json")
	unauthResp, err := h.client.Do(unauthReq)
	if err != nil {
		t.Fatalf("unauth PATCH failed: %v", err)
	}
	defer unauthResp.Body.Close()
	if unauthResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for unauth profile update, got %d", unauthResp.StatusCode)
	}

	// 3. Authenticated PATCH with invalid data (too short) -> 400
	invalidBody := `{"full_name":"X"}`
	invReq, _ := http.NewRequest(http.MethodPatch, h.server.URL+"/api/v1/account/profile", strings.NewReader(invalidBody))
	invReq.Header.Set("Content-Type", "application/json")
	invReq.Header.Set("Origin", "http://localhost:3000")
	invResp, err := userClient.Do(invReq)
	if err != nil {
		t.Fatalf("inv PATCH failed: %v", err)
	}
	defer invResp.Body.Close()
	if invResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for short name, got %d", invResp.StatusCode)
	}

	// 4. Authenticated PATCH with valid data -> 200 OK
	validBody := `{"full_name":"Abebe Bikila", "avatar_url":"https://images.cafnet.et/abebe.jpg"}`
	validReq, _ := http.NewRequest(http.MethodPatch, h.server.URL+"/api/v1/account/profile", strings.NewReader(validBody))
	validReq.Header.Set("Content-Type", "application/json")
	validReq.Header.Set("Origin", "http://localhost:3000")
	validResp, err := userClient.Do(validReq)
	if err != nil {
		t.Fatalf("valid PATCH failed: %v", err)
	}
	defer validResp.Body.Close()
	if validResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid profile update, got %d", validResp.StatusCode)
	}

	var patchResult struct {
		Data struct {
			ID               string `json:"id"`
			FullName         string `json:"full_name"`
			AvatarURL        string `json:"avatar_url"`
			ProfileCompleted bool   `json:"profile_completed"`
			Role             string `json:"role"`
		} `json:"data"`
	}
	if err := json.NewDecoder(validResp.Body).Decode(&patchResult); err != nil {
		t.Fatalf("failed to decode patch result: %v", err)
	}
	if patchResult.Data.FullName != "Abebe Bikila" {
		t.Errorf("expected FullName 'Abebe Bikila', got %s", patchResult.Data.FullName)
	}
	if !patchResult.Data.ProfileCompleted {
		t.Errorf("expected ProfileCompleted true after patch")
	}
	if patchResult.Data.Role != "CUSTOMER" {
		t.Errorf("role changed! expected CUSTOMER, got %s", patchResult.Data.Role)
	}

	// 5. GET /api/v1/account/profile returns updated data
	accResp, err := userClient.Get(h.server.URL + "/api/v1/account/profile")
	if err != nil {
		t.Fatalf("GET /account/profile failed: %v", err)
	}
	defer accResp.Body.Close()
	if accResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for GET /account/profile, got %d", accResp.StatusCode)
	}

	var accResult struct {
		Data struct {
			FullName         string `json:"full_name"`
			ProfileCompleted bool   `json:"profile_completed"`
		} `json:"data"`
	}
	_ = json.NewDecoder(accResp.Body).Decode(&accResult)
	if accResult.Data.FullName != "Abebe Bikila" {
		t.Errorf("expected Abebe Bikila from GET /account/profile, got %s", accResult.Data.FullName)
	}
}

func TestHTTP_EmailPasswordAuthLifecycle(t *testing.T) {
	h := setupHTTPTest(t)
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("http-user-%s@cafnet.et", uniqueSuffix)
	password := "SecurePassword123"
	name := "Bilen Alazar"

	// 1. Invalid input: password too short -> 400 Bad Request
	shortReqBody, _ := json.Marshal(map[string]string{
		"name":     name,
		"email":    email,
		"password": "short",
	})
	shortResp, err := h.client.Post(h.server.URL+"/api/v1/auth/signup", "application/json", strings.NewReader(string(shortReqBody)))
	if err != nil {
		t.Fatalf("POST /auth/signup short password failed: %v", err)
	}
	defer shortResp.Body.Close()
	if shortResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d", shortResp.StatusCode)
	}

	// 2. Successful Signup -> 201 Created + Cookie
	signUpBody, _ := json.Marshal(map[string]string{
		"name":     name,
		"email":    email,
		"password": password,
	})
	signUpReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/auth/signup", strings.NewReader(string(signUpBody)))
	signUpReq.Header.Set("Content-Type", "application/json")
	signUpReq.Header.Set("Origin", "http://localhost:3000")

	// Use client with cookie jar
	client := &http.Client{
		Jar: h.client.Jar,
	}

	signUpResp, err := client.Do(signUpReq)
	if err != nil {
		t.Fatalf("POST /auth/signup failed: %v", err)
	}
	defer signUpResp.Body.Close()
	if signUpResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d", signUpResp.StatusCode)
	}

	// Check cookie
	cookies := signUpResp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "cafnet_session" {
			sessionCookie = c
			break
		}
	}
	if sessionCookie == nil || sessionCookie.Value == "" {
		t.Fatalf("expected cafnet_session cookie in signup response")
	}
	if !sessionCookie.HttpOnly {
		t.Errorf("expected session cookie to be HttpOnly")
	}

	// Check body
	var signUpRes struct {
		Data struct {
			User struct {
				ID               string  `json:"id"`
				FullName         string  `json:"full_name"`
				Email            *string `json:"email"`
				Role             string  `json:"role"`
				ProfileCompleted bool    `json:"profile_completed"`
			} `json:"user"`
		} `json:"data"`
	}
	if err := json.NewDecoder(signUpResp.Body).Decode(&signUpRes); err != nil {
		t.Fatalf("failed to decode signup response: %v", err)
	}
	if signUpRes.Data.User.FullName != name {
		t.Errorf("expected FullName %s, got %s", name, signUpRes.Data.User.FullName)
	}
	if signUpRes.Data.User.Email == nil || *signUpRes.Data.User.Email != email {
		t.Errorf("expected email %s, got %v", email, signUpRes.Data.User.Email)
	}
	if signUpRes.Data.User.Role != "CUSTOMER" {
		t.Errorf("expected role CUSTOMER, got %s", signUpRes.Data.User.Role)
	}
	if !signUpRes.Data.User.ProfileCompleted {
		t.Errorf("expected ProfileCompleted true")
	}

	// 3. Duplicate email signup -> 409 Conflict
	dupReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/auth/signup", strings.NewReader(string(signUpBody)))
	dupReq.Header.Set("Content-Type", "application/json")
	dupReq.Header.Set("Origin", "http://localhost:3000")
	dupResp, err := client.Do(dupReq)
	if err != nil {
		t.Fatalf("POST duplicate signup failed: %v", err)
	}
	defer dupResp.Body.Close()
	if dupResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate email, got %d", dupResp.StatusCode)
	}

	// 4. Authenticated GET /api/v1/auth/me returns user
	meReq, _ := http.NewRequest(http.MethodGet, h.server.URL+"/api/v1/auth/me", nil)
	meReq.Header.Set("Origin", "http://localhost:3000")
	meResp, err := client.Do(meReq)
	if err != nil {
		t.Fatalf("GET /auth/me failed: %v", err)
	}
	defer meResp.Body.Close()
	if meResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for /auth/me, got %d", meResp.StatusCode)
	}

	// 5. Logout
	logoutReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/auth/logout", nil)
	logoutReq.Header.Set("Origin", "http://localhost:3000")
	logoutResp, err := client.Do(logoutReq)
	if err != nil {
		t.Fatalf("POST /auth/logout failed: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for logout, got %d", logoutResp.StatusCode)
	}

	// 6. Sign in with wrong password -> 401 Unauthorized
	wrongPassBody, _ := json.Marshal(map[string]string{
		"email":    email,
		"password": "WrongPassword999",
	})
	wpReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/auth/signin", strings.NewReader(string(wrongPassBody)))
	wpReq.Header.Set("Content-Type", "application/json")
	wpReq.Header.Set("Origin", "http://localhost:3000")
	wpResp, err := client.Do(wpReq)
	if err != nil {
		t.Fatalf("POST /auth/signin wrong pass failed: %v", err)
	}
	defer wpResp.Body.Close()
	if wpResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401 Unauthorized for wrong password, got %d", wpResp.StatusCode)
	}

	// 7. Sign in with valid credentials -> 200 OK
	signInBody, _ := json.Marshal(map[string]string{
		"email":    fmt.Sprintf("  %s  ", strings.ToUpper(email)), // test normalization
		"password": password,
	})
	signInReq, _ := http.NewRequest(http.MethodPost, h.server.URL+"/api/v1/auth/signin", strings.NewReader(string(signInBody)))
	signInReq.Header.Set("Content-Type", "application/json")
	signInReq.Header.Set("Origin", "http://localhost:3000")
	signInResp, err := client.Do(signInReq)
	if err != nil {
		t.Fatalf("POST /auth/signin failed: %v", err)
	}
	defer signInResp.Body.Close()
	if signInResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for signin, got %d", signInResp.StatusCode)
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

	// 6. Submit venue suggestion: upload cover first, then POST /api/v1/submissions
	upReq, _ := json.Marshal(map[string]any{
		"target_type":  "submission",
		"content_type": "image/jpeg",
		"file_size":    1024,
	})
	upResp, _ := client.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(upReq))
	var upData struct {
		Data struct {
			MediaID string `json:"media_id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(upResp.Body).Decode(&upData)
	upResp.Body.Close()
	compResp, _ := client.Post(h.server.URL+"/api/v1/media/uploads/"+upData.Data.MediaID+"/complete", "application/json", nil)
	compResp.Body.Close()

	subReq, _ := json.Marshal(map[string]any{
		"business_name": fmt.Sprintf("Discovered Coffee Roasters %d", time.Now().UnixNano()),
		"category":      "cafe",
		"area":          "Kazanchis",
		"media_id":      upData.Data.MediaID,
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

func TestHTTP_BusinessUpdatesManagement(t *testing.T) {
	h := setupHTTPTest(t)
	ctx := context.Background()

	// Setup Business A with Manager A
	bizA := h.createBusiness(t, "Updates Venue A", domain.PublicationStatusPublished)
	phoneA := fmt.Sprintf("+251911%06d", time.Now().UnixNano()%1000000)
	clientA, userA := h.loginClient(t, phoneA, domain.RoleCustomer)
	if err := h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizA.ID,
		UserID:      userA.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("failed to assign manager A to biz A: %v", err)
	}

	// Setup Business B with Manager B
	bizB := h.createBusiness(t, "Updates Venue B", domain.PublicationStatusPublished)
	phoneB := fmt.Sprintf("+251922%06d", time.Now().UnixNano()%1000000)
	clientB, userB := h.loginClient(t, phoneB, domain.RoleCustomer)
	if err := h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizB.ID,
		UserID:      userB.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("failed to assign manager B to biz B: %v", err)
	}

	// Setup Customer C (unauthorized)
	phoneC := fmt.Sprintf("+251933%06d", time.Now().UnixNano()%1000000)
	clientC, _ := h.loginClient(t, phoneC, domain.RoleCustomer)

	// 1. Unauthorized customer cannot create update for Business A (403)
	payload := map[string]any{
		"update_type": "announcement",
		"title":       "Customer fake update",
		"description": "Should fail",
	}
	body, _ := json.Marshal(payload)
	resp, err := clientC.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("clientC create update failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer creating update, got %d", resp.StatusCode)
	}

	// 2. Manager B cannot create update for Business A (403 - cross business)
	respB, err := clientB.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("clientB create update failed: %v", err)
	}
	defer respB.Body.Close()
	if respB.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B creating update on Business A, got %d", respB.StatusCode)
	}

	// 3. Manager A cannot create update with invalid type (400)
	invalidTypePayload, _ := json.Marshal(map[string]any{
		"update_type": "invalid_type",
		"title":       "Invalid type title",
		"description": "Some description",
	})
	respInvType, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(invalidTypePayload))
	if err != nil {
		t.Fatalf("clientA create invalid type update failed: %v", err)
	}
	defer respInvType.Body.Close()
	if respInvType.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for invalid type, got %d", respInvType.StatusCode)
	}

	// 4. Manager A cannot create offer with past expiration date (400)
	pastOfferPayload, _ := json.Marshal(map[string]any{
		"update_type":   "offer",
		"title":         "Expired discount",
		"description":   "Discount was yesterday",
		"offer_details": "50% off",
		"valid_until":   "2020-01-01",
	})
	respPastOffer, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(pastOfferPayload))
	if err != nil {
		t.Fatalf("clientA create past offer failed: %v", err)
	}
	defer respPastOffer.Body.Close()
	if respPastOffer.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for past offer expiration, got %d", respPastOffer.StatusCode)
	}

	// 5. Manager A creates all 5 types successfully
	// 5a. Offer with valid future expiration
	futureOfferPayload, _ := json.Marshal(map[string]any{
		"update_type":   "offer",
		"title":         "Weekend 20% Off Brunch",
		"description":   "Enjoy 20% off all brunch items this weekend.",
		"offer_details": "20% off brunch",
		"valid_until":   "2030-12-31",
	})
	respOffer, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(futureOfferPayload))
	if err != nil {
		t.Fatalf("create offer failed: %v", err)
	}
	defer respOffer.Body.Close()
	if respOffer.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for offer, got %d", respOffer.StatusCode)
	}
	var offerCreated struct {
		Data domain.BusinessUpdate `json:"data"`
	}
	if err := json.NewDecoder(respOffer.Body).Decode(&offerCreated); err != nil {
		t.Fatalf("failed to decode offer response: %v", err)
	}
	if offerCreated.Data.ExpiresAt == nil {
		t.Fatalf("expected derived expires_at for offer, got nil")
	}

	// 5b. New Menu
	newMenuPayload, _ := json.Marshal(map[string]any{
		"update_type": "new_menu",
		"title":       "Summer Pastry Collection",
		"description": "Freshly baked croissants, tarts, and local honey cakes.",
	})
	respNewMenu, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(newMenuPayload))
	if err != nil {
		t.Fatalf("create new_menu failed: %v", err)
	}
	defer respNewMenu.Body.Close()
	if respNewMenu.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for new_menu, got %d", respNewMenu.StatusCode)
	}

	// 5c. Event
	eventPayload, _ := json.Marshal(map[string]any{
		"update_type": "event",
		"title":       "Coffee Cupping & Tasting Session",
		"description": "Join our master baristas for a single-origin cupping workshop.",
		"event_date":  "2030-05-15",
		"event_time":  "4:00 PM",
	})
	respEvent, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(eventPayload))
	if err != nil {
		t.Fatalf("create event failed: %v", err)
	}
	defer respEvent.Body.Close()
	if respEvent.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for event, got %d", respEvent.StatusCode)
	}

	// 5d. Price Update
	pricePayload, _ := json.Marshal(map[string]any{
		"update_type": "price_update",
		"title":       "Revised Beverage Pricing",
		"description": "Updated pricing for cold brews and seasonal fruit smoothies.",
		"price_text":  "Starting from 120 ETB",
	})
	respPrice, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(pricePayload))
	if err != nil {
		t.Fatalf("create price_update failed: %v", err)
	}
	defer respPrice.Body.Close()
	if respPrice.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for price_update, got %d", respPrice.StatusCode)
	}

	// 5e. Announcement
	annPayload, _ := json.Marshal(map[string]any{
		"update_type": "announcement",
		"title":       "Patio Renovation Complete",
		"description": "Our garden patio is fully open with expanded seating and Wi-Fi.",
	})
	respAnn, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID), "application/json", bytes.NewReader(annPayload))
	if err != nil {
		t.Fatalf("create announcement failed: %v", err)
	}
	defer respAnn.Body.Close()
	if respAnn.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for announcement, got %d", respAnn.StatusCode)
	}
	var annCreated struct {
		Data domain.BusinessUpdate `json:"data"`
	}
	if err := json.NewDecoder(respAnn.Body).Decode(&annCreated); err != nil {
		t.Fatalf("failed to decode announcement response: %v", err)
	}
	annID := annCreated.Data.ID

	// 6. Cross-business update attack (Manager B attempts to modify Manager A's announcement) -> 403
	patchPayload, _ := json.Marshal(map[string]any{
		"title": "Hacked title by Manager B",
	})
	patchReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d/updates/%s", h.server.URL, bizA.ID, annID), bytes.NewReader(patchPayload))
	patchReq.Header.Set("Content-Type", "application/json")
	respPatchB, err := clientB.Do(patchReq)
	if err != nil {
		t.Fatalf("Manager B patch attempt failed: %v", err)
	}
	defer respPatchB.Body.Close()
	if respPatchB.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B patching Business A update, got %d", respPatchB.StatusCode)
	}

	// 7. IDOR attack: Manager B uses own business URL but passes Manager A's update ID -> 403
	idorReq, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d/updates/%s", h.server.URL, bizB.ID, annID), bytes.NewReader(patchPayload))
	idorReq.Header.Set("Content-Type", "application/json")
	respIDOR, err := clientB.Do(idorReq)
	if err != nil {
		t.Fatalf("IDOR patch attempt failed: %v", err)
	}
	defer respIDOR.Body.Close()
	if respIDOR.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for cross-business IDOR patch, got %d", respIDOR.StatusCode)
	}

	// 8. Manager A successfully modifies announcement
	validPatch, _ := json.Marshal(map[string]any{
		"title":       "Garden Patio Officially Open!",
		"description": "Updated description with new seating photos available.",
	})
	patchReqA, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d/updates/%s", h.server.URL, bizA.ID, annID), bytes.NewReader(validPatch))
	patchReqA.Header.Set("Content-Type", "application/json")
	respPatchA, err := clientA.Do(patchReqA)
	if err != nil {
		t.Fatalf("Manager A patch failed: %v", err)
	}
	defer respPatchA.Body.Close()
	if respPatchA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for Manager A patch, got %d", respPatchA.StatusCode)
	}
	var patchedResult struct {
		Data domain.BusinessUpdate `json:"data"`
	}
	if err := json.NewDecoder(respPatchA.Body).Decode(&patchedResult); err != nil {
		t.Fatalf("failed to decode patched result: %v", err)
	}
	if patchedResult.Data.Title != "Garden Patio Officially Open!" {
		t.Fatalf("expected updated title, got %s", patchedResult.Data.Title)
	}

	// 9. List Business Updates via Manager A (with ?include_expired=true)
	listRespA, err := clientA.Get(fmt.Sprintf("%s/api/v1/businesses/%d/updates?include_expired=true", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("Manager A list updates failed: %v", err)
	}
	defer listRespA.Body.Close()
	if listRespA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for Manager A list, got %d", listRespA.StatusCode)
	}
	var listResultA struct {
		Data []domain.BusinessUpdate `json:"data"`
	}
	if err := json.NewDecoder(listRespA.Body).Decode(&listResultA); err != nil {
		t.Fatalf("failed to decode list result: %v", err)
	}
	if len(listResultA.Data) != 5 {
		t.Fatalf("expected 5 updates in list, got %d", len(listResultA.Data))
	}

	// 10. Manager B cannot delete Manager A's update (403)
	delReqB, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/businesses/%d/updates/%s", h.server.URL, bizA.ID, annID), nil)
	respDelB, err := clientB.Do(delReqB)
	if err != nil {
		t.Fatalf("Manager B delete attempt failed: %v", err)
	}
	defer respDelB.Body.Close()
	if respDelB.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for Manager B deleting Business A update, got %d", respDelB.StatusCode)
	}

	// 11. Manager A deletes announcement update -> 200 OK
	delReqA, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/api/v1/businesses/%d/updates/%s", h.server.URL, bizA.ID, annID), nil)
	respDelA, err := clientA.Do(delReqA)
	if err != nil {
		t.Fatalf("Manager A delete failed: %v", err)
	}
	defer respDelA.Body.Close()
	if respDelA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for delete update, got %d", respDelA.StatusCode)
	}

	// 12. Verify public view no longer shows the deleted announcement
	publicResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%d/updates", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("public get updates failed: %v", err)
	}
	defer publicResp.Body.Close()
	if publicResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for public get updates, got %d", publicResp.StatusCode)
	}
	var publicResult struct {
		Data []domain.BusinessUpdate `json:"data"`
	}
	if err := json.NewDecoder(publicResp.Body).Decode(&publicResult); err != nil {
		t.Fatalf("failed to decode public updates: %v", err)
	}
	if len(publicResult.Data) != 4 {
		t.Fatalf("expected 4 active updates after deletion, got %d", len(publicResult.Data))
	}
	for _, u := range publicResult.Data {
		if u.ID == annID {
			t.Fatalf("deleted announcement %s should not appear in active public list", annID)
		}
	}
}

func TestHTTP_BusinessReservationManagement(t *testing.T) {
	h := setupHTTPTest(t)
	ctx := context.Background()

	// 1. Setup Business A and Manager A
	bizA := h.createBusiness(t, "Reservation Venue A", domain.PublicationStatusPublished)
	phoneA := fmt.Sprintf("+251911%06d", time.Now().UnixNano()%1000000)
	clientA, userA := h.loginClient(t, phoneA, domain.RoleCustomer)
	if err := h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizA.ID,
		UserID:      userA.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("failed to assign manager A to biz A: %v", err)
	}

	// 2. Setup Business B and Manager B
	bizB := h.createBusiness(t, "Reservation Venue B", domain.PublicationStatusPublished)
	phoneB := fmt.Sprintf("+251922%06d", time.Now().UnixNano()%1000000)
	clientB, userB := h.loginClient(t, phoneB, domain.RoleCustomer)
	if err := h.ownershipRepo.AddManager(ctx, &domain.BusinessManager{
		BusinessID:  bizB.ID,
		UserID:      userB.ID,
		ManagerRole: domain.ManagerRolePrimaryOwner,
	}); err != nil {
		t.Fatalf("failed to assign manager B to biz B: %v", err)
	}

	// 3. Setup Customer C
	phoneC := fmt.Sprintf("+251933%06d", time.Now().UnixNano()%1000000)
	clientC, _ := h.loginClient(t, phoneC, domain.RoleCustomer)

	// 4. Customer C creates reservation RA on Business A
	resDate := time.Now().Add(24 * time.Hour).Format("2006-01-02")
	resPayloadA, _ := json.Marshal(map[string]any{
		"business_id":      bizA.ID,
		"customer_name":    "Customer C",
		"customer_phone":   phoneC,
		"reservation_date": resDate,
		"reservation_time": "19:00:00",
		"party_size":       4,
		"special_request":  "Window table please",
	})
	resRespA, err := clientC.Post(h.server.URL+"/api/v1/reservations", "application/json", bytes.NewReader(resPayloadA))
	if err != nil {
		t.Fatalf("failed to create reservation RA: %v", err)
	}
	defer resRespA.Body.Close()
	if resRespA.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for reservation RA, got %d", resRespA.StatusCode)
	}
	var createdResA struct {
		Data domain.Reservation `json:"data"`
	}
	if err := json.NewDecoder(resRespA.Body).Decode(&createdResA); err != nil {
		t.Fatalf("failed to decode reservation RA: %v", err)
	}
	resAID := createdResA.Data.ID

	// 5. Customer C creates reservation RB on Business B
	resPayloadB, _ := json.Marshal(map[string]any{
		"business_id":      bizB.ID,
		"customer_name":    "Customer C",
		"customer_phone":   phoneC,
		"reservation_date": resDate,
		"reservation_time": "20:00:00",
		"party_size":       2,
	})
	resRespB, err := clientC.Post(h.server.URL+"/api/v1/reservations", "application/json", bytes.NewReader(resPayloadB))
	if err != nil {
		t.Fatalf("failed to create reservation RB: %v", err)
	}
	defer resRespB.Body.Close()
	if resRespB.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for reservation RB, got %d", resRespB.StatusCode)
	}

	// 6. Non-manager Customer C tries to list Business A reservations -> 403 Forbidden
	listUnauth, err := clientC.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reservations", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("unauth list failed: %v", err)
	}
	defer listUnauth.Body.Close()
	if listUnauth.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer listing manager reservations, got %d", listUnauth.StatusCode)
	}

	// 7. Manager B tries to list Business A reservations -> 403 Forbidden (cross-business)
	listCross, err := clientB.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reservations", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("cross-business list failed: %v", err)
	}
	defer listCross.Body.Close()
	if listCross.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for manager B listing biz A reservations, got %d", listCross.StatusCode)
	}

	// 8. Manager A lists Business A reservations -> 200 OK
	listA, err := clientA.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reservations", h.server.URL, bizA.ID))
	if err != nil {
		t.Fatalf("manager A list failed: %v", err)
	}
	defer listA.Body.Close()
	if listA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for manager A listing biz A reservations, got %d", listA.StatusCode)
	}
	var listResA struct {
		Data []domain.Reservation `json:"data"`
	}
	if err := json.NewDecoder(listA.Body).Decode(&listResA); err != nil {
		t.Fatalf("failed to decode list response: %v", err)
	}
	if len(listResA.Data) != 1 || listResA.Data[0].ID != resAID {
		t.Fatalf("expected 1 reservation with id %s, got %d", resAID, len(listResA.Data))
	}

	// 9. Manager B tries to GET reservation RA directly -> 403 Forbidden
	getCross, err := clientB.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s", h.server.URL, bizA.ID, resAID))
	if err != nil {
		t.Fatalf("cross-business get failed: %v", err)
	}
	defer getCross.Body.Close()
	if getCross.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for manager B getting biz A reservation, got %d", getCross.StatusCode)
	}

	// 10. IDOR attack: Manager B uses own business URL but passes RA's ID -> 403 Forbidden
	getIdor, err := clientB.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s", h.server.URL, bizB.ID, resAID))
	if err != nil {
		t.Fatalf("idor get failed: %v", err)
	}
	defer getIdor.Body.Close()
	if getIdor.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for IDOR cross-business reservation get, got %d", getIdor.StatusCode)
	}

	// 11. Manager B tries to confirm RA -> 403 Forbidden
	confirmCross, err := clientB.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/confirm", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("cross confirm failed: %v", err)
	}
	defer confirmCross.Body.Close()
	if confirmCross.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for manager B confirming biz A reservation, got %d", confirmCross.StatusCode)
	}

	// 12. IDOR attack: Manager B tries to confirm RA via biz B URL -> 403 Forbidden
	confirmIdor, err := clientB.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/confirm", h.server.URL, bizB.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("idor confirm failed: %v", err)
	}
	defer confirmIdor.Body.Close()
	if confirmIdor.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for IDOR reservation confirmation, got %d", confirmIdor.StatusCode)
	}

	// 13. State machine test: Manager A tries to COMPLETE RA while it is still PENDING -> 409 Conflict
	prematureComp, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/complete", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("premature complete failed: %v", err)
	}
	defer prematureComp.Body.Close()
	if prematureComp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for completing PENDING reservation, got %d", prematureComp.StatusCode)
	}

	// 14. Manager A successfully confirms RA -> 200 OK
	confirmA, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/confirm", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("manager A confirm failed: %v", err)
	}
	defer confirmA.Body.Close()
	if confirmA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for confirming reservation RA, got %d", confirmA.StatusCode)
	}

	// 15. State machine test: Cannot CONFIRM already CONFIRMED reservation -> 409 Conflict
	reconfirm, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/confirm", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("reconfirm failed: %v", err)
	}
	defer reconfirm.Body.Close()
	if reconfirm.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for re-confirming reservation, got %d", reconfirm.StatusCode)
	}

	// 16. State machine test: Cannot DECLINE a CONFIRMED reservation -> 409 Conflict
	declineConfirmedPayload, _ := json.Marshal(map[string]any{"reason": "Changed our minds"})
	declineConfirmed, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/decline", h.server.URL, bizA.ID, resAID), "application/json", bytes.NewReader(declineConfirmedPayload))
	if err != nil {
		t.Fatalf("decline confirmed failed: %v", err)
	}
	defer declineConfirmed.Body.Close()
	if declineConfirmed.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for declining CONFIRMED reservation, got %d", declineConfirmed.StatusCode)
	}

	// 17. Manager A marks confirmed reservation as COMPLETED -> 200 OK
	completeA, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/complete", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("manager A complete failed: %v", err)
	}
	defer completeA.Body.Close()
	if completeA.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for completing reservation RA, got %d", completeA.StatusCode)
	}

	// 18. State machine test: Cannot complete COMPLETED reservation again -> 409 Conflict
	recomplete, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/complete", h.server.URL, bizA.ID, resAID), "application/json", nil)
	if err != nil {
		t.Fatalf("recomplete failed: %v", err)
	}
	defer recomplete.Body.Close()
	if recomplete.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for re-completing completed reservation, got %d", recomplete.StatusCode)
	}

	// 19. Customer C creates another reservation RA2 for testing decline workflow
	resRespA2, err := clientC.Post(h.server.URL+"/api/v1/reservations", "application/json", bytes.NewReader(resPayloadA))
	if err != nil {
		t.Fatalf("create reservation RA2 failed: %v", err)
	}
	defer resRespA2.Body.Close()
	var createdResA2 struct {
		Data domain.Reservation `json:"data"`
	}
	if err := json.NewDecoder(resRespA2.Body).Decode(&createdResA2); err != nil {
		t.Fatalf("failed to decode RA2: %v", err)
	}
	resA2ID := createdResA2.Data.ID

	// 20. Decline with empty reason fails with 400 Bad Request
	emptyDeclinePayload, _ := json.Marshal(map[string]any{"reason": "   "})
	emptyDeclineResp, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/decline", h.server.URL, bizA.ID, resA2ID), "application/json", bytes.NewReader(emptyDeclinePayload))
	if err != nil {
		t.Fatalf("empty decline request failed: %v", err)
	}
	defer emptyDeclineResp.Body.Close()
	if emptyDeclineResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 Bad Request for empty decline reason, got %d", emptyDeclineResp.StatusCode)
	}

	// 21. Decline with valid reason succeeds -> 200 OK
	validDeclinePayload, _ := json.Marshal(map[string]any{"reason": "Fully booked at this hour"})
	validDeclineResp, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/decline", h.server.URL, bizA.ID, resA2ID), "application/json", bytes.NewReader(validDeclinePayload))
	if err != nil {
		t.Fatalf("valid decline request failed: %v", err)
	}
	defer validDeclineResp.Body.Close()
	if validDeclineResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid decline, got %d", validDeclineResp.StatusCode)
	}

	// 22. Customer C verifies reservation status is DECLINED with reason
	custGetA2, err := clientC.Get(fmt.Sprintf("%s/api/v1/reservations/%s", h.server.URL, resA2ID))
	if err != nil {
		t.Fatalf("customer get RA2 failed: %v", err)
	}
	defer custGetA2.Body.Close()
	if custGetA2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for customer get RA2, got %d", custGetA2.StatusCode)
	}
	var custA2Result struct {
		Data domain.Reservation `json:"data"`
	}
	if err := json.NewDecoder(custGetA2.Body).Decode(&custA2Result); err != nil {
		t.Fatalf("failed to decode cust RA2: %v", err)
	}
	if custA2Result.Data.Status != domain.ReservationStatusDeclined {
		t.Fatalf("expected DECLINED status, got %s", custA2Result.Data.Status)
	}
	if custA2Result.Data.DeclineReason == nil || *custA2Result.Data.DeclineReason != "Fully booked at this hour" {
		t.Fatalf("expected decline reason 'Fully booked at this hour', got %v", custA2Result.Data.DeclineReason)
	}

	// 23. State machine test: Cannot COMPLETE a DECLINED reservation -> 409 Conflict
	compDeclined, err := clientA.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reservations/%s/complete", h.server.URL, bizA.ID, resA2ID), "application/json", nil)
	if err != nil {
		t.Fatalf("compDeclined request failed: %v", err)
	}
	defer compDeclined.Body.Close()
	if compDeclined.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for completing DECLINED reservation, got %d", compDeclined.StatusCode)
	}
}

func TestHTTP_AdminCenterCompleteWorkflow(t *testing.T) {
	h := setupHTTPTest(t)
	defer h.server.Close()

	// 1. Setup actors:
	// - customerA (normal customer)
	// - customerB (another customer / submitter)
	// - adminUser (platform administrator)
	// - adminSelfApprover (admin who also submits a claim/submission to test self-approval blocks)
	phoneCustA := uniquePhone()
	clientCustA, userCustA := h.loginClient(t, phoneCustA, domain.RoleCustomer)

	phoneCustB := uniquePhone()
	clientCustB, userCustB := h.loginClient(t, phoneCustB, domain.RoleCustomer)

	phoneAdmin := uniquePhone()
	clientAdmin, userAdmin := h.loginClient(t, phoneAdmin, domain.RoleAdmin)

	phoneSelfAdmin := uniquePhone()
	clientSelfAdmin, userSelfAdmin := h.loginClient(t, phoneSelfAdmin, domain.RoleAdmin)

	_ = userCustA
	_ = userCustB
	_ = userAdmin
	_ = userSelfAdmin

	// 2. Security Enforcement: Normal customer cannot access admin endpoints -> 403 Forbidden
	custStatsResp, err := clientCustA.Get(h.server.URL + "/api/v1/admin/stats")
	if err != nil {
		t.Fatalf("customer stats request failed: %v", err)
	}
	defer custStatsResp.Body.Close()
	if custStatsResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer accessing admin stats, got %d", custStatsResp.StatusCode)
	}

	custSubsResp, err := clientCustA.Get(h.server.URL + "/api/v1/admin/submissions")
	if err != nil {
		t.Fatalf("customer submissions request failed: %v", err)
	}
	defer custSubsResp.Body.Close()
	if custSubsResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for customer accessing admin submissions, got %d", custSubsResp.StatusCode)
	}

	// 3. Admin gets stats -> 200 OK
	adminStatsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/stats")
	if err != nil {
		t.Fatalf("admin stats request failed: %v", err)
	}
	defer adminStatsResp.Body.Close()
	if adminStatsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for admin stats, got %d", adminStatsResp.StatusCode)
	}
	var statsResult struct {
		Data domain.AdminStats `json:"data"`
	}
	if err := json.NewDecoder(adminStatsResp.Body).Decode(&statsResult); err != nil {
		t.Fatalf("failed to decode stats: %v", err)
	}

	// 4. Submissions Workflow:
	// A. Customer B creates a submission
	subUpReq, _ := json.Marshal(map[string]any{
		"target_type":  "submission",
		"content_type": "image/jpeg",
		"file_size":    1024,
	})
	subUpResp, _ := clientCustB.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(subUpReq))
	var subUpData struct {
		Data struct {
			MediaID string `json:"media_id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(subUpResp.Body).Decode(&subUpData)
	subUpResp.Body.Close()
	compSubResp, _ := clientCustB.Post(h.server.URL+"/api/v1/media/uploads/"+subUpData.Data.MediaID+"/complete", "application/json", nil)
	compSubResp.Body.Close()

	subPayload, _ := json.Marshal(map[string]any{
		"business_name": fmt.Sprintf("Addis Highland Cafe %d", time.Now().UnixNano()),
		"category":      "cafe",
		"area":          "Bole",
		"media_id":      subUpData.Data.MediaID,
		"payload": map[string]any{
			"description": "Artisan coffee and pastries in Bole",
		},
	})
	subResp, err := clientCustB.Post(h.server.URL+"/api/v1/submissions", "application/json", bytes.NewReader(subPayload))
	if err != nil {
		t.Fatalf("create submission failed: %v", err)
	}
	defer subResp.Body.Close()
	if subResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for submission, got %d", subResp.StatusCode)
	}
	var subResult struct {
		Data domain.BusinessSubmission `json:"data"`
	}
	if err := json.NewDecoder(subResp.Body).Decode(&subResult); err != nil {
		t.Fatalf("failed to decode submission: %v", err)
	}
	subID := subResult.Data.ID

	// B. Admin lists submissions and retrieves by ID
	listSubsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/submissions?status=PENDING")
	if err != nil {
		t.Fatalf("list submissions failed: %v", err)
	}
	defer listSubsResp.Body.Close()
	if listSubsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for list submissions, got %d", listSubsResp.StatusCode)
	}

	getSubResp, err := clientAdmin.Get(fmt.Sprintf("%s/api/v1/admin/submissions/%s", h.server.URL, subID))
	if err != nil {
		t.Fatalf("get submission failed: %v", err)
	}
	defer getSubResp.Body.Close()
	if getSubResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get submission, got %d", getSubResp.StatusCode)
	}

	// C. Self-approval protection: If submitter user is also admin, attempting to approve their own submission returns 403 Forbidden
	selfSubUpReq, _ := json.Marshal(map[string]any{
		"target_type":  "submission",
		"content_type": "image/jpeg",
		"file_size":    1024,
	})
	selfSubUpResp, _ := clientSelfAdmin.Post(h.server.URL+"/api/v1/media/uploads", "application/json", bytes.NewReader(selfSubUpReq))
	var selfSubUpData struct {
		Data struct {
			MediaID string `json:"media_id"`
		} `json:"data"`
	}
	_ = json.NewDecoder(selfSubUpResp.Body).Decode(&selfSubUpData)
	selfSubUpResp.Body.Close()
	compSelfSubResp, _ := clientSelfAdmin.Post(h.server.URL+"/api/v1/media/uploads/"+selfSubUpData.Data.MediaID+"/complete", "application/json", nil)
	compSelfSubResp.Body.Close()

	selfSubPayload, _ := json.Marshal(map[string]any{
		"business_name": fmt.Sprintf("Self Admin Bistro %d", time.Now().UnixNano()),
		"category":      "restaurant",
		"area":          "Piazza",
		"media_id":      selfSubUpData.Data.MediaID,
	})
	selfSubResp, err := clientSelfAdmin.Post(h.server.URL+"/api/v1/submissions", "application/json", bytes.NewReader(selfSubPayload))
	if err != nil {
		t.Fatalf("create self submission failed: %v", err)
	}
	defer selfSubResp.Body.Close()
	var selfSubResult struct {
		Data domain.BusinessSubmission `json:"data"`
	}
	_ = json.NewDecoder(selfSubResp.Body).Decode(&selfSubResult)
	selfSubID := selfSubResult.Data.ID

	selfApproveResp, err := clientSelfAdmin.Post(fmt.Sprintf("%s/api/v1/admin/submissions/%s/approve", h.server.URL, selfSubID), "application/json", nil)
	if err != nil {
		t.Fatalf("self approve request failed: %v", err)
	}
	defer selfApproveResp.Body.Close()
	if selfApproveResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 Forbidden for self-approval, got %d", selfApproveResp.StatusCode)
	}

	// D. Reject submission with reason
	rejectPayload, _ := json.Marshal(map[string]string{"reason": "Incomplete contact details provided"})
	rejectResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/submissions/%s/reject", h.server.URL, selfSubID), "application/json", bytes.NewReader(rejectPayload))
	if err != nil {
		t.Fatalf("reject submission request failed: %v", err)
	}
	defer rejectResp.Body.Close()
	if rejectResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for reject submission, got %d", rejectResp.StatusCode)
	}

	// E. Admin approves Customer B's submission -> creates business & assigns Customer B as PRIMARY_OWNER
	approveResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/submissions/%s/approve", h.server.URL, subID), "application/json", nil)
	if err != nil {
		t.Fatalf("approve submission request failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for approve submission, got %d", approveResp.StatusCode)
	}
	var approvedBizResult struct {
		Data domain.Business `json:"data"`
	}
	if err := json.NewDecoder(approveResp.Body).Decode(&approvedBizResult); err != nil {
		t.Fatalf("failed to decode approved business: %v", err)
	}
	newBizID := approvedBizResult.Data.ID

	// Customer B can now manage the created business
	custBMgmt, err := clientCustB.Get(fmt.Sprintf("%s/api/v1/businesses/%d/manage", h.server.URL, newBizID))
	if err != nil {
		t.Fatalf("customer B manage request failed: %v", err)
	}
	defer custBMgmt.Body.Close()
	if custBMgmt.StatusCode != http.StatusOK {
		t.Fatalf("expected Customer B to have management access after submission approval, got %d", custBMgmt.StatusCode)
	}

	// 5. Claims Workflow:
	// A. Seed an unclaimed business
	unclaimedBiz := &domain.Business{
		Name:                  uniqueSlug("Old Town Bakery"),
		Slug:                  uniqueSlug("old-town-bakery"),
		Category:              domain.CategoryCafe,
		PriceRange:            domain.PriceRangeBudget,
		PublicationStatus:     domain.PublicationStatusPublished,
		VerificationStatus:    domain.VerificationStatusUnverified,
		ClaimStatus:           domain.ClaimStatusUnclaimed,
		ReservationAvailable:  true,
		LocationName:          "Kazanchis",
		Neighborhood:          "Kazanchis",
	}
	if err := h.businessRepo.Create(context.Background(), unclaimedBiz); err != nil {
		t.Fatalf("failed to seed unclaimed business: %v", err)
	}

	// B. Customer A claims Old Town Bakery
	claimPayload, _ := json.Marshal(map[string]any{"business_id": unclaimedBiz.ID})
	claimResp, err := clientCustA.Post(h.server.URL+"/api/v1/claims", "application/json", bytes.NewReader(claimPayload))
	if err != nil {
		t.Fatalf("create claim failed: %v", err)
	}
	defer claimResp.Body.Close()
	if claimResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for claim, got %d", claimResp.StatusCode)
	}
	var claimResult struct {
		Data domain.BusinessClaim `json:"data"`
	}
	if err := json.NewDecoder(claimResp.Body).Decode(&claimResult); err != nil {
		t.Fatalf("failed to decode claim: %v", err)
	}
	claimID := claimResult.Data.ID

	// C. Admin lists claims and gets claim detail
	listClaimsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/claims?status=PENDING")
	if err != nil {
		t.Fatalf("list claims failed: %v", err)
	}
	defer listClaimsResp.Body.Close()
	if listClaimsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for list claims, got %d", listClaimsResp.StatusCode)
	}

	getClaimResp, err := clientAdmin.Get(fmt.Sprintf("%s/api/v1/admin/claims/%s", h.server.URL, claimID))
	if err != nil {
		t.Fatalf("get claim failed: %v", err)
	}
	defer getClaimResp.Body.Close()
	if getClaimResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get claim, got %d", getClaimResp.StatusCode)
	}

	// D. Admin approves claim
	approveClaimPayload, _ := json.Marshal(map[string]string{"notes": "Verified business license #8821"})
	approveClaimResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/claims/%s/approve", h.server.URL, claimID), "application/json", bytes.NewReader(approveClaimPayload))
	if err != nil {
		t.Fatalf("approve claim request failed: %v", err)
	}
	defer approveClaimResp.Body.Close()
	if approveClaimResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for approve claim, got %d", approveClaimResp.StatusCode)
	}

	// Customer A is now authorized manager of Old Town Bakery
	custAMgmt, err := clientCustA.Get(fmt.Sprintf("%s/api/v1/businesses/%d/manage", h.server.URL, unclaimedBiz.ID))
	if err != nil {
		t.Fatalf("customer A manage request failed: %v", err)
	}
	defer custAMgmt.Body.Close()
	if custAMgmt.StatusCode != http.StatusOK {
		t.Fatalf("expected Customer A to have management rights after claim approval, got %d", custAMgmt.StatusCode)
	}

	// 6. Review Moderation & Aggregate Recalculation:
	// A. Customer creates a confirmed visit and leaves a 1-star review
	res := &domain.Reservation{
		ReservationCode: fmt.Sprintf("CAF-%d", time.Now().UnixNano()%100000),
		BusinessID:      unclaimedBiz.ID,
		CustomerUserID:  userCustB.ID,
		CustomerName:    "Customer B",
		CustomerPhone:   phoneCustB,
		ReservationDate: time.Now().AddDate(0, 0, 1),
		ReservationTime: "19:00",
		PartySize:       2,
		Status:          domain.ReservationStatusCompleted,
	}
	if err := h.resRepo.Create(context.Background(), res); err != nil {
		t.Fatalf("failed to create completed reservation: %v", err)
	}
	rev := &domain.Review{
		BusinessID:      unclaimedBiz.ID,
		CustomerUserID:  userCustB.ID,
		ReservationID:   res.ID,
		Rating:          1,
		ReviewText:      "Terrible abusive language and false allegations",
		IsVerifiedVisit: true,
		Status:          domain.ReviewStatusVisible,
	}
	if err := h.reviewRepo.Create(context.Background(), rev); err != nil {
		t.Fatalf("failed to create review: %v", err)
	}

	// Verify initial review summary includes this review
	sumResp1, err := clientCustA.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reviews/summary", h.server.URL, unclaimedBiz.ID))
	if err != nil {
		t.Fatalf("get summary failed: %v", err)
	}
	defer sumResp1.Body.Close()
	var sumResult1 struct {
		Data domain.ReviewSummary `json:"data"`
	}
	_ = json.NewDecoder(sumResp1.Body).Decode(&sumResult1)
	if sumResult1.Data.Count != 1 || sumResult1.Data.Average != 1.0 {
		t.Fatalf("expected 1 review with 1.0 average, got count=%d avg=%f", sumResult1.Data.Count, sumResult1.Data.Average)
	}

	// B. Customer A reports the review
	repPayload, _ := json.Marshal(map[string]string{
		"reason":      "offensive",
		"explanation": "Review contains abusive defamation",
	})
	repResp, err := clientCustA.Post(fmt.Sprintf("%s/api/v1/reviews/%s/reports", h.server.URL, rev.ID), "application/json", bytes.NewReader(repPayload))
	if err != nil {
		t.Fatalf("report review failed: %v", err)
	}
	defer repResp.Body.Close()
	if repResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for review report, got %d", repResp.StatusCode)
	}
	var repResult struct {
		Data domain.ReviewReport `json:"data"`
	}
	_ = json.NewDecoder(repResp.Body).Decode(&repResult)
	reviewReportID := repResult.Data.ID

	// C. Admin inspects the review report and review details
	adminRepResp, err := clientAdmin.Get(fmt.Sprintf("%s/api/v1/admin/reviews/reports/%s", h.server.URL, reviewReportID))
	if err != nil {
		t.Fatalf("get review report failed: %v", err)
	}
	defer adminRepResp.Body.Close()
	if adminRepResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get review report, got %d", adminRepResp.StatusCode)
	}

	adminRevResp, err := clientAdmin.Get(fmt.Sprintf("%s/api/v1/admin/reviews/%s", h.server.URL, rev.ID))
	if err != nil {
		t.Fatalf("get review detail failed: %v", err)
	}
	defer adminRevResp.Body.Close()
	if adminRevResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for admin review inspection, got %d", adminRevResp.StatusCode)
	}

	// D. Admin moderates review -> status = REMOVED_BY_ADMIN
	modPayload, _ := json.Marshal(map[string]string{
		"status": "REMOVED_BY_ADMIN",
		"reason": "Violates community policy on profanity and harassment",
	})
	modResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/reviews/%s/moderate", h.server.URL, rev.ID), "application/json", bytes.NewReader(modPayload))
	if err != nil {
		t.Fatalf("moderate review failed: %v", err)
	}
	defer modResp.Body.Close()
	if modResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for review moderation, got %d", modResp.StatusCode)
	}

	// E. Resolve report -> ACTIONED
	resRepPayload, _ := json.Marshal(map[string]string{
		"status": "ACTIONED",
		"action": "Removed review for terms violation",
	})
	resRepResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/reviews/reports/%s/resolve", h.server.URL, reviewReportID), "application/json", bytes.NewReader(resRepPayload))
	if err != nil {
		t.Fatalf("resolve review report failed: %v", err)
	}
	defer resRepResp.Body.Close()
	if resRepResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for resolve review report, got %d", resRepResp.StatusCode)
	}

	// F. Verify dynamically recalculated summary: Review is no longer visible and count is 0!
	sumResp2, err := clientCustA.Get(fmt.Sprintf("%s/api/v1/businesses/%d/reviews/summary", h.server.URL, unclaimedBiz.ID))
	if err != nil {
		t.Fatalf("get summary after moderation failed: %v", err)
	}
	defer sumResp2.Body.Close()
	var sumResult2 struct {
		Data domain.ReviewSummary `json:"data"`
	}
	_ = json.NewDecoder(sumResp2.Body).Decode(&sumResult2)
	if sumResult2.Data.Count != 0 || sumResult2.Data.Average != 0.0 {
		t.Fatalf("expected 0 reviews and 0.0 average after removal, got count=%d avg=%f", sumResult2.Data.Count, sumResult2.Data.Average)
	}

	// 7. Business Reports Workflow:
	// A. Customer reports business information inaccuracy
	bizRepPayload, _ := json.Marshal(map[string]string{
		"reason":  "WRONG_HOURS",
		"details": "They actually close at 9pm on Sundays",
	})
	bizRepResp, err := clientCustB.Post(fmt.Sprintf("%s/api/v1/businesses/%d/reports", h.server.URL, unclaimedBiz.ID), "application/json", bytes.NewReader(bizRepPayload))
	if err != nil {
		t.Fatalf("report business failed: %v", err)
	}
	defer bizRepResp.Body.Close()
	if bizRepResp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201 Created for business report, got %d", bizRepResp.StatusCode)
	}
	var bizRepResult struct {
		Data domain.BusinessReport `json:"data"`
	}
	_ = json.NewDecoder(bizRepResp.Body).Decode(&bizRepResult)
	bizReportID := bizRepResult.Data.ID

	// B. Admin lists business reports and retrieves by ID
	listBizRepsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/business-reports?status=PENDING")
	if err != nil {
		t.Fatalf("list business reports failed: %v", err)
	}
	defer listBizRepsResp.Body.Close()
	if listBizRepsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for list business reports, got %d", listBizRepsResp.StatusCode)
	}

	getBizRepResp, err := clientAdmin.Get(fmt.Sprintf("%s/api/v1/admin/business-reports/%s", h.server.URL, bizReportID))
	if err != nil {
		t.Fatalf("get business report failed: %v", err)
	}
	defer getBizRepResp.Body.Close()
	if getBizRepResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for get business report, got %d", getBizRepResp.StatusCode)
	}

	// C. Admin resolves business report
	resolveBizRepPayload, _ := json.Marshal(map[string]string{
		"status": "ACTIONED",
		"notes":  "Notified owner to update Sunday closing time",
	})
	resolveBizRepResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/business-reports/%s/resolve", h.server.URL, bizReportID), "application/json", bytes.NewReader(resolveBizRepPayload))
	if err != nil {
		t.Fatalf("resolve business report failed: %v", err)
	}
	defer resolveBizRepResp.Body.Close()
	if resolveBizRepResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for resolve business report, got %d", resolveBizRepResp.StatusCode)
	}

	// 8. Business Badging & Publication Status:
	// A. Admin sets business verification
	verPayload, _ := json.Marshal(map[string]string{"status": "VERIFIED"})
	verResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/businesses/%d/verification", h.server.URL, unclaimedBiz.ID), "application/json", bytes.NewReader(verPayload))
	if err != nil {
		t.Fatalf("set verification failed: %v", err)
	}
	defer verResp.Body.Close()
	if verResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for set verification, got %d", verResp.StatusCode)
	}

	// B. Admin sets business publication
	pubPayload, _ := json.Marshal(map[string]string{"status": "UNPUBLISHED"})
	pubResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/businesses/%d/publication", h.server.URL, unclaimedBiz.ID), "application/json", bytes.NewReader(pubPayload))
	if err != nil {
		t.Fatalf("set publication failed: %v", err)
	}
	defer pubResp.Body.Close()
	if pubResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for set publication, got %d", pubResp.StatusCode)
	}

	// 9. Audit History:
	actionsResp, err := clientAdmin.Get(h.server.URL + "/api/v1/admin/actions")
	if err != nil {
		t.Fatalf("get admin actions failed: %v", err)
	}
	defer actionsResp.Body.Close()
	if actionsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for admin actions, got %d", actionsResp.StatusCode)
	}
	var actionsResult struct {
		Data []domain.AdminAction `json:"data"`
	}
	if err := json.NewDecoder(actionsResp.Body).Decode(&actionsResult); err != nil {
		t.Fatalf("failed to decode admin actions: %v", err)
	}
	if len(actionsResult.Data) == 0 {
		t.Fatalf("expected admin actions to be recorded, got 0")
	}
}

func TestHTTP_RealBusinessLifecycle(t *testing.T) {
	h := setupHTTPTest(t)
	defer h.server.Close()

	// 1. Setup actors
	phoneCust := uniquePhone()
	clientCust, userCust := h.loginClient(t, phoneCust, domain.RoleCustomer)
	_ = userCust

	phoneAdmin := uniquePhone()
	clientAdmin, userAdmin := h.loginClient(t, phoneAdmin, domain.RoleAdmin)
	_ = userAdmin

	// 2. User submits business via POST /api/v1/businesses/submissions
	uniqueBizName := fmt.Sprintf("B20 Artisan Cafe %d", time.Now().UnixNano())
	subPayload, _ := json.Marshal(map[string]any{
		"business_name": uniqueBizName,
		"category":      "cafe",
		"area":          "Bole Japan",
		"payload": map[string]any{
			"description":   "Authentic Ethiopian specialty coffee house",
			"contact_phone": "+251911998877",
		},
	})

	subResp, err := clientCust.Post(h.server.URL+"/api/v1/businesses/submissions", "application/json", bytes.NewReader(subPayload))
	if err != nil {
		t.Fatalf("submit business failed: %v", err)
	}
	defer subResp.Body.Close()
	if subResp.StatusCode != http.StatusCreated {
		respBytes, _ := io.ReadAll(subResp.Body)
		t.Fatalf("expected 201 Created for submission alias, got %d: %s", subResp.StatusCode, string(respBytes))
	}

	var subResult struct {
		Data domain.BusinessSubmission `json:"data"`
	}
	if err := json.NewDecoder(subResp.Body).Decode(&subResult); err != nil {
		t.Fatalf("decode submission failed: %v", err)
	}
	subID := subResult.Data.ID
	if subResult.Data.Status != domain.SubmissionStatusPending {
		t.Fatalf("expected status PENDING, got %s", subResult.Data.Status)
	}

	// 3. Duplicate submission while pending returns 409 Conflict
	dupResp, err := clientCust.Post(h.server.URL+"/api/v1/businesses/submissions", "application/json", bytes.NewReader(subPayload))
	if err != nil {
		t.Fatalf("duplicate submission request failed: %v", err)
	}
	defer dupResp.Body.Close()
	if dupResp.StatusCode != http.StatusConflict {
		t.Fatalf("expected 409 Conflict for duplicate pending submission, got %d", dupResp.StatusCode)
	}

	// 4. Submitting user can see their pending submission via GET /api/v1/businesses/submissions
	mySubsResp, err := clientCust.Get(h.server.URL + "/api/v1/businesses/submissions")
	if err != nil {
		t.Fatalf("get my submissions failed: %v", err)
	}
	defer mySubsResp.Body.Close()
	if mySubsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for my submissions, got %d", mySubsResp.StatusCode)
	}
	var mySubsResult struct {
		Data []domain.BusinessSubmission `json:"data"`
	}
	if err := json.NewDecoder(mySubsResp.Body).Decode(&mySubsResult); err != nil {
		t.Fatalf("decode my submissions failed: %v", err)
	}
	foundSub := false
	for _, s := range mySubsResult.Data {
		if s.ID == subID {
			foundSub = true
			break
		}
	}
	if !foundSub {
		t.Fatalf("expected submitted business in user's submissions")
	}

	// 5. Admin approves submission
	approveResp, err := clientAdmin.Post(fmt.Sprintf("%s/api/v1/admin/submissions/%s/approve", h.server.URL, subID), "application/json", nil)
	if err != nil {
		t.Fatalf("admin approve request failed: %v", err)
	}
	defer approveResp.Body.Close()
	if approveResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for approve submission, got %d", approveResp.StatusCode)
	}
	var approvedResult struct {
		Data domain.Business `json:"data"`
	}
	if err := json.NewDecoder(approveResp.Body).Decode(&approvedResult); err != nil {
		t.Fatalf("decode approved business failed: %v", err)
	}
	realBizID := approvedResult.Data.ID

	// 6. Submitting user now sees business in Business Center via GET /api/v1/business-center/managed
	bcResp, err := clientCust.Get(h.server.URL + "/api/v1/business-center/managed")
	if err != nil {
		t.Fatalf("get business center managed failed: %v", err)
	}
	defer bcResp.Body.Close()
	if bcResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for business-center/managed, got %d", bcResp.StatusCode)
	}
	var bcResult struct {
		Data []domain.Business `json:"data"`
	}
	if err := json.NewDecoder(bcResp.Body).Decode(&bcResult); err != nil {
		t.Fatalf("decode business-center/managed failed: %v", err)
	}
	foundInBC := false
	for _, b := range bcResult.Data {
		if b.ID == realBizID {
			foundInBC = true
			if b.ClaimStatus != domain.ClaimStatusClaimed {
				t.Fatalf("expected business claim status CLAIMED, got %s", b.ClaimStatus)
			}
			break
		}
	}
	if !foundInBC {
		t.Fatalf("approved business not found in Business Center managed list for submitter")
	}

	// 7. Submitting user can update profile via PATCH /api/v1/businesses/{id}
	updatePayload, _ := json.Marshal(map[string]any{
		"description": "Updated description by verified business owner",
		"price_range": "$$$",
	})
	req, _ := http.NewRequest(http.MethodPatch, fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, realBizID), bytes.NewReader(updatePayload))
	req.Header.Set("Content-Type", "application/json")
	patchResp, err := clientCust.Do(req)
	if err != nil {
		t.Fatalf("patch business failed: %v", err)
	}
	defer patchResp.Body.Close()
	if patchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for patch business, got %d", patchResp.StatusCode)
	}

	// 8. Public Discovery reflects the exact same business
	publicResp, err := h.client.Get(fmt.Sprintf("%s/api/v1/businesses/%d", h.server.URL, realBizID))
	if err != nil {
		t.Fatalf("public get business failed: %v", err)
	}
	defer publicResp.Body.Close()
	if publicResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for public business profile, got %d", publicResp.StatusCode)
	}
	var publicResult struct {
		Data service.PublicBusinessProfile `json:"data"`
	}
	if err := json.NewDecoder(publicResp.Body).Decode(&publicResult); err != nil {
		t.Fatalf("decode public profile failed: %v", err)
	}
	if publicResult.Data.Business.ID != realBizID {
		t.Fatalf("expected public profile ID %d, got %d", realBizID, publicResult.Data.Business.ID)
	}
	if publicResult.Data.Business.Description != "Updated description by verified business owner" {
		t.Fatalf("expected updated description, got %s", publicResult.Data.Business.Description)
	}
}

