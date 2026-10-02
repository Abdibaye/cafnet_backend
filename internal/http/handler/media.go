package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// MediaHandler adapts HTTP requests for Tigris media uploads and business/menu imagery.
type MediaHandler struct {
	mediaService service.MediaService
}

// NewMediaHandler constructs a MediaHandler.
func NewMediaHandler(mediaService service.MediaService) *MediaHandler {
	return &MediaHandler{mediaService: mediaService}
}

// CreateUpload initiates a direct browser upload by returning a short-lived presigned PUT URL.
func (h *MediaHandler) CreateUpload(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input domain.CreateUploadInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.mediaService.CreateUpload(r.Context(), actor, input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

// CompleteUpload verifies object existence in Tigris and marks the media record READY.
func (h *MediaHandler) CompleteUpload(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	mediaID, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	media, err := h.mediaService.CompleteUpload(r.Context(), actor, domain.MediaID(mediaID))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, domain.CompleteUploadResponse{Media: *media})
}

// DeleteMedia authorizes and removes a media object from Tigris and PostgreSQL.
func (h *MediaHandler) DeleteMedia(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	mediaID, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.mediaService.DeleteMedia(r.Context(), actor, domain.MediaID(mediaID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// SetBusinessCover sets an existing business gallery photo as the primary cover image.
func (h *MediaHandler) SetBusinessCover(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	bizID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var payload struct {
		MediaID domain.MediaID `json:"media_id"`
	}
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.mediaService.SetBusinessCover(r.Context(), actor, domain.BusinessID(bizID), payload.MediaID); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"updated": true})
}

// ReorderBusinessGallery saves an ordered list of gallery media IDs for a business.
func (h *MediaHandler) ReorderBusinessGallery(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	bizID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input domain.ReorderMediaInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.mediaService.ReorderBusinessGallery(r.Context(), actor, domain.BusinessID(bizID), input); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"reordered": true})
}

// GetBusinessMedia returns all ready media for a business (cover and gallery).
func (h *MediaHandler) GetBusinessMedia(w http.ResponseWriter, r *http.Request) {
	bizID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	media, err := h.mediaService.GetBusinessMedia(r.Context(), domain.BusinessID(bizID))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, media)
}

// GetSubmissionCover returns the cover image for a pending or reviewed submission.
func (h *MediaHandler) GetSubmissionCover(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	subID, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	cover, err := h.mediaService.GetSubmissionCover(r.Context(), actor, domain.SubmissionID(subID))
	if err != nil {
		response.Error(w, r, err)
		return
	}
	if cover == nil {
		response.JSON(w, http.StatusOK, map[string]any{"cover": nil})
		return
	}

	response.JSON(w, http.StatusOK, cover)
}

// DeleteMenuItemMedia removes an associated image from a menu item.
func (h *MediaHandler) DeleteMenuItemMedia(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	itemID, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.mediaService.DeleteMenuItemMedia(r.Context(), actor, domain.ItemID(itemID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}
