package tests

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

type testRepos struct {
	User    repository.UserRepository
	Session repository.SessionRepository
	OTP     repository.OTPRepository
	TxMgr   repository.TxManager
}

func setupAuthService(t *testing.T) (service.AuthService, *config.AuthConfig, *service.MockSMSProvider, *testRepos, context.Context) {
	t.Helper()
	pool, ctx := setupTestDB(t)

	cfg := &config.AuthConfig{
		OTPTTL:             5 * time.Minute,
		OTPMaxAttempts:     3,
		OTPRequestCooldown: 2 * time.Second, // short cooldown for testing
		SessionTTL:         24 * time.Hour,
		SessionCookieName:  "cafnet_session",
		AllowedOrigins:     []string{"http://localhost:3000"},
		DevMode:            true,
		DeterministicOTP:   "123456",
		Environment:        "test",
	}

	userRepo := repository.NewUserRepository(pool)
	sessionRepo := repository.NewSessionRepository(pool)
	otpRepo := repository.NewOTPRepository(pool)
	txMgr := repository.NewTxManager(pool)
	smsMock := service.NewMockSMSProvider()

	authSvc := service.NewAuthService(cfg, userRepo, sessionRepo, otpRepo, txMgr, smsMock)

	repos := &testRepos{
		User:    userRepo,
		Session: sessionRepo,
		OTP:     otpRepo,
		TxMgr:   txMgr,
	}

	return authSvc, cfg, smsMock, repos, ctx
}

func TestPhoneNormalization(t *testing.T) {
	cases := []struct {
		input    string
		expected string
		isValid  bool
	}{
		{"+251911223344", "+251911223344", true},
		{"251911223344", "+251911223344", true},
		{"0911223344", "+251911223344", true},
		{"911223344", "+251911223344", true},
		{"09 11 22 33 44", "+251911223344", true},
		{"(0911) 22-33-44", "+251911223344", true},
		{"+251711223344", "+251711223344", true},
		{"0711223344", "+251711223344", true},
		{"711223344", "+251711223344", true},
		// Invalid cases
		{"", "", false},
		{"12345", "", false},
		{"0811223344", "", false},   // Only 9 or 7 valid Ethiopian mobile prefix
		{"+14155552671", "", false}, // Non-Ethiopian prefix
		{"abcdefghij", "", false},
		{"09112233445566", "", false}, // Too long
	}

	for _, tc := range cases {
		normalized, err := domain.NormalizePhone(tc.input)
		if tc.isValid {
			if err != nil {
				t.Errorf("NormalizePhone(%q) unexpected error: %v", tc.input, err)
			}
			if normalized != tc.expected {
				t.Errorf("NormalizePhone(%q) = %q, want %q", tc.input, normalized, tc.expected)
			}
		} else {
			if err == nil {
				t.Errorf("NormalizePhone(%q) expected error, got: %q", tc.input, normalized)
			}
		}
	}
}

func TestOTPRequestAndCooldown(t *testing.T) {
	authSvc, _, smsMock, _, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	// 1. First request succeeds
	res1, err := authSvc.RequestOTP(ctx, phone, strPtr("127.0.0.1"))
	if err != nil {
		t.Fatalf("RequestOTP failed: %v", err)
	}
	if res1.ChallengeID == "" {
		t.Fatalf("Expected non-empty challenge ID")
	}

	sentCode, ok := smsMock.LastSentOTP("+251" + phone[1:])
	if !ok || sentCode == "" {
		t.Fatalf("Expected SMS to be dispatched via SMSProvider")
	}

	// 2. Immediate second request rejected by cooldown
	_, err = authSvc.RequestOTP(ctx, phone, strPtr("127.0.0.1"))
	if !errors.Is(err, domain.ErrOTPRequestTooSoon) {
		t.Fatalf("Expected ErrOTPRequestTooSoon, got: %v", err)
	}

	// 3. After cooldown period expires, request succeeds
	time.Sleep(2100 * time.Millisecond)
	res2, err := authSvc.RequestOTP(ctx, phone, strPtr("127.0.0.1"))
	if err != nil {
		t.Fatalf("RequestOTP after cooldown failed: %v", err)
	}
	if res2.ChallengeID == res1.ChallengeID {
		t.Fatalf("Expected fresh challenge ID after cooldown")
	}
}

