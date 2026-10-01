package service

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// CreateReservationInput encapsulates parameters required to book a table.
type CreateReservationInput struct {
	BusinessID      domain.BusinessID `json:"business_id"`
	CustomerName    string            `json:"customer_name"`
	CustomerPhone   string            `json:"customer_phone"`
	ReservationDate string            `json:"reservation_date"` // Format: "2006-01-02"
	ReservationTime string            `json:"reservation_time"` // Format: "15:04" or "15:04:05"
	PartySize       int               `json:"party_size"`
	SpecialRequest  *string           `json:"special_request"`
}

// ReservationFilterInput provides pagination and status filters for manager reservation views.
type ReservationFilterInput struct {
	Status          *domain.ReservationStatus `json:"status"`
	ReservationDate *string                   `json:"reservation_date"` // Format: "2006-01-02"
	Limit           int                       `json:"limit"`
	Offset          int                       `json:"offset"`
}

// ReservationService orchestrates customer table bookings and manager reservation lifecycles.
type ReservationService interface {
	// Customer use cases
	CreateReservation(ctx context.Context, actor *domain.User, input CreateReservationInput) (*domain.Reservation, error)
	GetCustomerReservation(ctx context.Context, actor *domain.User, id domain.ReservationID) (*domain.Reservation, error)
	ListCustomerReservations(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.Reservation, error)
	CancelReservation(ctx context.Context, actor *domain.User, id domain.ReservationID) error

	// Business Manager use cases
	ListBusinessReservations(ctx context.Context, actor *domain.User, businessID domain.BusinessID, filter ReservationFilterInput) ([]domain.Reservation, error)
	GetBusinessReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) (*domain.Reservation, error)
	ConfirmReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) error
	DeclineReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID, reason string) error
	CompleteReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) error
}

type reservationService struct {
	reservationRepo repository.ReservationRepository
	businessRepo    repository.BusinessRepository
	authzService    AuthorizationService
	txMgr           repository.TxManager
}

// NewReservationService constructs a ReservationService.
func NewReservationService(
	reservationRepo repository.ReservationRepository,
	businessRepo repository.BusinessRepository,
	authzService AuthorizationService,
	txMgr repository.TxManager,
) ReservationService {
	return &reservationService{
		reservationRepo: reservationRepo,
		businessRepo:    businessRepo,
		authzService:    authzService,
		txMgr:           txMgr,
	}
}

func generateReservationCode() string {
	n, _ := rand.Int(rand.Reader, big.NewInt(9000))
	num := n.Int64() + 1000
	return fmt.Sprintf("RES-%s-%04d", time.Now().Format("20060102"), num)
}

func (s *reservationService) CreateReservation(ctx context.Context, actor *domain.User, input CreateReservationInput) (*domain.Reservation, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if input.PartySize < 1 || input.PartySize > 50 {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "party size must be between 1 and 50", nil)
	}

	if strings.TrimSpace(input.ReservationDate) == "" || strings.TrimSpace(input.ReservationTime) == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "reservation date and time are required", nil)
	}

	parsedDate, err := time.Parse("2006-01-02", strings.TrimSpace(input.ReservationDate))
	if err != nil {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "reservation_date must be formatted as YYYY-MM-DD", err)
	}

	customerName := strings.TrimSpace(input.CustomerName)
	if customerName == "" {
		customerName = actor.FullName
	}
	if customerName == "" {
		customerName = "Guest"
	}

	customerPhone := strings.TrimSpace(input.CustomerPhone)
	if customerPhone == "" {
		customerPhone = actor.GetPhone()
	}
	if customerPhone == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "customer phone number is required", nil)
	}

	// Verify target business exists and offers reservations
	b, err := s.businessRepo.GetByID(ctx, input.BusinessID)
	if err != nil {
		return nil, err
	}
	if !b.ReservationAvailable {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "reservations are not enabled for this business", nil)
	}

	specialReq := ""
	if input.SpecialRequest != nil {
		specialReq = *input.SpecialRequest
	}

	res := &domain.Reservation{
		ReservationCode: generateReservationCode(),
		BusinessID:      input.BusinessID,
		CustomerUserID:  actor.ID,
		CustomerName:    customerName,
		CustomerPhone:   customerPhone,
		ReservationDate: parsedDate,
		ReservationTime: input.ReservationTime,
		PartySize:       input.PartySize,
		SpecialRequest:  specialReq,
		Status:          domain.ReservationStatusPending,
	}

	err = s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		return s.reservationRepo.Create(txCtx, res)
	})
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (s *reservationService) GetCustomerReservation(ctx context.Context, actor *domain.User, id domain.ReservationID) (*domain.Reservation, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	res, err := s.reservationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if res.CustomerUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	return res, nil
}

