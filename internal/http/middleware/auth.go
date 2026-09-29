package middleware

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/auth"
	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// Authenticate resolves the session cookie and attaches the authenticated domain.User to the request context.
// If required is true and no valid session is found, it terminates the request with HTTP 401 Unauthorized.
func Authenticate(authService service.AuthService, cookieName string, required bool) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			cookie, err := r.Cookie(cookieName)
			if err != nil || cookie == nil || cookie.Value == "" {
				if required {
					response.Error(w, r, domain.ErrUnauthenticated)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			user, session, err := authService.ResolveSession(r.Context(), cookie.Value)
			if err != nil {
				if required {
					response.Error(w, r, err)
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			ctx := auth.WithUser(r.Context(), user)
			ctx = auth.WithSession(ctx, session)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// CurrentUser retrieves the authenticated user from the request context if present.
func CurrentUser(r *http.Request) *domain.User {
	u, _ := auth.UserFromContext(r.Context())
	return u
}

// MustCurrentUser retrieves the authenticated user, or nil if unauthenticated.
func MustCurrentUser(r *http.Request) (*domain.User, error) {
	u := CurrentUser(r)
	if u == nil {
		return nil, domain.ErrUnauthenticated
	}
	return u, nil
}