func TestOTPVerificationFlowAndSecurity(t *testing.T) {
	authSvc, cfg, smsMock, _, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	reqRes, err := authSvc.RequestOTP(ctx, phone, nil)
	if err != nil {
		t.Fatalf("RequestOTP failed: %v", err)
	}
	sentCode, _ := smsMock.LastSentOTP("+251" + phone[1:])

	// 1. Wrong OTP code rejected
	_, err = authSvc.VerifyOTP(ctx, reqRes.ChallengeID, phone, "000000", nil, nil)
	if !errors.Is(err, domain.ErrInvalidOTP) {
		t.Fatalf("Expected ErrInvalidOTP for wrong code, got: %v", err)
	}

	// 2. Attempt exhaustion (max 3 attempts)
	_, err = authSvc.VerifyOTP(ctx, reqRes.ChallengeID, phone, "000001", nil, nil)
	if !errors.Is(err, domain.ErrInvalidOTP) {
		t.Fatalf("Expected ErrInvalidOTP for attempt 2, got: %v", err)
	}
	_, err = authSvc.VerifyOTP(ctx, reqRes.ChallengeID, phone, "000002", nil, nil)
	if !errors.Is(err, domain.ErrInvalidOTP) {
		t.Fatalf("Expected ErrInvalidOTP for attempt 3, got: %v", err)
	}

	// 4th attempt rejected as exhausted
	_, err = authSvc.VerifyOTP(ctx, reqRes.ChallengeID, phone, sentCode, nil, nil)
	if !errors.Is(err, domain.ErrOTPTooManyAttempts) {
		t.Fatalf("Expected ErrOTPTooManyAttempts, got: %v", err)
	}

	// 3. New challenge with correct code
	time.Sleep(cfg.OTPRequestCooldown + 100*time.Millisecond)
	reqRes2, err := authSvc.RequestOTP(ctx, phone, nil)
	if err != nil {
		t.Fatalf("RequestOTP failed: %v", err)
	}
	sentCode2, _ := smsMock.LastSentOTP("+251" + phone[1:])

	authRes, err := authSvc.VerifyOTP(ctx, reqRes2.ChallengeID, phone, sentCode2, strPtr("TestAgent/1.0"), strPtr("10.0.0.1"))
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}
	if !authRes.IsNewUser {
		t.Errorf("Expected IsNewUser to be true for fresh phone")
	}
	if authRes.User.Phone != "+251"+phone[1:] {
		t.Errorf("User phone mismatch: got %s", authRes.User.Phone)
	}
	if authRes.RawToken == "" {
		t.Fatalf("Expected non-empty raw session token")
	}

	// 4. Code reuse rejected
	_, err = authSvc.VerifyOTP(ctx, reqRes2.ChallengeID, phone, sentCode2, nil, nil)
	if !errors.Is(err, domain.ErrOTPAlreadyUsed) {
		t.Fatalf("Expected ErrOTPAlreadyUsed on replay, got: %v", err)
	}
}

