package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// UpdateBusinessInput defines mutable business profile fields available to authorized managers.
type UpdateBusinessInput struct {
	Name                 *string                  `json:"name"`
	Category             *domain.BusinessCategory `json:"category"`
	Cuisine              *string                  `json:"cuisine"`
	Description          *string                  `json:"description"`
	ShortDescription     *string                  `json:"short_description"`
	Phone                *string                  `json:"phone"`
	Website              *string                  `json:"website"`
	LocationName         *string                  `json:"location_name"`
	Neighborhood         *string                  `json:"neighborhood"`
	AddressLine          *string                  `json:"address_line"`
	Latitude             *float64                 `json:"latitude"`
	Longitude            *float64                 `json:"longitude"`
	PriceRange           *string                  `json:"price_range"`
	Tags                 []string                 `json:"tags"`
	PaymentMethods       []string                 `json:"payment_methods"`
	ReservationAvailable *bool                    `json:"reservation_available"`
}

// BusinessService orchestrates venue management operations for authenticated managers.
type BusinessService interface {
	GetManagedBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (*domain.Business, error)
	GetManagedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error)
	UpdateBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input UpdateBusinessInput) (*domain.Business, error)
	SetHours(ctx context.Context, actor *domain.User, businessID domain.BusinessID, hours []domain.BusinessHours) error
	GetHours(ctx context.Context, actor *domain.User, businessID domain.BusinessID) ([]domain.BusinessHours, error)
	SetImages(ctx context.Context, actor *domain.User, businessID domain.BusinessID, images []domain.BusinessImage) error
	GetImages(ctx context.Context, actor *domain.User, businessID domain.BusinessID) ([]domain.BusinessImage, error)
}

type businessService struct {
	businessRepo  repository.BusinessRepository
	ownershipRepo repository.OwnershipRepository
	authzService  AuthorizationService
	txMgr         repository.TxManager
}

// NewBusinessService constructs a BusinessService.
func NewBusinessService(
	businessRepo repository.BusinessRepository,
	ownershipRepo repository.OwnershipRepository,
	authzService AuthorizationService,
	txMgr repository.TxManager,
) BusinessService {
	return &businessService{
		businessRepo:  businessRepo,
		ownershipRepo: ownershipRepo,
		authzService:  authzService,
		txMgr:         txMgr,
	}
}

func (s *businessService) GetManagedBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (*domain.Business, error) {
	if err := s.authzService.CanViewBusinessManagement(ctx, actor, businessID); err != nil {
		return nil, err
	}

	return s.businessRepo.GetByID(ctx, businessID)
}

func (s *businessService) GetManagedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	// Platform administrators can see all businesses if needed, or their managed businesses
	if actor.IsAdmin() {
		return s.businessRepo.List(ctx, domain.BusinessFilter{Limit: 100})
	}

	return s.ownershipRepo.GetManagedBusinesses(ctx, actor.ID)
}

