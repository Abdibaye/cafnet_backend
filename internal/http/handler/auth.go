package handler

import (
	"net/http"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/auth"
	"github.com/Abdibaye/cafnet_backend/internal/config"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// AuthHandler adapts HTTP authentication requests to AuthService.
type AuthHandler struct {
	authService service.AuthService
	cfg         *config.AuthConfig
}

// NewAuthHandler constructs an AuthHandler.
func NewAuthHandler(authService service.AuthService, cfg *config.AuthConfig) *AuthHandler {
	return &AuthHandler{
		authService: authService,
		cfg:         cfg,
	}
}

type requestOTPPayload struct {
	Phone string `json:"phone"`
}

type verifyOTPPayload struct {
	ChallengeID string `json:"challenge_id"`
	Phone       string `json:"phone"`
	OTPCode     string `json:"otp_code"`
}

// RequestOTP handles OTP challenge initiation.
func (h *AuthHandler) RequestOTP(w http.ResponseWriter, r *http.Request) {
	var payload requestOTPPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	ip := r.RemoteAddr
	result, err := h.authService.RequestOTP(r.Context(), payload.Phone, &ip)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, result)
}

// VerifyOTP validates the OTP code, establishes a server session, and issues an HTTP-only cookie.
func (h *AuthHandler) VerifyOTP(w http.ResponseWriter, r *http.Request) {
	var payload verifyOTPPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	ua := r.UserAgent()
	ip := r.RemoteAddr

	result, err := h.authService.VerifyOTP(r.Context(), payload.ChallengeID, payload.Phone, payload.OTPCode, &ua, &ip)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	// Issue HTTP-only, secure session cookie. Never leak raw session token in the JSON body.
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    result.RawToken,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: parseSameSite(h.cfg.CookieSameSite),
		MaxAge:   int(h.cfg.SessionTTL.Seconds()),
	})

	response.JSON(w, http.StatusOK, map[string]any{
		"user":        result.User,
		"is_new_user": result.IsNewUser,
	})
}

// Logout revokes the session in the database and clears the client session cookie.
func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(h.cfg.SessionCookieName)
	if err == nil && cookie != nil && cookie.Value != "" {
		_ = h.authService.Logout(r.Context(), cookie.Value)
	}

	// Invalidate cookie in client browser
	http.SetCookie(w, &http.Cookie{
		Name:     h.cfg.SessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.cfg.CookieSecure,
		SameSite: parseSameSite(h.cfg.CookieSameSite),
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})

	response.JSON(w, http.StatusOK, map[string]bool{
		"logged_out": true,
	})
}

// Me returns the currently authenticated user identity.
func (h *AuthHandler) Me(w http.ResponseWriter, r *http.Request) {
	user, ok := auth.UserFromContext(r.Context())
	if !ok || user == nil {
		response.Error(w, r, domain.ErrUnauthenticated)
		return
	}

	response.JSON(w, http.StatusOK, user)
}

func parseSameSite(val string) http.SameSite {
	switch strings.ToLower(val) {
	case "strict":
		return http.SameSiteStrictMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteLaxMode
	}
}
