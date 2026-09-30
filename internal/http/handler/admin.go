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

// GetStats returns summary aggregate counters for the Admin Dashboard.
func (h *AdminHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	stats, err := h.adminService.GetAdminStats(r.Context(), actor)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, stats)
}

// ListSubmissions returns venue submissions with optional status filter.
func (h *AdminHandler) ListSubmissions(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)
	statusParam := r.URL.Query().Get("status")

	var statusPtr *domain.SubmissionWorkflowStatus
	if statusParam != "" && statusParam != "ALL" {
		s := domain.SubmissionWorkflowStatus(statusParam)
		statusPtr = &s
	}

	subs, err := h.adminService.ListSubmissions(r.Context(), actor, statusPtr, limit, offset)
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

// ListPendingSubmissions delegates to ListSubmissions for backwards compatibility.
func (h *AdminHandler) ListPendingSubmissions(w http.ResponseWriter, r *http.Request) {
	h.ListSubmissions(w, r)
}

// GetSubmission returns a single business submission by ID.
func (h *AdminHandler) GetSubmission(w http.ResponseWriter, r *http.Request) {
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

	sub, err := h.adminService.GetSubmission(r.Context(), actor, domain.SubmissionID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, sub)
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

// ListClaims returns business claims with optional status filter.
func (h *AdminHandler) ListClaims(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)
	statusParam := r.URL.Query().Get("status")

	var statusPtr *domain.ClaimWorkflowStatus
	if statusParam != "" && statusParam != "ALL" {
		c := domain.ClaimWorkflowStatus(statusParam)
		statusPtr = &c
	}

	claims, err := h.adminService.ListClaims(r.Context(), actor, statusPtr, limit, offset)
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

// ListPendingClaims delegates to ListClaims for backwards compatibility.
func (h *AdminHandler) ListPendingClaims(w http.ResponseWriter, r *http.Request) {
	h.ListClaims(w, r)
}

// GetClaim returns a single business claim by ID.
func (h *AdminHandler) GetClaim(w http.ResponseWriter, r *http.Request) {
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

	claim, err := h.adminService.GetClaim(r.Context(), actor, domain.ClaimID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, claim)
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

// ListReviewReports returns community reports filed against reviews with optional status filter.
func (h *AdminHandler) ListReviewReports(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)
	statusParam := r.URL.Query().Get("status")

	var statusPtr *domain.ReportStatus
	if statusParam != "" && statusParam != "ALL" {
		s := domain.ReportStatus(statusParam)
		statusPtr = &s
	}

	reports, err := h.adminService.ListReviewReports(r.Context(), actor, statusPtr, limit, offset)
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

// ListPendingReviewReports delegates to ListReviewReports for backwards compatibility.
func (h *AdminHandler) ListPendingReviewReports(w http.ResponseWriter, r *http.Request) {
	h.ListReviewReports(w, r)
}

// GetReviewReport returns a single review report by ID.
func (h *AdminHandler) GetReviewReport(w http.ResponseWriter, r *http.Request) {
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

	report, err := h.adminService.GetReviewReport(r.Context(), actor, domain.ReportID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, report)
}

// GetReview returns a single review by ID for administrative inspection.
func (h *AdminHandler) GetReview(w http.ResponseWriter, r *http.Request) {
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

	rev, err := h.adminService.GetReview(r.Context(), actor, domain.ReviewID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, rev)
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

// ListBusinessReports returns community reports filed against venues with optional status filter.
func (h *AdminHandler) ListBusinessReports(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)
	statusParam := r.URL.Query().Get("status")

	var statusPtr *domain.ReportStatus
	if statusParam != "" && statusParam != "ALL" {
		s := domain.ReportStatus(statusParam)
		statusPtr = &s
	}

	reports, err := h.adminService.ListBusinessReports(r.Context(), actor, statusPtr, limit, offset)
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

// ListPendingBusinessReports delegates to ListBusinessReports for backwards compatibility.
func (h *AdminHandler) ListPendingBusinessReports(w http.ResponseWriter, r *http.Request) {
	h.ListBusinessReports(w, r)
}

// GetBusinessReport returns a single business report by ID.
func (h *AdminHandler) GetBusinessReport(w http.ResponseWriter, r *http.Request) {
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

	report, err := h.adminService.GetBusinessReport(r.Context(), actor, domain.ReportID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, report)
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
