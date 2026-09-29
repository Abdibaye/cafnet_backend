package handler

import (
	"net/http"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/http/middleware"
	"github.com/Abdibaye/cafnet_backend/internal/http/request"
	"github.com/Abdibaye/cafnet_backend/internal/http/response"
	"github.com/Abdibaye/cafnet_backend/internal/service"
)

// ReservationHandler adapts customer booking requests and manager reservation lifecycles.
type ReservationHandler struct {
	reservationService service.ReservationService
}

// NewReservationHandler constructs a ReservationHandler.
func NewReservationHandler(reservationService service.ReservationService) *ReservationHandler {
	return &ReservationHandler{reservationService: reservationService}
}

// CreateReservation initiates a table reservation request.
func (h *ReservationHandler) CreateReservation(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var input service.CreateReservationInput
	if err := request.DecodeJSON(r, &input, 8192); err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.reservationService.CreateReservation(r.Context(), actor, input)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusCreated, res)
}

// ListCustomerReservations returns all reservations for the authenticated customer.
func (h *ReservationHandler) ListCustomerReservations(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	reservations, err := h.reservationService.ListCustomerReservations(r.Context(), actor, limit, offset)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(reservations) == limit
	response.JSONWithPagination(w, http.StatusOK, reservations, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetCustomerReservation returns a single customer reservation by ID.
func (h *ReservationHandler) GetCustomerReservation(w http.ResponseWriter, r *http.Request) {
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

	res, err := h.reservationService.GetCustomerReservation(r.Context(), actor, domain.ReservationID(id))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

// CancelReservation allows the dining customer to cancel a pending or confirmed booking.
func (h *ReservationHandler) CancelReservation(w http.ResponseWriter, r *http.Request) {
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

	if err := h.reservationService.CancelReservation(r.Context(), actor, domain.ReservationID(id)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"cancelled": true})
}

// ListBusinessReservations provides venue managers with booking schedules and status filters.
func (h *ReservationHandler) ListBusinessReservations(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	limit := request.QueryInt(r, "limit", 20)
	offset := request.QueryInt(r, "offset", 0)

	filter := service.ReservationFilterInput{
		Limit:  limit,
		Offset: offset,
	}

	if statusStr := r.URL.Query().Get("status"); statusStr != "" {
		st := domain.ReservationStatus(statusStr)
		filter.Status = &st
	}
	if dateStr := r.URL.Query().Get("date"); dateStr != "" {
		filter.ReservationDate = &dateStr
	}

	reservations, err := h.reservationService.ListBusinessReservations(r.Context(), actor, domain.BusinessID(businessID), filter)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	hasMore := len(reservations) == limit
	response.JSONWithPagination(w, http.StatusOK, reservations, &response.Pagination{
		Limit:   limit,
		Offset:  offset,
		HasMore: hasMore,
	})
}

// GetBusinessReservation returns a specific reservation for venue managers.
func (h *ReservationHandler) GetBusinessReservation(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	resID, err := request.PathUUID(r, "resID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	res, err := h.reservationService.GetBusinessReservation(r.Context(), actor, domain.BusinessID(businessID), domain.ReservationID(resID))
	if err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, res)
}

// ConfirmReservation confirms a pending table reservation.
func (h *ReservationHandler) ConfirmReservation(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	resID, err := request.PathUUID(r, "resID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reservationService.ConfirmReservation(r.Context(), actor, domain.BusinessID(businessID), domain.ReservationID(resID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"confirmed": true})
}

type declinePayload struct {
	Reason string `json:"reason"`
}

// DeclineReservation declines a pending reservation with a reason.
func (h *ReservationHandler) DeclineReservation(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	resID, err := request.PathUUID(r, "resID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	var payload declinePayload
	if err := request.DecodeJSON(r, &payload, 4096); err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reservationService.DeclineReservation(r.Context(), actor, domain.BusinessID(businessID), domain.ReservationID(resID), payload.Reason); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"declined": true})
}

// CompleteReservation marks a dining visit completed, unlocking verified reviews.
func (h *ReservationHandler) CompleteReservation(w http.ResponseWriter, r *http.Request) {
	actor, err := middleware.MustCurrentUser(r)
	if err != nil {
		response.Error(w, r, err)
		return
	}

	businessID, err := request.PathInt64(r, "id")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	resID, err := request.PathUUID(r, "resID")
	if err != nil {
		response.Error(w, r, err)
		return
	}

	if err := h.reservationService.CompleteReservation(r.Context(), actor, domain.BusinessID(businessID), domain.ReservationID(resID)); err != nil {
		response.Error(w, r, err)
		return
	}

	response.JSON(w, http.StatusOK, map[string]bool{"completed": true})
}
