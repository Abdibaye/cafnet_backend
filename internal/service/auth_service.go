package service

import (
	"context"
	"crypto/subtle"
	"errors"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// OTPRequestResult represents the safe public response returned after requesting an OTP.
type OTPRequestResult struct {
	ChallengeID     string    `json:"challenge_id"`
	ExpiresAt       time.Time `json:"expires_at"`
	CooldownSeconds int       `json:"cooldown_seconds"`
	DevOTP          string    `json:"dev_otp,omitempty"` // populated only in dev mode for local testing
}

// AuthResult represents the successful authentication output containing identity and credential.
type AuthResult struct {
	User      *domain.User    `json:"user"`
	Session   *domain.Session `json:"session"`
	RawToken  string          `json:"token"` // High-entropy opaque token for HTTP-only cookie
	IsNewUser bool            `json:"is_new_user"`
}

// AuthService defines the core identity and session boundary.
type AuthService interface {
	RequestOTP(ctx context.Context, rawPhone string, ipAddress *string) (*OTPRequestResult, error)
	VerifyOTP(ctx context.Context, challengeID string, rawPhone string, otpCode string, userAgent *string, ipAddress *string) (*AuthResult, error)
	ResolveSession(ctx context.Context, rawSessionToken string) (*domain.User, *domain.Session, error)
	Logout(ctx context.Context, rawSessionToken string) error
	LogoutAll(ctx context.Context, userID domain.UserID) error
}

type authService struct {
	cfg         *config.AuthConfig
	userRepo    repository.UserRepository
	sessionRepo repository.SessionRepository
	otpRepo     repository.OTPRepository
	txMgr       repository.TxManager
	smsProvider SMSProvider
}

// NewAuthService constructs a production-ready authentication service.
func NewAuthService(
	cfg *config.AuthConfig,
	userRepo repository.UserRepository,
	sessionRepo repository.SessionRepository,
	otpRepo repository.OTPRepository,
	txMgr repository.TxManager,
	smsProvider SMSProvider,
) AuthService {
	if smsProvider == nil {
		smsProvider = NewMockSMSProvider()
	}
	return &authService{
		cfg:         cfg,
		userRepo:    userRepo,
		sessionRepo: sessionRepo,
		otpRepo:     otpRepo,
		txMgr:       txMgr,
		smsProvider: smsProvider,
	}
}

func (s *authService) RequestOTP(ctx context.Context, rawPhone string, ipAddress *string) (*OTPRequestResult, error) {
	normalizedPhone, err := domain.NormalizePhone(rawPhone)
	if err != nil {
		return nil, domain.ErrInvalidPhone
	}

	// 1. Rate limiting & request cooldown
	recent, err := s.otpRepo.GetRecentRequest(ctx, normalizedPhone, s.cfg.OTPRequestCooldown)
	if err != nil {
		return nil, err
	}
	if recent != nil {
		return nil, domain.ErrOTPRequestTooSoon
	}

	// 2. Invalidate any previous unconsumed challenges for this phone
	if err := s.otpRepo.InvalidatePrevious(ctx, normalizedPhone); err != nil {
		return nil, err
	}

	// 3. Generate verification code
	var otpCode string
	if s.cfg.DevMode && s.cfg.DeterministicOTP != "" {
		otpCode = s.cfg.DeterministicOTP
	} else {
		generated, err := GenerateRandomOTP()
		if err != nil {
			return nil, err
		}
		otpCode = generated
	}

	// 4. Compute salted hash (never persist plaintext OTP)
	otpHash := HashOTP(normalizedPhone, otpCode)
	expiresAt := time.Now().UTC().Add(s.cfg.OTPTTL)

	challenge := &domain.OTPChallenge{
		Phone:       normalizedPhone,
		OTPCodeHash: otpHash,
		ExpiresAt:   expiresAt,
		MaxAttempts: s.cfg.OTPMaxAttempts,
		IPAddress:   ipAddress,
	}

	if err := s.otpRepo.CreateChallenge(ctx, challenge); err != nil {
		return nil, err
	}

	// 5. Dispatch SMS (mock or real provider)
	if err := s.smsProvider.SendOTP(ctx, normalizedPhone, otpCode); err != nil {
		return nil, err
	}

	result := &OTPRequestResult{
		ChallengeID:     challenge.ID,
		ExpiresAt:       challenge.ExpiresAt,
		CooldownSeconds: int(s.cfg.OTPRequestCooldown.Seconds()),
	}

	if s.cfg.DevMode {
		result.DevOTP = otpCode
	}

	return result, nil
}

func (s *authService) VerifyOTP(
	ctx context.Context,
	challengeID string,
	rawPhone string,
	otpCode string,
	userAgent *string,
	ipAddress *string,
) (*AuthResult, error) {
	normalizedPhone, err := domain.NormalizePhone(rawPhone)
	if err != nil {
		return nil, domain.ErrInvalidPhone
	}

	var authResult *AuthResult

	// Execute verification, user creation, and session issuance inside an atomic transaction
	err = s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		// 1. Fetch challenge with database row lock (FOR UPDATE) to prevent concurrent double-consumption
		challenge, err := s.otpRepo.GetForUpdate(txCtx, challengeID)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				return domain.ErrInvalidOTP
			}
			return err
		}

		if challenge.Phone != normalizedPhone {
			return domain.ErrInvalidOTP
		}

		if challenge.IsConsumed {
			return domain.ErrOTPAlreadyUsed
		}

		if challenge.IsExpired() {
			return domain.ErrOTPExpired
		}

		if challenge.IsExhausted() {
			return domain.ErrOTPTooManyAttempts
		}

		// 2. Validate OTP code via constant-time comparison
		expectedHash := HashOTP(normalizedPhone, otpCode)
		if subtle.ConstantTimeCompare([]byte(challenge.OTPCodeHash), []byte(expectedHash)) != 1 {
			return domain.ErrInvalidOTP
		}

		// 3. Mark challenge consumed
		if err := s.otpRepo.ConsumeChallenge(txCtx, challenge.ID); err != nil {
			return err
		}

		// 4. Resolve or create user identity
		var user *domain.User
		var isNewUser bool

		existingUser, err := s.userRepo.GetByPhone(txCtx, normalizedPhone)
		if err != nil {
			if errors.Is(err, domain.ErrNotFound) {
				// Create new canonical User
				isNewUser = true
				last4 := normalizedPhone
				if len(last4) >= 4 {
					last4 = last4[len(last4)-4:]
				}
				newUser := &domain.User{
					Phone:    normalizedPhone,
					FullName: "Customer " + last4,
					Role:     domain.RoleCustomer,
					IsActive: true,
				}
				if err := s.userRepo.Create(txCtx, newUser); err != nil {
					return err
				}
				user = newUser
			} else {
				return err
			}
		} else {
			user = existingUser
		}

		if !user.IsActive || user.DeletedAt != nil {
			return domain.ErrUnauthenticated
		}

		// 5. Generate secure session credential
		rawToken, err := GenerateSecureSessionToken()
		if err != nil {
			return err
		}

		tokenHash := HashToken(rawToken)
		session := &domain.Session{
			UserID:    user.ID,
			TokenHash: tokenHash,
			UserAgent: userAgent,
			IPAddress: ipAddress,
			ExpiresAt: time.Now().UTC().Add(s.cfg.SessionTTL),
		}

		if err := s.sessionRepo.Create(txCtx, session); err != nil {
			return err
		}

		authResult = &AuthResult{
			User:      user,
			Session:   session,
			RawToken:  rawToken,
			IsNewUser: isNewUser,
		}

		return nil
	})

	if err != nil {
		if errors.Is(err, domain.ErrInvalidOTP) {
			_ = s.otpRepo.IncrementAttempts(ctx, challengeID)
		}
		return nil, err
	}

	return authResult, nil
}

