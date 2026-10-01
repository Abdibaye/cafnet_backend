package domain

import (
	"strings"
	"time"
)

// User represents the canonical user identity across Cafnet.
type User struct {
	ID               UserID     `json:"id"`
	Phone            string     `json:"phone,omitempty"`
	Email            *string    `json:"email,omitempty"`
	PasswordHash     *string    `json:"-"` // Never expose in JSON serialization
	FullName         string     `json:"full_name"`
	AvatarURL        *string    `json:"avatar_url,omitempty"`
	Role             UserRole   `json:"role"`
	IsActive         bool       `json:"is_active"`
	ProfileCompleted bool       `json:"profile_completed"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	DeletedAt        *time.Time `json:"deleted_at,omitempty"`
}

// GetPhone safely returns the user's phone number.
func (u *User) GetPhone() string {
	return u.Phone
}

// GetEmail safely returns the user's email address or empty string if nil.
func (u *User) GetEmail() string {
	if u.Email == nil {
		return ""
	}
	return *u.Email
}

// IsAdmin returns true if the user holds platform governance permissions.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// IsProfileComplete determines whether the user has set a personalized full name beyond the default auto-generated placeholder.
func (u *User) IsProfileComplete() bool {
	trimmed := strings.TrimSpace(u.FullName)
	if trimmed == "" {
		return false
	}
	last4 := ""
	if len(u.Phone) >= 4 {
		last4 = u.Phone[len(u.Phone)-4:]
	}
	if last4 != "" && trimmed == "Customer "+last4 {
		return false
	}
	if trimmed == "Customer" {
		return false
	}
	return true
}

// SetProfileCompleted sets the computed ProfileCompleted field on the User instance.
func (u *User) SetProfileCompleted() {
	u.ProfileCompleted = u.IsProfileComplete()
}

// Session represents an active authenticated session.
type Session struct {
	ID         string     `json:"id"`
	UserID     UserID     `json:"user_id"`
	TokenHash  string     `json:"token_hash"`
	UserAgent  *string    `json:"user_agent,omitempty"`
	IPAddress  *string    `json:"ip_address,omitempty"`
	ExpiresAt  time.Time  `json:"expires_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
	RevokedAt  *time.Time `json:"revoked_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IsExpired returns true if the session timestamp has elapsed.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}

// IsRevoked returns true if the session has been explicitly invalidated.
func (s *Session) IsRevoked() bool {
	return s.RevokedAt != nil
}

// IsActive returns true if the session is unexpired and unrevoked.
func (s *Session) IsActive() bool {
	return !s.IsExpired() && !s.IsRevoked()
}