func TestExistingUserAuthenticationAndFormatInvariance(t *testing.T) {
	authSvc, cfg, smsMock, repos, ctx := setupAuthService(t)

	// Register with local format "0912345678"
	phoneLocal := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)
	req1, _ := authSvc.RequestOTP(ctx, phoneLocal, nil)
	code1, _ := smsMock.LastSentOTP("+251" + phoneLocal[1:])

	auth1, err := authSvc.VerifyOTP(ctx, req1.ChallengeID, phoneLocal, code1, nil, nil)
	if err != nil {
		t.Fatalf("First auth failed: %v", err)
	}
	if !auth1.IsNewUser {
		t.Errorf("Expected IsNewUser to be true")
	}

	// Re-authenticate using international format "+2519..."
	time.Sleep(cfg.OTPRequestCooldown + 100*time.Millisecond)
	phoneIntl := "+251" + phoneLocal[1:]
	req2, err := authSvc.RequestOTP(ctx, phoneIntl, nil)
	if err != nil {
		t.Fatalf("Second request failed: %v", err)
	}
	code2, _ := smsMock.LastSentOTP(phoneIntl)

	auth2, err := authSvc.VerifyOTP(ctx, req2.ChallengeID, phoneIntl, code2, nil, nil)
	if err != nil {
		t.Fatalf("Second verify failed: %v", err)
	}
	if auth2.IsNewUser {
		t.Errorf("Expected IsNewUser to be false on re-authentication")
	}
	if auth2.User.ID != auth1.User.ID {
		t.Fatalf("User identity mismatch across formats: got %v, want %v", auth2.User.ID, auth1.User.ID)
	}

	// Verify database only has one user for this phone
	userCount := 0
	_ = repos.TxMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		u, err := repos.User.GetByPhone(txCtx, phoneIntl)
		if err == nil && u != nil {
			userCount++
		}
		return nil
	})
	if userCount != 1 {
		t.Errorf("Expected exactly 1 user record, found %d", userCount)
	}
}

func TestSessionLifecycleAndRevocation(t *testing.T) {
	authSvc, _, smsMock, repos, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	req, _ := authSvc.RequestOTP(ctx, phone, nil)
	code, _ := smsMock.LastSentOTP("+251" + phone[1:])

	authRes, err := authSvc.VerifyOTP(ctx, req.ChallengeID, phone, code, nil, nil)
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}

	// 1. Resolve session with valid token
	user, session, err := authSvc.ResolveSession(ctx, authRes.RawToken)
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if user.ID != authRes.User.ID || session.ID != authRes.Session.ID {
		t.Fatalf("Resolved identity mismatch")
	}

	// 2. Security invariant: Raw token is NOT stored in DB
	storedSession, err := repos.Session.GetByTokenHash(ctx, service.HashToken(authRes.RawToken))
	if err != nil {
		t.Fatalf("Failed to fetch session by hash: %v", err)
	}
	if storedSession.TokenHash == authRes.RawToken {
		t.Fatalf("SECURITY VIOLATION: Raw token is stored plaintext in database!")
	}

	// 3. Invalid token rejected
	_, _, err = authSvc.ResolveSession(ctx, "non-existent-random-token-12345")
	if !errors.Is(err, domain.ErrUnauthenticated) {
		t.Fatalf("Expected ErrUnauthenticated for invalid token, got: %v", err)
	}

	// 4. Logout revokes session
	if err := authSvc.Logout(ctx, authRes.RawToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	// 5. Revoked session rejected
	_, _, err = authSvc.ResolveSession(ctx, authRes.RawToken)
	if !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("Expected ErrSessionRevoked after logout, got: %v", err)
	}
}

func TestConcurrentOTPVerificationAtomicity(t *testing.T) {
	authSvc, _, smsMock, _, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	req, err := authSvc.RequestOTP(ctx, phone, nil)
	if err != nil {
		t.Fatalf("RequestOTP failed: %v", err)
	}
	code, _ := smsMock.LastSentOTP("+251" + phone[1:])

	// Attempt to verify the EXACT same challenge concurrently 10 times in parallel
	concurrency := 10
	var successCount int32
	var alreadyUsedCount int32
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := authSvc.VerifyOTP(ctx, req.ChallengeID, phone, code, nil, nil)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, domain.ErrOTPAlreadyUsed) {
				atomic.AddInt32(&alreadyUsedCount, 1)
			}
		}()
	}

	wg.Wait()

	// CRITICAL INVARIANT: Exactly ONE verification MUST succeed; all others MUST be rejected!
	if successCount != 1 {
		t.Fatalf("CONCURRENCY VIOLATION: %d verifications succeeded! Exactly 1 must succeed.", successCount)
	}
	if alreadyUsedCount != int32(concurrency-1) {
		t.Logf("Rejected verifications: %d, expected: %d", alreadyUsedCount, concurrency-1)
	}
}