func (s *reservationService) ListCustomerReservations(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.Reservation, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.reservationRepo.List(ctx, domain.ReservationFilter{
		CustomerUserID: &actor.ID,
		Limit:          limit,
		Offset:         offset,
	})
}

func (s *reservationService) CancelReservation(ctx context.Context, actor *domain.User, id domain.ReservationID) error {
	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		res, err := s.reservationRepo.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		if err := s.authzService.AuthorizeReservationCancellation(actor, res.CustomerUserID, res.Status); err != nil {
			return err
		}

		return s.reservationRepo.Cancel(txCtx, id)
	})
}

func (s *reservationService) ListBusinessReservations(ctx context.Context, actor *domain.User, businessID domain.BusinessID, filter ReservationFilterInput) ([]domain.Reservation, error) {
	if err := s.authzService.CanManageReservations(ctx, actor, businessID); err != nil {
		return nil, err
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	var parsedDate *time.Time
	if filter.ReservationDate != nil && strings.TrimSpace(*filter.ReservationDate) != "" {
		d, err := time.Parse("2006-01-02", strings.TrimSpace(*filter.ReservationDate))
		if err == nil {
			parsedDate = &d
		}
	}

	return s.reservationRepo.List(ctx, domain.ReservationFilter{
		BusinessID:      &businessID,
		Status:          filter.Status,
		ReservationDate: parsedDate,
		Limit:           limit,
		Offset:          offset,
	})
}

func (s *reservationService) GetBusinessReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) (*domain.Reservation, error) {
	if err := s.authzService.CanManageReservations(ctx, actor, businessID); err != nil {
		return nil, err
	}

	res, err := s.reservationRepo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if res.BusinessID != businessID {
		return nil, domain.ErrResourceNotManaged
	}

	return res, nil
}

func (s *reservationService) ConfirmReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) error {
	if err := s.authzService.CanManageReservations(ctx, actor, businessID); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		res, err := s.reservationRepo.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		if res.BusinessID != businessID {
			return domain.ErrResourceNotManaged
		}

		// State machine invariant: only PENDING reservations may be CONFIRMED
		if res.Status != domain.ReservationStatusPending {
			return domain.NewDomainError(
				domain.ErrIllegalStateTransition,
				fmt.Sprintf("cannot confirm reservation in state %s; must be PENDING", res.Status),
				nil,
			)
		}

		return s.reservationRepo.Confirm(txCtx, id, &actor.ID)
	})
}

func (s *reservationService) DeclineReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID, reason string) error {
	if err := s.authzService.CanManageReservations(ctx, actor, businessID); err != nil {
		return err
	}

	if strings.TrimSpace(reason) == "" {
		return domain.NewDomainError(domain.ErrInvalidInput, "rejection reason is required", nil)
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		res, err := s.reservationRepo.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		if res.BusinessID != businessID {
			return domain.ErrResourceNotManaged
		}

		// State machine invariant: only PENDING reservations may be DECLINED
		if res.Status != domain.ReservationStatusPending {
			return domain.NewDomainError(
				domain.ErrIllegalStateTransition,
				fmt.Sprintf("cannot decline reservation in state %s; must be PENDING", res.Status),
				nil,
			)
		}

		return s.reservationRepo.Decline(txCtx, id, strings.TrimSpace(reason), &actor.ID)
	})
}

func (s *reservationService) CompleteReservation(ctx context.Context, actor *domain.User, businessID domain.BusinessID, id domain.ReservationID) error {
	if err := s.authzService.CanManageReservations(ctx, actor, businessID); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		res, err := s.reservationRepo.GetByID(txCtx, id)
		if err != nil {
			return err
		}

		if res.BusinessID != businessID {
			return domain.ErrResourceNotManaged
		}

		// State machine invariant: only CONFIRMED reservations can transition to COMPLETED
		if res.Status != domain.ReservationStatusConfirmed {
			return domain.NewDomainError(
				domain.ErrIllegalStateTransition,
				fmt.Sprintf("cannot complete reservation in state %s; must be CONFIRMED", res.Status),
				nil,
			)
		}

		return s.reservationRepo.Complete(txCtx, id, &actor.ID)
	})
}
