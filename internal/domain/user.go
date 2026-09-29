package domain

import (
	"time"
)

// User represents the canonical user identity across Cafnet.
type User struct {
	ID        UserID     `json:"id"`
	Phone     string     `json:"phone"`
	FullName  string     `json:"full_name"`
	AvatarURL *string    `json:"avatar_url,omitempty"`
	Role      UserRole   `json:"role"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	DeletedAt *time.Time `json:"deleted_at,omitempty"`
}

// IsAdmin returns true if the user holds platform governance permissions.
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// Session represents an active authenticated session.
type Session struct {
	ID        string    `json:"id"`
	UserID    UserID    `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	UserAgent *string   `json:"user_agent,omitempty"`
	IPAddress *string   `json:"ip_address,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true if the session timestamp has elapsed.
func (s *Session) IsExpired() bool {
	return time.Now().UTC().After(s.ExpiresAt)
}
