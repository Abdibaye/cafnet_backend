package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
)

// CSRF guards state-changing endpoints against cross-site request forgery attacks.
func CSRF(allowedOrigins []string, sessionCookieName string) func(http.Handler) http.Handler {
	originMap := make(map[string]bool)
	for _, o := range allowedOrigins {
		clean := strings.TrimRight(strings.TrimSpace(o), "/")
		if clean != "" {
			originMap[clean] = true
		}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Safe methods do not mutate state
			if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
				next.ServeHTTP(w, r)
				return
			}

			// If no session cookie is sent, this request is unauthenticated or not using browser cookie credentials
			_, err := r.Cookie(sessionCookieName)
			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			// Check Origin header first
			origin := strings.TrimRight(r.Header.Get("Origin"), "/")
			if origin != "" {
				if !originMap[origin] {
					response.Error(w, r, domain.NewDomainError(domain.ErrForbidden, "cross-origin request forbidden by CSRF policy", nil))
					return
				}
				next.ServeHTTP(w, r)
				return
			}

			// Fallback to Referer header if Origin is omitted
			referer := r.Header.Get("Referer")
			if referer != "" {
				parsedURL, err := url.Parse(referer)
				if err == nil {
					refOrigin := fmtOrigin(parsedURL)
					if !originMap[refOrigin] {
						response.Error(w, r, domain.NewDomainError(domain.ErrForbidden, "invalid referer origin for state mutation", nil))
						return
					}
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

func fmtOrigin(u *url.URL) string {
	if u.Port() != "" {
		return u.Scheme + "://" + u.Hostname() + ":" + u.Port()
	}
	return u.Scheme + "://" + u.Hostname()
}