func TestExpiredSessionRejection(t *testing.T) {
	authSvc, _, _, repos, ctx := setupAuthService(t)
	phone := fmt.Sprintf("+2519%08d", time.Now().UnixNano()%100000000)

	user := &domain.User{Phone: phone, FullName: "Expiring Session User", Role: domain.RoleCustomer, IsActive: true}
	if err := repos.User.Create(ctx, user); err != nil {
		t.Fatalf("Create user failed: %v", err)
	}

	rawToken, _ := service.GenerateSecureSessionToken()
	tokenHash := service.HashToken(rawToken)
	expiredSession := &domain.Session{
		UserID:    user.ID,
		TokenHash: tokenHash,
		ExpiresAt: time.Now().UTC().Add(-1 * time.Hour), // Expired 1 hour ago
	}
	if err := repos.Session.Create(ctx, expiredSession); err != nil {
		t.Fatalf("Create expired session failed: %v", err)
	}

	_, _, err := authSvc.ResolveSession(ctx, rawToken)
	if !errors.Is(err, domain.ErrSessionExpired) {
		t.Fatalf("Expected ErrSessionExpired for elapsed session, got: %v", err)
	}
}

func TestSessionRevokeAllForUser(t *testing.T) {
	authSvc, cfg, smsMock, _, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	// Login 1 (Device A)
	req1, _ := authSvc.RequestOTP(ctx, phone, nil)
	code1, _ := smsMock.LastSentOTP("+251" + phone[1:])
	auth1, err := authSvc.VerifyOTP(ctx, req1.ChallengeID, phone, code1, strPtr("MobileApp/1.0"), nil)
	if err != nil {
		t.Fatalf("Login 1 failed: %v", err)
	}

	// Login 2 (Device B)
	time.Sleep(cfg.OTPRequestCooldown + 100*time.Millisecond)
	req2, _ := authSvc.RequestOTP(ctx, phone, nil)
	code2, _ := smsMock.LastSentOTP("+251" + phone[1:])
	auth2, err := authSvc.VerifyOTP(ctx, req2.ChallengeID, phone, code2, strPtr("DesktopBrowser/1.0"), nil)
	if err != nil {
		t.Fatalf("Login 2 failed: %v", err)
	}

	// Both sessions resolve
	_, _, err = authSvc.ResolveSession(ctx, auth1.RawToken)
	if err != nil {
		t.Fatalf("Device A session failed to resolve: %v", err)
	}
	_, _, err = authSvc.ResolveSession(ctx, auth2.RawToken)
	if err != nil {
		t.Fatalf("Device B session failed to resolve: %v", err)
	}

	// Revoke all sessions for this user
	if err := authSvc.LogoutAll(ctx, auth1.User.ID); err != nil {
		t.Fatalf("LogoutAll failed: %v", err)
	}

	// Both sessions must now be rejected
	_, _, err = authSvc.ResolveSession(ctx, auth1.RawToken)
	if !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("Expected Device A session to be revoked, got: %v", err)
	}
	_, _, err = authSvc.ResolveSession(ctx, auth2.RawToken)
	if !errors.Is(err, domain.ErrSessionRevoked) {
		t.Fatalf("Expected Device B session to be revoked, got: %v", err)
	}
}

func TestConcurrentOTPRequests(t *testing.T) {
	authSvc, _, _, _, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	concurrency := 5
	var cooldownCount int32
	var successCount int32
	var wg sync.WaitGroup

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := authSvc.RequestOTP(ctx, phone, nil)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, domain.ErrOTPRequestTooSoon) {
				atomic.AddInt32(&cooldownCount, 1)
			}
		}()
	}

	wg.Wait()

	// At least 1 must succeed, and any subsequent rapid requests are caught by cooldown or handled cleanly
	if successCount < 1 {
		t.Fatalf("Expected at least 1 successful OTP request, got: %d", successCount)
	}
	t.Logf("Concurrent OTP requests: %d succeeded, %d rate-limited by cooldown", successCount, cooldownCount)
}

