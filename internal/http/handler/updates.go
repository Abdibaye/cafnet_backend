package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// UpdatesHandler adapts public viewing and authorized manager publishing of venue broadcasts.
type UpdatesHandler struct {
	updatesService service.UpdatesService
}

// NewUpdatesHandler constructs an UpdatesHandler.
func NewUpdatesHandler(updatesService service.UpdatesService) *UpdatesHandler {
	return &UpdatesHandler{updatesService: updatesService}
}

// ListUpdates returns active broadcasts for public display, or all updates if authorized.
func (h *UpdatesHandler) ListUpdates(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	includeExpired := r.URL.Query().Get("include_expired") == "true"
	actor := middleware.CurrentUser(r)

	updates, err := h.updatesService.ListBusinessUpdates(r.Context(), actor, domain.BusinessID(id), includeExpired)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, updates)
}

// ListActiveUpdates returns active broadcasts across all businesses for discovery feeds (e.g. Explore).
func (h *UpdatesHandler) ListActiveUpdates(w http.ResponseWriter, r *http.Request) {
	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	updates, err := h.updatesService.ListActiveUpdates(r.Context(), limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, updates)
}

// ListFollowingUpdates returns active broadcasts for businesses followed by the authenticated user.
func (h *UpdatesHandler) ListFollowingUpdates(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	updates, err := h.updatesService.ListFollowingUpdates(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, updates)
}

// CreateUpdate broadcasts a new announcement, event, or offer.
func (h *UpdatesHandler) CreateUpdate(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateUpdateInput
	if err := request.DecodeJSON(r, &input, 16384); err != nil {
		response.Error(w, r, err)
		return
	}

	upd, err := h.updatesService.CreateUpdate(r.Context(), actor, domain.BusinessID(id), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, upd)
}

// UpdateUpdate modifies an existing broadcast post.
func (h *UpdatesHandler) UpdateUpdate(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	updateID, err := request.PathUUID(r, "updateID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.UpdateUpdateInput
	if err := request.DecodeJSON(r, &input, 16384); err != nil {
		response.Error(w, r, err)
		return
	}

	upd, err := h.updatesService.UpdateUpdate(r.Context(), actor, domain.BusinessID(id), domain.UpdateID(updateID), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, upd)
}

// DeleteUpdate removes a broadcast update.
func (h *UpdatesHandler) DeleteUpdate(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	updateID, err := request.PathUUID(r, "updateID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.updatesService.DeleteUpdate(r.Context(), actor, domain.BusinessID(id), domain.UpdateID(updateID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
