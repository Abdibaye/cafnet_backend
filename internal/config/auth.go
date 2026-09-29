package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// AuthConfig encapsulates all runtime configuration parameters for authentication and sessions.
type AuthConfig struct {
	OTPTTL             time.Duration
	OTPMaxAttempts     int
	OTPRequestCooldown time.Duration
	SessionTTL         time.Duration
	SessionCookieName  string
	AllowedOrigins     []string
	DevMode            bool
	DeterministicOTP   string
	Environment        string
	CookieSecure       bool
	CookieSameSite     string
}

// LoadAuthConfig loads authentication configuration from environment variables with safe defaults.
func LoadAuthConfig() (*AuthConfig, error) {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = os.Getenv("ENV")
	}
	if env == "" {
		env = "development"
	}

	otpTTL := parseDurationEnv("AUTH_OTP_TTL", 5*time.Minute)
	otpMaxAttempts := parseIntEnv("AUTH_OTP_MAX_ATTEMPTS", 3)
	otpCooldown := parseDurationEnv("AUTH_OTP_REQUEST_COOLDOWN", 60*time.Second)
	sessionTTL := parseDurationEnv("AUTH_SESSION_TTL", 30*24*time.Hour) // 30 days

	cookieName := os.Getenv("AUTH_SESSION_COOKIE_NAME")
	if cookieName == "" {
		cookieName = "cafnet_session"
	}

	originsStr := os.Getenv("AUTH_ALLOWED_ORIGINS")
	var origins []string
	if originsStr != "" {
		for _, o := range strings.Split(originsStr, ",") {
			trimmed := strings.TrimSpace(o)
			if trimmed != "" {
				origins = append(origins, trimmed)
			}
		}
	}
	if len(origins) == 0 {
		origins = []string{"http://localhost:3000"}
	}

	devMode := parseBoolEnv("AUTH_DEV_MODE", env == "development" || env == "test")
	deterministicOTP := os.Getenv("AUTH_DETERMINISTIC_OTP")
	if deterministicOTP == "" {
		deterministicOTP = "123456"
	}

	cookieSecure := parseBoolEnv("AUTH_COOKIE_SECURE", env == "production")
	cookieSameSite := os.Getenv("AUTH_COOKIE_SAME_SITE")
	if cookieSameSite == "" {
		cookieSameSite = "Lax"
	}

	cfg := &AuthConfig{
		OTPTTL:             otpTTL,
		OTPMaxAttempts:     otpMaxAttempts,
		OTPRequestCooldown: otpCooldown,
		SessionTTL:         sessionTTL,
		SessionCookieName:  cookieName,
		AllowedOrigins:     origins,
		DevMode:            devMode,
		DeterministicOTP:   deterministicOTP,
		Environment:        env,
		CookieSecure:       cookieSecure,
		CookieSameSite:     cookieSameSite,
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Validate checks critical security invariants of the configuration.
func (c *AuthConfig) Validate() error {
	if c.DevMode && (c.Environment == "production" || c.Environment == "prod") {
		return fmt.Errorf("security violation: AUTH_DEV_MODE cannot be enabled in %s environment", c.Environment)
	}

	if c.OTPMaxAttempts < 1 || c.OTPMaxAttempts > 10 {
		return fmt.Errorf("AUTH_OTP_MAX_ATTEMPTS must be between 1 and 10, got %d", c.OTPMaxAttempts)
	}

	if c.OTPTTL < 30*time.Second || c.OTPTTL > 30*time.Minute {
		return fmt.Errorf("AUTH_OTP_TTL must be between 30s and 30m, got %v", c.OTPTTL)
	}

	if c.SessionTTL < 1*time.Hour {
		return fmt.Errorf("AUTH_SESSION_TTL must be at least 1 hour, got %v", c.SessionTTL)
	}

	return nil
}

func parseDurationEnv(key string, defaultVal time.Duration) time.Duration {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	d, err := time.ParseDuration(val)
	if err != nil {
		return defaultVal
	}
	return d
}

func parseIntEnv(key string, defaultVal int) int {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}

func parseBoolEnv(key string, defaultVal bool) bool {
	val := os.Getenv(key)
	if val == "" {
		return defaultVal
	}
	b, err := strconv.ParseBool(val)
	if err != nil {
		return defaultVal
	}
	return b
}
