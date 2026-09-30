package service

import (
	"context"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// CreateUpdateInput specifies fields required to broadcast an announcement, event, or special offer.
type CreateUpdateInput struct {
	UpdateType   domain.UpdateType `json:"update_type"`
	Title        string            `json:"title"`
	Description  string            `json:"description"`
	EventDate    *string           `json:"event_date,omitempty"`
	EventTime    *string           `json:"event_time,omitempty"`
	PriceText    *string           `json:"price_text,omitempty"`
	OfferDetails *string           `json:"offer_details,omitempty"`
	ValidUntil   *string           `json:"valid_until,omitempty"`
	ExpiresAt    *time.Time        `json:"expires_at,omitempty"`
}

// UpdateUpdateInput specifies mutable fields for an existing update broadcast.
type UpdateUpdateInput struct {
	UpdateType   *domain.UpdateType   `json:"update_type,omitempty"`
	Title        *string              `json:"title,omitempty"`
	Description  *string              `json:"description,omitempty"`
	EventDate    *string              `json:"event_date,omitempty"`
	EventTime    *string              `json:"event_time,omitempty"`
	PriceText    *string              `json:"price_text,omitempty"`
	OfferDetails *string              `json:"offer_details,omitempty"`
	ValidUntil   *string              `json:"valid_until,omitempty"`
	ExpiresAt    *time.Time           `json:"expires_at,omitempty"`
	Status       *domain.UpdateStatus `json:"status,omitempty"`
}

// UpdatesService orchestrates time-sensitive venue broadcasts and events.
type UpdatesService interface {
	CreateUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateUpdateInput) (*domain.BusinessUpdate, error)
	UpdateUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, updateID domain.UpdateID, input UpdateUpdateInput) (*domain.BusinessUpdate, error)
	DeleteUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, updateID domain.UpdateID) error
	ListBusinessUpdates(ctx context.Context, actor *domain.User, businessID domain.BusinessID, includeExpired bool) ([]domain.BusinessUpdate, error)
	ListActiveUpdates(ctx context.Context, limit, offset int) ([]domain.BusinessUpdate, error)
	ListFollowingUpdates(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessUpdate, error)
}

type updatesService struct {
	updateRepo       repository.UpdateRepository
	authzService     AuthorizationService
	customerListRepo repository.CustomerListRepository
}

// NewUpdatesService constructs an UpdatesService.
func NewUpdatesService(
	updateRepo repository.UpdateRepository,
	authzService AuthorizationService,
	customerListRepo repository.CustomerListRepository,
) UpdatesService {
	return &updatesService{
		updateRepo:       updateRepo,
		authzService:     authzService,
		customerListRepo: customerListRepo,
	}
}

func isValidUpdateType(t domain.UpdateType) bool {
	switch t {
	case domain.UpdateTypeNewMenu,
		domain.UpdateTypeOffer,
		domain.UpdateTypeEvent,
		domain.UpdateTypePriceUpdate,
		domain.UpdateTypeAnnouncement:
		return true
	default:
		return false
	}
}

func deriveExpiration(updateType domain.UpdateType, validUntil, eventDate *string, expiresAt *time.Time) (*time.Time, error) {
	now := time.Now().UTC()

	if expiresAt != nil {
		if expiresAt.Before(now.Add(-1 * time.Minute)) {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "expiration cannot be in the past", nil)
		}
		return expiresAt, nil
	}

	if updateType == domain.UpdateTypeOffer {
		if validUntil == nil || strings.TrimSpace(*validUntil) == "" {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "offer requires valid_until or expires_at", nil)
		}
		vu := strings.TrimSpace(*validUntil)
		if t, err := time.Parse("2006-01-02", vu); err == nil {
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
			if endOfDay.Before(now) {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "offer expiration date cannot be in the past", nil)
			}
			return &endOfDay, nil
		}
		if t, err := time.Parse(time.RFC3339, vu); err == nil {
			if t.Before(now) {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "offer expiration date cannot be in the past", nil)
			}
			return &t, nil
		}
	}

	if updateType == domain.UpdateTypeEvent && eventDate != nil && strings.TrimSpace(*eventDate) != "" {
		ed := strings.TrimSpace(*eventDate)
		if t, err := time.Parse("2006-01-02", ed); err == nil {
			endOfDay := time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, time.UTC)
			if endOfDay.Before(now) {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "event date cannot be in the past", nil)
			}
			return &endOfDay, nil
		}
	}

	return nil, nil
}