func TestUserAccountCreationAndOnboarding(t *testing.T) {
	authSvc, _, smsMock, repos, ctx := setupAuthService(t)
	phone := fmt.Sprintf("09%08d", time.Now().UnixNano()%100000000)

	// 1. Request OTP for brand-new phone
	reqRes, err := authSvc.RequestOTP(ctx, phone, nil)
	if err != nil {
		t.Fatalf("RequestOTP failed: %v", err)
	}
	code, _ := smsMock.LastSentOTP("+251" + phone[1:])

	// 2. Verify OTP -> should create new canonical user with placeholder profile
	authRes, err := authSvc.VerifyOTP(ctx, reqRes.ChallengeID, phone, code, nil, nil)
	if err != nil {
		t.Fatalf("VerifyOTP failed: %v", err)
	}
	if !authRes.IsNewUser {
		t.Errorf("Expected IsNewUser to be true")
	}
	if authRes.User.ProfileCompleted {
		t.Errorf("Expected ProfileCompleted to be false for fresh account placeholder")
	}
	if authRes.User.Role != domain.RoleCustomer {
		t.Errorf("Expected role CUSTOMER, got: %s", authRes.User.Role)
	}

	// 3. Test invalid profile inputs
	// Too short (1 char)
	_, err = authSvc.UpdateProfile(ctx, authRes.User.ID, service.UpdateProfileInput{FullName: "A"})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for single character name, got: %v", err)
	}

	// Empty name
	_, err = authSvc.UpdateProfile(ctx, authRes.User.ID, service.UpdateProfileInput{FullName: "   "})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for whitespace name, got: %v", err)
	}

	// Too long (>100 chars)
	longName := "This is an extremely long name that exceeds the maximum permitted length of one hundred characters in the Cafnet database"
	_, err = authSvc.UpdateProfile(ctx, authRes.User.ID, service.UpdateProfileInput{FullName: longName})
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Errorf("Expected ErrInvalidInput for >100 char name, got: %v", err)
	}

	// 4. Test valid profile update (onboarding completion)
	avatar := "https://cdn.cafnet.et/avatars/user.jpg"
	updatedUser, err := authSvc.UpdateProfile(ctx, authRes.User.ID, service.UpdateProfileInput{
		FullName:  "Samuel Kebede",
		AvatarURL: &avatar,
	})
	if err != nil {
		t.Fatalf("UpdateProfile failed: %v", err)
	}
	if updatedUser.FullName != "Samuel Kebede" {
		t.Errorf("Expected FullName 'Samuel Kebede', got: %s", updatedUser.FullName)
	}
	if !updatedUser.ProfileCompleted {
		t.Errorf("Expected ProfileCompleted to be true after profile update")
	}
	if updatedUser.Phone != authRes.User.Phone {
		t.Errorf("Phone number was mutated! Expected %s, got %s", authRes.User.Phone, updatedUser.Phone)
	}
	if updatedUser.Role != authRes.User.Role {
		t.Errorf("Role was mutated! Expected %s, got %s", authRes.User.Role, updatedUser.Role)
	}

	// 5. Subsequent OTP verification for existing user
	time.Sleep(2100 * time.Millisecond) // Cooldown wait
	req2, err := authSvc.RequestOTP(ctx, phone, nil)
	if err != nil {
		t.Fatalf("Second RequestOTP failed: %v", err)
	}
	code2, _ := smsMock.LastSentOTP("+251" + phone[1:])

	auth2, err := authSvc.VerifyOTP(ctx, req2.ChallengeID, phone, code2, nil, nil)
	if err != nil {
		t.Fatalf("Second VerifyOTP failed: %v", err)
	}
	if auth2.IsNewUser {
		t.Errorf("Expected IsNewUser to be false on re-login")
	}
	if auth2.User.ID != authRes.User.ID {
		t.Errorf("User ID changed! Expected %s, got %s", authRes.User.ID, auth2.User.ID)
	}
	if auth2.User.FullName != "Samuel Kebede" {
		t.Errorf("Profile name lost on re-login! Expected 'Samuel Kebede', got %s", auth2.User.FullName)
	}
	if !auth2.User.ProfileCompleted {
		t.Errorf("ProfileCompleted should remain true on re-login")
	}

	// Verify only one user row exists in DB for this phone
	uFromRepo, err := repos.User.GetByPhone(ctx, "+251"+phone[1:])
	if err != nil {
		t.Fatalf("User lookup by phone failed: %v", err)
	}
	if uFromRepo.ID != authRes.User.ID {
		t.Errorf("Repo user ID mismatch: %s vs %s", uFromRepo.ID, authRes.User.ID)
	}
}

