package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"runtime/debug"

	"github.com/Abdibaye/cafnet_backend/internal/http/response"
)

// Recovery catches unhandled panics, logs stack traces server-side, and returns a safe 500 JSON response.
func Recovery(next http.Handler) http.Handler {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				reqID := GetRequestID(r.Context())
				stack := string(debug.Stack())

				logger.Error("panic recovered in HTTP handler",
					"error", fmt.Sprintf("%v", rec),
					"request_id", reqID,
					"method", r.Method,
					"path", r.URL.Path,
					"stack", stack,
				)

				response.Error(w, r, fmt.Errorf("internal server error"))
			}
		}()

		next.ServeHTTP(w, r)
	})
}
