package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// CustomerHandler adapts customer-facing saved lists, following, claims, and venue submissions.
type CustomerHandler struct {
	customerService service.CustomerService
}

// NewCustomerHandler constructs a CustomerHandler.
func NewCustomerHandler(customerService service.CustomerService) *CustomerHandler {
	return &CustomerHandler{customerService: customerService}
}

// SaveBusiness bookmarks a venue for the authenticated customer.
func (h *CustomerHandler) SaveBusiness(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "businessID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.customerService.SaveBusiness(r.Context(), actor, domain.BusinessID(businessID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"saved": true})
}

// UnsaveBusiness removes a bookmarked venue.
func (h *CustomerHandler) UnsaveBusiness(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "businessID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.customerService.UnsaveBusiness(r.Context(), actor, domain.BusinessID(businessID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"saved": false})
}

// ListSavedBusinesses returns all bookmarked venues for the authenticated customer.
func (h *CustomerHandler) ListSavedBusinesses(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	list, err := h.customerService.ListSavedBusinesses(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, list)
}

// FollowBusiness subscribes the customer to a venue's broadcasts.
func (h *CustomerHandler) FollowBusiness(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "businessID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.customerService.FollowBusiness(r.Context(), actor, domain.BusinessID(businessID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"following": true})
}

// UnfollowBusiness unsubscribes the customer from a venue.
func (h *CustomerHandler) UnfollowBusiness(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "businessID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.customerService.UnfollowBusiness(r.Context(), actor, domain.BusinessID(businessID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"following": false})
}

// ListFollowedBusinesses returns all venues followed by the authenticated customer.
func (h *CustomerHandler) ListFollowedBusinesses(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	list, err := h.customerService.ListFollowedBusinesses(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, list)
}

type submitClaimPayload struct {
	BusinessID int64 `json:"business_id"`
}

// SubmitClaim allows a venue owner to submit a claim for an existing venue.
func (h *CustomerHandler) SubmitClaim(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var payload submitClaimPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	claim, err := h.customerService.SubmitClaim(r.Context(), actor, domain.BusinessID(payload.BusinessID))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, claim)
}

// ListMyClaims returns claims submitted by the authenticated user.
func (h *CustomerHandler) ListMyClaims(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	claims, err := h.customerService.ListMyClaims(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(claims) == limit
	response.JSONWithPagination(w, http.StatusOK, claims, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetMyClaim returns a specific claim by ID for the claimant.
func (h *CustomerHandler) GetMyClaim(w http.ResponseWriter, r *http.Request) {
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

	claim, err := h.customerService.GetMyClaim(r.Context(), actor, domain.ClaimID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, claim)
}

// CreateSubmission allows customers or owners to submit a venue recommendation.
func (h *CustomerHandler) CreateSubmission(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateSubmissionInput
	if err := request.DecodeJSON(r, &input, 16384); err != nil {
		response.Error(w, r, err)
		return
	}

	sub, err := h.customerService.CreateSubmission(r.Context(), actor, input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, sub)
}

// ListMySubmissions returns submissions created by the authenticated user.
func (h *CustomerHandler) ListMySubmissions(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	subs, err := h.customerService.ListMySubmissions(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(subs) == limit
	response.JSONWithPagination(w, http.StatusOK, subs, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetMySubmission returns a specific submission by ID for the submitter.
func (h *CustomerHandler) GetMySubmission(w http.ResponseWriter, r *http.Request) {
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

	sub, err := h.customerService.GetMySubmission(r.Context(), actor, domain.SubmissionID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, sub)
}
