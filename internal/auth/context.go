package auth

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type userContextKey struct{}
type sessionContextKey struct{}

// WithUser returns a child context populated with the authenticated domain.User.
func WithUser(ctx context.Context, user *domain.User) context.Context {
	return context.WithValue(ctx, userContextKey{}, user)
}

// UserFromContext extracts the authenticated domain.User from the context if present.
func UserFromContext(ctx context.Context) (*domain.User, bool) {
	u, ok := ctx.Value(userContextKey{}).(*domain.User)
	return u, ok && u != nil
}

// WithSession returns a child context populated with the active domain.Session.
func WithSession(ctx context.Context, session *domain.Session) context.Context {
	return context.WithValue(ctx, sessionContextKey{}, session)
}

// SessionFromContext extracts the active domain.Session from the context if present.
func SessionFromContext(ctx context.Context) (*domain.Session, bool) {
	s, ok := ctx.Value(sessionContextKey{}).(*domain.Session)
	return s, ok && s != nil
}