func (s *businessService) UpdateBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID, input UpdateBusinessInput) (*domain.Business, error) {
	if err := s.authzService.CanEditBusiness(ctx, actor, businessID); err != nil {
		return nil, err
	}

	b, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, err
	}

	if input.Name != nil {
		trimmedName := strings.TrimSpace(*input.Name)
		if trimmedName == "" {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "business name cannot be empty", nil)
		}
		if len(trimmedName) > 100 {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "business name cannot exceed 100 characters", nil)
		}
		b.Name = trimmedName
	}
	if input.Category != nil {
		switch *input.Category {
		case domain.CategoryRestaurant, domain.CategoryCafe, domain.CategoryDrinks:
			b.Category = *input.Category
		default:
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid business category", nil)
		}
	}
	if input.Cuisine != nil {
		b.Cuisine = strings.TrimSpace(*input.Cuisine)
	}
	if input.Description != nil {
		if len(*input.Description) > 2000 {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "description cannot exceed 2000 characters", nil)
		}
		b.Description = *input.Description
	}
	if input.ShortDescription != nil {
		b.ShortDescription = *input.ShortDescription
	}
	if input.Phone != nil {
		trimmedPhone := strings.TrimSpace(*input.Phone)
		if trimmedPhone != "" {
			normalized, err := domain.NormalizePhone(trimmedPhone)
			if err != nil {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid phone format", err)
			}
			b.Phone = normalized
		}
	}
	if input.Website != nil {
		trimmedWeb := strings.TrimSpace(*input.Website)
		if trimmedWeb != "" {
			if !strings.HasPrefix(trimmedWeb, "http://") && !strings.HasPrefix(trimmedWeb, "https://") {
				return nil, domain.NewDomainError(domain.ErrInvalidInput, "website must start with http:// or https://", nil)
			}
			b.Website = &trimmedWeb
		} else {
			b.Website = nil
		}
	}
	if input.LocationName != nil {
		b.LocationName = strings.TrimSpace(*input.LocationName)
	}
	if input.Neighborhood != nil {
		b.Neighborhood = strings.TrimSpace(*input.Neighborhood)
	}
	if input.AddressLine != nil {
		trimmedAddr := strings.TrimSpace(*input.AddressLine)
		if trimmedAddr != "" {
			b.AddressLine = &trimmedAddr
		} else {
			b.AddressLine = nil
		}
	}
	if input.Latitude != nil {
		b.Latitude = input.Latitude
	}
	if input.Longitude != nil {
		b.Longitude = input.Longitude
	}
	if input.PriceRange != nil && strings.TrimSpace(*input.PriceRange) != "" {
		pr := domain.PriceRange(strings.TrimSpace(*input.PriceRange))
		switch pr {
		case domain.PriceRangeBudget, domain.PriceRangeModerate, domain.PriceRangeExpensive, domain.PriceRangeVeryExpensive:
			b.PriceRange = pr
		default:
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "invalid price range", nil)
		}
	}
	if input.Tags != nil {
		b.Tags = input.Tags
	}
	if input.PaymentMethods != nil {
		b.PaymentMethods = input.PaymentMethods
	}
	if input.ReservationAvailable != nil {
		b.ReservationAvailable = *input.ReservationAvailable
	}

	if err := s.businessRepo.Update(ctx, b); err != nil {
		return nil, err
	}

	return b, nil
}

func (s *businessService) SetHours(ctx context.Context, actor *domain.User, businessID domain.BusinessID, hours []domain.BusinessHours) error {
	if err := s.authzService.CanEditBusiness(ctx, actor, businessID); err != nil {
		return err
	}

	for i := range hours {
		hours[i].BusinessID = businessID
		if hours[i].DayOfWeek < 0 || hours[i].DayOfWeek > 6 {
			return domain.NewDomainError(domain.ErrInvalidInput, "day of week must be 0 (Sun) through 6 (Sat)", nil)
		}
	}

	return s.businessRepo.SetHours(ctx, businessID, hours)
}

func (s *businessService) GetHours(ctx context.Context, actor *domain.User, businessID domain.BusinessID) ([]domain.BusinessHours, error) {
	if err := s.authzService.CanViewBusinessManagement(ctx, actor, businessID); err != nil {
		return nil, err
	}

	return s.businessRepo.GetHours(ctx, businessID)
}

func (s *businessService) SetImages(ctx context.Context, actor *domain.User, businessID domain.BusinessID, images []domain.BusinessImage) error {
	if err := s.authzService.CanEditBusiness(ctx, actor, businessID); err != nil {
		return err
	}

	for i := range images {
		images[i].BusinessID = businessID
		if strings.TrimSpace(images[i].ImageURL) == "" {
			return domain.NewDomainError(domain.ErrInvalidInput, "image URL cannot be empty", nil)
		}
	}

	return s.businessRepo.SetImages(ctx, businessID, images)
}

func (s *businessService) GetImages(ctx context.Context, actor *domain.User, businessID domain.BusinessID) ([]domain.BusinessImage, error) {
	if err := s.authzService.CanViewBusinessManagement(ctx, actor, businessID); err != nil {
		return nil, err
	}

	return s.businessRepo.GetImages(ctx, businessID)
}
