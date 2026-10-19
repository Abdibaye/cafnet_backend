package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// BusinessHandler adapts venue profile and management operations for authorized managers.
type BusinessHandler struct {
	businessService service.BusinessService
}

// NewBusinessHandler constructs a BusinessHandler.
func NewBusinessHandler(businessService service.BusinessService) *BusinessHandler {
	return &BusinessHandler{businessService: businessService}
}

// ListManagedBusinesses returns venues managed by the authenticated actor.
func (h *BusinessHandler) ListManagedBusinesses(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businesses, err := h.businessService.GetManagedBusinesses(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, businesses)
}

// GetManagedBusiness returns management-level venue details.
func (h *BusinessHandler) GetManagedBusiness(w http.ResponseWriter, r *http.Request) {
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

	b, err := h.businessService.GetManagedBusiness(r.Context(), actor, domain.BusinessID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, b)
}

// UpdateBusiness applies updates to venue metadata.
func (h *BusinessHandler) UpdateBusiness(w http.ResponseWriter, r *http.Request) {
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

	var input service.UpdateBusinessInput
	if err := request.DecodeJSON(r, &input, 32768); err != nil {
		response.Error(w, r, err)
		return
	}

	updated, err := h.businessService.UpdateBusiness(r.Context(), actor, domain.BusinessID(id), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, updated)
}

// GetHours retrieves operating hours for management.
func (h *BusinessHandler) GetHours(w http.ResponseWriter, r *http.Request) {
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

	hours, err := h.businessService.GetHours(r.Context(), actor, domain.BusinessID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, hours)
}

// SetHours replaces operating hours schedule.
func (h *BusinessHandler) SetHours(w http.ResponseWriter, r *http.Request) {
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

	var hours []domain.BusinessHours
	if err := request.DecodeJSON(r, &hours, 16384); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.businessService.SetHours(r.Context(), actor, domain.BusinessID(id), hours); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"updated": true})
}

// GetImages retrieves gallery images for management.
func (h *BusinessHandler) GetImages(w http.ResponseWriter, r *http.Request) {
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

	images, err := h.businessService.GetImages(r.Context(), actor, domain.BusinessID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, images)
}

// SetImages replaces venue gallery images.
func (h *BusinessHandler) SetImages(w http.ResponseWriter, r *http.Request) {
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

	var images []domain.BusinessImage
	if err := request.DecodeJSON(r, &images, 32768); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.businessService.SetImages(r.Context(), actor, domain.BusinessID(id), images); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"updated": true})
}

// DeleteImage removes a gallery image from a business.
func (h *BusinessHandler) DeleteImage(w http.ResponseWriter, r *http.Request) {
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

	imageID := r.PathValue("imageId")
	if imageID == "" {
		response.Error(w, r, domain.NewDomainError(domain.ErrInvalidInput, "image ID is required", nil))
		return
	}

	if err := h.businessService.DeleteImage(r.Context(), actor, domain.BusinessID(id), imageID); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