func (s *authService) ResolveSession(ctx context.Context, rawSessionToken string) (*domain.User, *domain.Session, error) {
	if rawSessionToken == "" {
		return nil, nil, domain.ErrUnauthenticated
	}

	tokenHash := HashToken(rawSessionToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil, nil, domain.ErrUnauthenticated
		}
		return nil, nil, err
	}

	if session.IsRevoked() {
		return nil, nil, domain.ErrSessionRevoked
	}

	if session.IsExpired() {
		return nil, nil, domain.ErrSessionExpired
	}

	user, err := s.userRepo.GetByID(ctx, session.UserID)
	if err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			return nil, nil, domain.ErrUnauthenticated
		}
		return nil, nil, err
	}

	if !user.IsActive || user.DeletedAt != nil {
		return nil, nil, domain.ErrUnauthenticated
	}

	// Update last used timestamp (non-blocking / error swallowed)
	_ = s.sessionRepo.UpdateLastUsed(ctx, session.ID)

	return user, session, nil
}

func (s *authService) Logout(ctx context.Context, rawSessionToken string) error {
	if rawSessionToken == "" {
		return nil
	}

	tokenHash := HashToken(rawSessionToken)
	session, err := s.sessionRepo.GetByTokenHash(ctx, tokenHash)
	if err != nil {
		if errors.Is(err, domain.ErrSessionNotFound) {
			return nil
		}
		return err
	}

	return s.sessionRepo.Revoke(ctx, session.ID)
}

func (s *authService) LogoutAll(ctx context.Context, userID domain.UserID) error {
	return s.sessionRepo.RevokeAllForUser(ctx, userID)
}