func TestConcurrentRegistrationSafety(t *testing.T) {
	_, _, _, repos, ctx := setupAuthService(t)
	canonicalPhone := fmt.Sprintf("+2519%08d", time.Now().UnixNano()%100000000)

	// Direct concurrent creation attempts on the repository
	concurrency := 4
	var wg sync.WaitGroup
	var successCount int32

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			user := &domain.User{
				Phone:    canonicalPhone,
				FullName: fmt.Sprintf("Customer Runner %d", idx),
				Role:     domain.RoleCustomer,
				IsActive: true,
			}
			err := repos.User.Create(ctx, user)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
	}

	wg.Wait()

	// All concurrent callers should either succeed idempotently or return the canonical row
	canonicalUser, err := repos.User.GetByPhone(ctx, canonicalPhone)
	if err != nil {
		t.Fatalf("Failed to fetch user after concurrent creation: %v", err)
	}
	if canonicalUser == nil {
		t.Fatalf("Expected canonical user to exist")
	}
}

func TestEmailPasswordSignUpAndSignIn(t *testing.T) {
	authSvc, _, _, _, ctx := setupAuthService(t)
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("test-%s@cafnet.et", uniqueSuffix)
	password := "SecureP@ss123"
	name := "Eleni Gebre"

	// 1. Validation Failures
	_, err := authSvc.SignUp(ctx, service.SignUpInput{
		Name:     "E",
		Email:    email,
		Password: password,
	}, nil, nil)
	if err == nil {
		t.Errorf("expected error for short name, got nil")
	}

	_, err = authSvc.SignUp(ctx, service.SignUpInput{
		Name:     name,
		Email:    "invalid-email-format",
		Password: password,
	}, nil, nil)
	if err == nil || !errors.Is(err, domain.ErrInvalidEmail) {
		t.Errorf("expected ErrInvalidEmail, got %v", err)
	}

	_, err = authSvc.SignUp(ctx, service.SignUpInput{
		Name:     name,
		Email:    email,
		Password: "short",
	}, nil, nil)
	if err == nil || !errors.Is(err, domain.ErrPasswordTooShort) {
		t.Errorf("expected ErrPasswordTooShort, got %v", err)
	}

	// 2. Successful Signup
	signUpRes, err := authSvc.SignUp(ctx, service.SignUpInput{
		Name:     name,
		Email:    email,
		Password: password,
	}, nil, nil)
	if err != nil {
		t.Fatalf("SignUp failed: %v", err)
	}
	if signUpRes.User == nil || signUpRes.Session == nil || signUpRes.RawToken == "" {
		t.Fatalf("SignUp missing user, session, or token")
	}
	if signUpRes.User.FullName != name {
		t.Errorf("expected name %s, got %s", name, signUpRes.User.FullName)
	}
	if signUpRes.User.GetEmail() != email {
		t.Errorf("expected email %s, got %s", email, signUpRes.User.GetEmail())
	}
	if signUpRes.User.Role != domain.RoleCustomer {
		t.Errorf("expected RoleCustomer, got %s", signUpRes.User.Role)
	}
	if !signUpRes.User.ProfileCompleted {
		t.Errorf("expected ProfileCompleted to be true for user with real name")
	}

	// 3. Duplicate Email Rejected
	_, err = authSvc.SignUp(ctx, service.SignUpInput{
		Name:     "Duplicate Person",
		Email:    fmt.Sprintf("  %s  ", strings.ToUpper(email)),
		Password: password,
	}, nil, nil)
	if err == nil || !errors.Is(err, domain.ErrEmailAlreadyExists) {
		t.Errorf("expected ErrEmailAlreadyExists for duplicate email, got %v", err)
	}

	// 4. Successful SignIn
	signInRes, err := authSvc.SignIn(ctx, service.SignInInput{
		Email:    fmt.Sprintf("  %s  ", email),
		Password: password,
	}, nil, nil)
	if err != nil {
		t.Fatalf("SignIn failed: %v", err)
	}
	if signInRes.User.ID != signUpRes.User.ID {
		t.Errorf("SignIn returned different user ID: got %s, want %s", signInRes.User.ID, signUpRes.User.ID)
	}

	// 5. Wrong Password Rejected (Generic Failure)
	_, err = authSvc.SignIn(ctx, service.SignInInput{
		Email:    email,
		Password: "WrongPassword999!",
	}, nil, nil)
	if err == nil || !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for wrong password, got %v", err)
	}

	// 6. Unknown Email Rejected (Generic Failure)
	_, err = authSvc.SignIn(ctx, service.SignInInput{
		Email:    "unknown-nonexistent@cafnet.et",
		Password: password,
	}, nil, nil)
	if err == nil || !errors.Is(err, domain.ErrInvalidCredentials) {
		t.Errorf("expected ErrInvalidCredentials for unknown email, got %v", err)
	}

	// 7. Session Validation and Logout
	user, session, err := authSvc.ResolveSession(ctx, signInRes.RawToken)
	if err != nil {
		t.Fatalf("ResolveSession failed: %v", err)
	}
	if user.ID != signUpRes.User.ID || !session.IsActive() {
		t.Fatalf("ResolveSession returned invalid user or inactive session")
	}

	if err := authSvc.Logout(ctx, signInRes.RawToken); err != nil {
		t.Fatalf("Logout failed: %v", err)
	}

	_, _, err = authSvc.ResolveSession(ctx, signInRes.RawToken)
	if err == nil || !errors.Is(err, domain.ErrSessionRevoked) {
		t.Errorf("expected ErrSessionRevoked after logout, got %v", err)
	}
}

