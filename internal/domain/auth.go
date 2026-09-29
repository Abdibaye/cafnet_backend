package domain

import (
	"time"
)

// OTPChallenge represents a cryptographic one-time password verification challenge.
type OTPChallenge struct {
	ID          string     `json:"id"`
	Phone       string     `json:"phone"` // Normalized E.164 phone
	OTPCodeHash string     `json:"-"`     // Stored SHA-256 hash (never serialized or exposed)
	ExpiresAt   time.Time  `json:"expires_at"`
	Attempts    int        `json:"attempts"`
	MaxAttempts int        `json:"max_attempts"`
	IsConsumed  bool       `json:"is_consumed"`
	ConsumedAt  *time.Time `json:"consumed_at,omitempty"`
	IPAddress   *string    `json:"ip_address,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// IsExpired returns true if the OTP challenge has exceeded its expiration window.
func (c *OTPChallenge) IsExpired() bool {
	return time.Now().UTC().After(c.ExpiresAt)
}

// IsExhausted returns true if verification attempts meet or exceed the maximum allowed.
func (c *OTPChallenge) IsExhausted() bool {
	return c.Attempts >= c.MaxAttempts
}

// CanAttempt returns true if the challenge is unexpired, unconsumed, and has attempts remaining.
func (c *OTPChallenge) CanAttempt() bool {
	return !c.IsExpired() && !c.IsConsumed && !c.IsExhausted()
}
