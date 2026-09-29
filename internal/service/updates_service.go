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
}

type updatesService struct {
	updateRepo   repository.UpdateRepository
	authzService AuthorizationService
}

// NewUpdatesService constructs an UpdatesService.
func NewUpdatesService(
	updateRepo repository.UpdateRepository,
	authzService AuthorizationService,
) UpdatesService {
	return &updatesService{
		updateRepo:   updateRepo,
		authzService: authzService,
	}
}

func isValidUpdateType(t domain.UpdateType) bool {
	switch t {
	case domain.UpdateTypeAnnouncement, domain.UpdateTypeOffer, domain.UpdateTypeEvent:
		return true
	default:
		return false
	}
}

func (s *updatesService) CreateUpdate(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input CreateUpdateInput) (*domain.BusinessUpdate, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if err := s.authzService.CanPublishUpdate(ctx, actor, businessID); err != nil {
		return nil, err
	}

	if strings.TrimSpace(input.Title) == "" || strings.TrimSpace(input.Description) == "" {
		return nil, domain.ErrInvalidInput
	}

	if !isValidUpdateType(input.UpdateType) {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid update type; must be ANNOUNCEMENT, OFFER, or EVENT", nil)
	}

	update := &domain.BusinessUpdate{
		BusinessID:   businessID,
		AuthorUserID: actor.ID,
		UpdateType:   input.UpdateType,
		Title:        strings.TrimSpace(input.Title),
		Description:  strings.TrimSpace(input.Description),
		Status:       domain.UpdateStatusPublished,
		EventDate:    input.EventDate,
		EventTime:    input.EventTime,
		PriceText:    input.PriceText,
		OfferDetails: input.OfferDetails,
		ValidUntil:   input.ValidUntil,
		ExpiresAt:    input.ExpiresAt,
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

	if input.Title != nil && strings.TrimSpace(*input.Title) != "" {
		update.Title = strings.TrimSpace(*input.Title)
	}
	if input.Description != nil && strings.TrimSpace(*input.Description) != "" {
		update.Description = strings.TrimSpace(*input.Description)
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
		update.ExpiresAt = input.ExpiresAt
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
