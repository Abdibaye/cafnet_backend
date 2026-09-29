package domain

import (
	"time"
)

// Reservation represents a table booking contract between a customer and a business.
type Reservation struct {
	ID              ReservationID     `json:"id"`
	ReservationCode string            `json:"reservation_code"` // Format: "CAF-XXXX"
	BusinessID      BusinessID        `json:"business_id"`
	CustomerUserID  UserID            `json:"customer_user_id"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	ReservationDate time.Time         `json:"reservation_date"` // Year-Month-Day
	ReservationTime string            `json:"reservation_time"` // Format: "HH:MM:SS" or "HH:MM"
	PartySize       int               `json:"party_size"`
	SpecialRequest  string            `json:"special_request"`
	Status          ReservationStatus `json:"status"`
	DeclineReason   *string           `json:"decline_reason,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	ConfirmedAt     *time.Time        `json:"confirmed_at,omitempty"`
	DeclinedAt      *time.Time        `json:"declined_at,omitempty"`
	CompletedAt     *time.Time        `json:"completed_at,omitempty"`
	CancelledAt     *time.Time        `json:"cancelled_at,omitempty"`
	ReviewedByUser  *UserID           `json:"reviewed_by_user,omitempty"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// IsTerminal returns true if the reservation lifecycle has finalized.
func (r *Reservation) IsTerminal() bool {
	return r.Status == ReservationStatusCompleted ||
		r.Status == ReservationStatusDeclined ||
		r.Status == ReservationStatusCancelled
}

// IsCompleted returns true if the guest has successfully completed their dining visit.
func (r *Reservation) IsCompleted() bool {
	return r.Status == ReservationStatusCompleted
}

// CanBeCancelled returns true if the reservation is in a state where customer cancellation is legal.
func (r *Reservation) CanBeCancelled() bool {
	return r.Status == ReservationStatusPending || r.Status == ReservationStatusConfirmed
}

// ReservationFilter defines query parameters for reservations.
type ReservationFilter struct {
	BusinessID      *BusinessID
	CustomerUserID  *UserID
	Status          *ReservationStatus
	ReservationDate *time.Time
	Limit           int
	Offset          int
}
