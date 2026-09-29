package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// ReviewHandler adapts public review browsing, verified dining review submission, and engagement.
type ReviewHandler struct {
	reviewService service.ReviewService
}

// NewReviewHandler constructs a ReviewHandler.
func NewReviewHandler(reviewService service.ReviewService) *ReviewHandler {
	return &ReviewHandler{reviewService: reviewService}
}

// ListBusinessReviews returns reviews for a venue.
func (h *ReviewHandler) ListBusinessReviews(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	reviews, err := h.reviewService.ListBusinessReviews(r.Context(), domain.BusinessID(id), limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(reviews) == limit
	response.JSONWithPagination(w, http.StatusOK, reviews, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetReviewSummary returns aggregated rating statistics.
func (h *ReviewHandler) GetReviewSummary(w http.ResponseWriter, r *http.Request) {
	id, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	summary, err := h.reviewService.GetReviewSummary(r.Context(), domain.BusinessID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, summary)
}

// CreateReview creates a verified review tied to a completed reservation.
func (h *ReviewHandler) CreateReview(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateReviewInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	rev, err := h.reviewService.CreateReview(r.Context(), actor, input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, rev)
}

// UpdateReview allows the author to edit rating or review text.
func (h *ReviewHandler) UpdateReview(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.UpdateReviewInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	rev, err := h.reviewService.UpdateReview(r.Context(), actor, domain.ReviewID(id), input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, rev)
}

// DeleteReview soft-deletes a review written by the author.
func (h *ReviewHandler) DeleteReview(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reviewService.DeleteReview(r.Context(), actor, domain.ReviewID(id)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"deleted": true})
}

// MarkHelpful casts an upvote for a review.
func (h *ReviewHandler) MarkHelpful(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reviewService.MarkHelpful(r.Context(), actor, domain.ReviewID(id)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"helpful": true})
}

// UnmarkHelpful removes an upvote from a review.
func (h *ReviewHandler) UnmarkHelpful(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	id, err := request.PathUUID(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reviewService.UnmarkHelpful(r.Context(), actor, domain.ReviewID(id)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"helpful": false})
}

// ListMyReviews returns all reviews written by the authenticated user.
func (h *ReviewHandler) ListMyReviews(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	reviews, err := h.reviewService.ListMyReviews(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, reviews)
}