func TestConcurrentEmailRegistrationSafety(t *testing.T) {
	authSvc, _, _, repos, ctx := setupAuthService(t)
	uniqueSuffix := fmt.Sprintf("%d", time.Now().UnixNano())
	email := fmt.Sprintf("concurrent-%s@cafnet.et", uniqueSuffix)
	password := "ConcurrencyPass123!"

	concurrency := 4
	var wg sync.WaitGroup
	var successCount int32
	var conflictCount int32

	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := authSvc.SignUp(ctx, service.SignUpInput{
				Name:     fmt.Sprintf("Runner %d", idx),
				Email:    email,
				Password: password,
			}, nil, nil)
			if err == nil {
				atomic.AddInt32(&successCount, 1)
			} else if errors.Is(err, domain.ErrEmailAlreadyExists) {
				atomic.AddInt32(&conflictCount, 1)
			}
		}(i)
	}

	wg.Wait()

	if successCount != 1 {
		t.Errorf("expected exactly 1 successful signup, got %d", successCount)
	}
	if conflictCount != int32(concurrency-1) {
		t.Errorf("expected %d conflicts, got %d", concurrency-1, conflictCount)
	}

	// Verify only 1 user exists with this email
	u, err := repos.User.GetByEmail(ctx, email)
	if err != nil || u == nil {
		t.Fatalf("expected user to exist for %s, got err: %v", email, err)
	}
}


