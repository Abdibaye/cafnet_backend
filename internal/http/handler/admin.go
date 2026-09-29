package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// AdminHandler adapts administrative governance, approvals, moderation, and auditing.
type AdminHandler struct {
	adminService service.AdminService
}

// NewAdminHandler constructs an AdminHandler.
func NewAdminHandler(adminService service.AdminService) *AdminHandler {
	return &AdminHandler{adminService: adminService}
}

// ListPendingSubmissions returns venue submissions awaiting administrative review.
func (h *AdminHandler) ListPendingSubmissions(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	subs, err := h.adminService.ListPendingSubmissions(r.Context(), actor, limit, offset)
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

// ApproveSubmission approves a venue submission, creating the business and granting primary ownership.
func (h *AdminHandler) ApproveSubmission(w http.ResponseWriter, r *http.Request) {
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

	b, err := h.adminService.ApproveSubmission(r.Context(), actor, domain.SubmissionID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, b)
}

type rejectPayload struct {
	Reason string `json:"reason"`
}

// RejectSubmission rejects a venue submission with a reason.
func (h *AdminHandler) RejectSubmission(w http.ResponseWriter, r *http.Request) {
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

	var payload rejectPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.RejectSubmission(r.Context(), actor, domain.SubmissionID(id), payload.Reason); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"rejected": true})
}

// ListPendingClaims returns business claims awaiting verification.
func (h *AdminHandler) ListPendingClaims(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	claims, err := h.adminService.ListPendingClaims(r.Context(), actor, limit, offset)
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

type approveClaimPayload struct {
	Notes *string `json:"notes"`
}

// ApproveClaim approves a venue claim, assigning the claimant as primary owner.
func (h *AdminHandler) ApproveClaim(w http.ResponseWriter, r *http.Request) {
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

	var payload approveClaimPayload
	_ = request.DecodeJSON(r, &payload, 4096)

	if err := h.adminService.ApproveClaim(r.Context(), actor, domain.ClaimID(id), payload.Notes); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"approved": true})
}

// RejectClaim rejects a business claim.
func (h *AdminHandler) RejectClaim(w http.ResponseWriter, r *http.Request) {
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

	var payload rejectPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.RejectClaim(r.Context(), actor, domain.ClaimID(id), payload.Reason); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"rejected": true})
}

// ListPendingReviewReports returns community reports filed against reviews.
func (h *AdminHandler) ListPendingReviewReports(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	reports, err := h.adminService.ListPendingReviewReports(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(reports) == limit
	response.JSONWithPagination(w, http.StatusOK, reports, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

type resolveReviewReportPayload struct {
	Status domain.ReportStatus `json:"status"` // DISMISSED or ACTIONED
	Action *string             `json:"action"`
}

// ResolveReviewReport updates report triage status and records audit action.
func (h *AdminHandler) ResolveReviewReport(w http.ResponseWriter, r *http.Request) {
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

	var payload resolveReviewReportPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.ResolveReviewReport(r.Context(), actor, domain.ReportID(id), payload.Status, payload.Action); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"resolved": true})
}

type moderateReviewPayload struct {
	Status domain.ReviewStatus `json:"status"` // VISIBLE or REMOVED_BY_ADMIN
	Reason string              `json:"reason"`
}

// ModerateReview changes visibility of a review and audits the action.
func (h *AdminHandler) ModerateReview(w http.ResponseWriter, r *http.Request) {
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

	var payload moderateReviewPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.ModerateReview(r.Context(), actor, domain.ReviewID(id), payload.Status, payload.Reason); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"moderated": true})
}

// ListPendingBusinessReports returns community reports filed against venues.
func (h *AdminHandler) ListPendingBusinessReports(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	reports, err := h.adminService.ListPendingBusinessReports(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(reports) == limit
	response.JSONWithPagination(w, http.StatusOK, reports, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

type resolveBusinessReportPayload struct {
	Status domain.ReportStatus `json:"status"`
	Notes  *string             `json:"notes"`
}

// ResolveBusinessReport marks a business report resolved or dismissed.
func (h *AdminHandler) ResolveBusinessReport(w http.ResponseWriter, r *http.Request) {
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

	var payload resolveBusinessReportPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.ResolveBusinessReport(r.Context(), actor, domain.ReportID(id), payload.Status, payload.Notes); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"resolved": true})
}

type setPublicationPayload struct {
	Status domain.PublicationStatus `json:"status"` // PUBLISHED or UNPUBLISHED
}

// SetBusinessPublicationStatus toggles public discovery visibility of a venue.
func (h *AdminHandler) SetBusinessPublicationStatus(w http.ResponseWriter, r *http.Request) {
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

	var payload setPublicationPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.SetBusinessPublicationStatus(r.Context(), actor, domain.BusinessID(id), payload.Status); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"updated": true})
}

type setVerificationPayload struct {
	Status domain.VerificationStatus `json:"status"` // VERIFIED or UNVERIFIED
}

// SetBusinessVerificationStatus toggles official verified badge of a venue.
func (h *AdminHandler) SetBusinessVerificationStatus(w http.ResponseWriter, r *http.Request) {
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

	var payload setVerificationPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.adminService.SetBusinessVerificationStatus(r.Context(), actor, domain.BusinessID(id), payload.Status); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"updated": true})
}

// ListAdminActions returns immutable audit log history.
func (h *AdminHandler) ListAdminActions(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	actions, err := h.adminService.ListAdminActions(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(actions) == limit
	response.JSONWithPagination(w, http.StatusOK, actions, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}
