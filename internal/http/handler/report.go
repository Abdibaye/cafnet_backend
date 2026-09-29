package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// ReportHandler adapts customer incident reporting for reviews and venues.
type ReportHandler struct {
	reportService service.ReportService
}

// NewReportHandler constructs a ReportHandler.
func NewReportHandler(reportService service.ReportService) *ReportHandler {
	return &ReportHandler{reportService: reportService}
}

type createReviewReportPayload struct {
	Reason      domain.ReviewReportReason `json:"reason"`
	Explanation string                    `json:"explanation"`
}

// ReportReview records a customer flag against an inappropriate or fake review.
func (h *ReportHandler) ReportReview(w http.ResponseWriter, r *http.Request) {
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

	var payload createReviewReportPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	rep, err := h.reportService.CreateReviewReport(r.Context(), actor, service.CreateReviewReportInput{
		ReviewID:    domain.ReviewID(id),
		Reason:      payload.Reason,
		Explanation: payload.Explanation,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, rep)
}

type createBusinessReportPayload struct {
	Reason  domain.BusinessReportReason `json:"reason"`
	Details *string                     `json:"details"`
}

// ReportBusiness records a customer flag regarding inaccurate venue details or policy breaches.
func (h *ReportHandler) ReportBusiness(w http.ResponseWriter, r *http.Request) {
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

	var payload createBusinessReportPayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	rep, err := h.reportService.CreateBusinessReport(r.Context(), actor, service.CreateBusinessReportInput{
		BusinessID: domain.BusinessID(id),
		Reason:     payload.Reason,
		Details:    payload.Details,
	})
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, rep)
}
