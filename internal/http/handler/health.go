package handler

import (
	"context"
	"net/http"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

// HealthHandler provides system liveness and database readiness checks without leaking sensitive internals.
type HealthHandler struct {
	pool *db.Pool
}

// NewHealthHandler constructs a HealthHandler.
func NewHealthHandler(pool *db.Pool) *HealthHandler {
	return &HealthHandler{pool: pool}
}

// Liveness responds with minimal 200 OK.
func (h *HealthHandler) Liveness(w http.ResponseWriter, r *http.Request) {
	response.JSON(w, http.StatusOK, map[string]string{
		"status": "ok",
	})
}

// Readiness verifies database connectivity with a bounded ping timeout.
func (h *HealthHandler) Readiness(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	if err := h.pool.Ping(ctx); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]string{
		"status":   "ready",
		"database": "connected",
	})
}