func (s *updatesService) CreateUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateUpdateInput) (*domain.BusinessUpdate, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if err := s.authzService.CanPublishUpdate(ctx, actor, businessID); err != nil {
		return nil, err
	}

	trimmedTitle := strings.TrimSpace(input.Title)
	trimmedDesc := strings.TrimSpace(input.Description)
	if trimmedTitle == "" || len(trimmedTitle) > 120 || trimmedDesc == "" || len(trimmedDesc) > 5000 {
		return nil, domain.ErrInvalidInput
	}

	if !isValidUpdateType(input.UpdateType) {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid update type", nil)
	}

	expiresAt, err := deriveExpiration(input.UpdateType, input.ValidUntil, input.EventDate, input.ExpiresAt)
	if err != nil {
		return nil, err
	}

	update := &domain.BusinessUpdate{
		BusinessID:   businessID,
		AuthorUserID: actor.ID,
		UpdateType:   input.UpdateType,
		Title:        trimmedTitle,
		Description:  trimmedDesc,
		Status:       domain.UpdateStatusPublished,
		EventDate:    input.EventDate,
		EventTime:    input.EventTime,
		PriceText:    input.PriceText,
		OfferDetails: input.OfferDetails,
		ValidUntil:   input.ValidUntil,
		ExpiresAt:    expiresAt,
	}

	if err := s.updateRepo.Create(ctx, update); err != nil {
		return nil, err
	}

	return update, nil
}

func (s *updatesService) UpdateUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, updateID domain.UpdateID, input UpdateUpdateInput) (*domain.BusinessUpdate, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if err := s.authzService.CanPublishUpdate(ctx, actor, businessID); err != nil {
		return nil, err
	}

	update, err := s.updateRepo.GetByID(ctx, updateID)
	if err != nil {
		return nil, err
	}

	if update.BusinessID != businessID {
		return nil, domain.ErrResourceNotManaged
	}

	if input.Title != nil {
		trimmed := strings.TrimSpace(*input.Title)
		if trimmed == "" || len(trimmed) > 120 {
			return nil, domain.ErrInvalidInput
		}
		update.Title = trimmed
	}
	if input.Description != nil {
		trimmed := strings.TrimSpace(*input.Description)
		if trimmed == "" || len(trimmed) > 5000 {
			return nil, domain.ErrInvalidInput
		}
		update.Description = trimmed
	}
	if input.UpdateType != nil {
		if !isValidUpdateType(*input.UpdateType) {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid update type", nil)
		}
		update.UpdateType = *input.UpdateType
	}
	if input.EventDate != nil {
		update.EventDate = input.EventDate
	}
	if input.EventTime != nil {
		update.EventTime = input.EventTime
	}
	if input.PriceText != nil {
		update.PriceText = input.PriceText
	}
	if input.OfferDetails != nil {
		update.OfferDetails = input.OfferDetails
	}
	if input.ValidUntil != nil {
		update.ValidUntil = input.ValidUntil
	}
	if input.ExpiresAt != nil {
		if input.ExpiresAt.Before(time.Now().UTC().Add(-1 * time.Minute)) {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "expiration cannot be in the past", nil)
		}
		update.ExpiresAt = input.ExpiresAt
	} else if input.ValidUntil != nil || input.EventDate != nil {
		derived, err := deriveExpiration(update.UpdateType, update.ValidUntil, update.EventDate, nil)
		if err != nil {
			return nil, err
		}
		if derived != nil {
			update.ExpiresAt = derived
		}
	}
	if input.Status != nil {
		update.Status = *input.Status
	}

	if err := s.updateRepo.Update(ctx, update); err != nil {
		return nil, err
	}

	return update, nil
}

func (s *updatesService) DeleteUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, updateID domain.UpdateID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	if err := s.authzService.CanPublishUpdate(ctx, actor, businessID); err != nil {
		return err
	}

	update, err := s.updateRepo.GetByID(ctx, updateID)
	if err != nil {
		return err
	}

	if update.BusinessID != businessID {
		return domain.ErrResourceNotManaged
	}

	return s.updateRepo.Delete(ctx, updateID)
}

func (s *updatesService) ListBusinessUpdates(ctx context.Context, actor *domain.User, businessID domain.BusinessID, includeExpired bool) ([]domain.BusinessUpdate, error) {
	// If requesting expired updates, verify user is an authorized manager or platform admin
	if includeExpired {
		canManage, err := s.authzService.CanManageBusiness(ctx, actor, businessID)
		if err != nil || !canManage {
			// Unauthorized callers are strictly scoped to non-expired active updates
			includeExpired = false
		}
	}

	return s.updateRepo.ListByBusiness(ctx, businessID, includeExpired)
}

func (s *updatesService) ListActiveUpdates(ctx context.Context, limit, offset int) ([]domain.BusinessUpdate, error) {
	return s.updateRepo.ListActive(ctx, nil, limit, offset)
}

func (s *updatesService) ListFollowingUpdates(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessUpdate, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	followed, err := s.customerListRepo.ListFollowedBusinesses(ctx, actor.ID)
	if err != nil {
		return nil, err
	}
	if len(followed) == 0 {
		return []domain.BusinessUpdate{}, nil
	}

	ids := make([]domain.BusinessID, len(followed))
	for i, b := range followed {
		ids[i] = b.ID
	}

	return s.updateRepo.ListActiveByBusinessIDs(ctx, ids, limit, offset)
}

