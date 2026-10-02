package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

// CreateSubmissionInput encapsulates data required to submit a new venue for administrative review.
type CreateSubmissionInput struct {
	BusinessName string                  `json:"business_name"`
	Category     domain.BusinessCategory `json:"category"`
	Area         string                  `json:"area"`
	MediaID      *domain.MediaID         `json:"media_id,omitempty"`
	Payload      map[string]any          `json:"payload"`
}

// CustomerService orchestrates customer-facing saved lists, follows, claims, and submissions.
type CustomerService interface {
	// Saved Places
	SaveBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error
	UnsaveBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error
	ListSavedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error)
	IsBusinessSaved(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (bool, error)

	// Followed Venues
	FollowBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error
	UnfollowBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error
	ListFollowedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error)
	IsBusinessFollowed(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (bool, error)

	// Ownership Claims
	SubmitClaim(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (*domain.BusinessClaim, error)
	ListMyClaims(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessClaim, error)
	GetMyClaim(ctx context.Context, actor *domain.User, claimID domain.ClaimID) (*domain.BusinessClaim, error)

	// Venue Submissions
	CreateSubmission(ctx context.Context, actor *domain.User, input CreateSubmissionInput) (*domain.BusinessSubmission, error)
	ListMySubmissions(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessSubmission, error)
	GetMySubmission(ctx context.Context, actor *domain.User, submissionID domain.SubmissionID) (*domain.BusinessSubmission, error)
}

type customerService struct {
	customerListRepo repository.CustomerListRepository
	businessRepo     repository.BusinessRepository
	claimRepo        repository.ClaimRepository
	submissionRepo   repository.SubmissionRepository
	mediaRepo        repository.MediaRepository
	storage          storage.ObjectStorage
}

// NewCustomerService constructs a CustomerService.
func NewCustomerService(
	customerListRepo repository.CustomerListRepository,
	businessRepo repository.BusinessRepository,
	claimRepo repository.ClaimRepository,
	submissionRepo repository.SubmissionRepository,
	mediaRepo repository.MediaRepository,
	objStorage storage.ObjectStorage,
) CustomerService {
	return &customerService{
		customerListRepo: customerListRepo,
		businessRepo:     businessRepo,
		claimRepo:        claimRepo,
		submissionRepo:   submissionRepo,
		mediaRepo:        mediaRepo,
		storage:          objStorage,
	}
}

func (s *customerService) SaveBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	// Verify target business exists
	_, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return err
	}

	return s.customerListRepo.SaveBusiness(ctx, actor.ID, businessID)
}

func (s *customerService) UnsaveBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	return s.customerListRepo.UnsaveBusiness(ctx, actor.ID, businessID)
}

func (s *customerService) ListSavedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	return s.customerListRepo.ListSavedBusinesses(ctx, actor.ID)
}

func (s *customerService) IsBusinessSaved(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (bool, error) {
	if actor == nil {
		return false, domain.ErrUnauthenticated
	}

	return s.customerListRepo.IsSaved(ctx, actor.ID, businessID)
}

func (s *customerService) FollowBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	// Verify target business exists
	_, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return err
	}

	return s.customerListRepo.FollowBusiness(ctx, actor.ID, businessID)
}

func (s *customerService) UnfollowBusiness(ctx context.Context, actor *domain.User, businessID domain.BusinessID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	return s.customerListRepo.UnfollowBusiness(ctx, actor.ID, businessID)
}

func (s *customerService) ListFollowedBusinesses(ctx context.Context, actor *domain.User) ([]domain.Business, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	return s.customerListRepo.ListFollowedBusinesses(ctx, actor.ID)
}

func (s *customerService) IsBusinessFollowed(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (bool, error) {
	if actor == nil {
		return false, domain.ErrUnauthenticated
	}

	return s.customerListRepo.IsFollowing(ctx, actor.ID, businessID)
}

func (s *customerService) SubmitClaim(ctx context.Context, actor *domain.User, businessID domain.BusinessID) (*domain.BusinessClaim, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	// Verify target business exists
	b, err := s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return nil, err
	}

	// Cannot claim an already claimed business
	if b.ClaimStatus == domain.ClaimStatusClaimed {
		return nil, domain.NewDomainError(domain.ErrConflict, "business is already claimed and verified", nil)
	}

	// Check if a pending claim already exists for this business
	existingPending, err := s.claimRepo.GetPendingForBusiness(ctx, businessID)
	if err == nil && existingPending != nil {
		return nil, domain.NewDomainError(domain.ErrConflict, "a pending claim is already under review for this venue", nil)
	}

	claim := &domain.BusinessClaim{
		BusinessID:     businessID,
		ClaimantUserID: actor.ID,
		Status:         domain.ClaimStatusPending,
	}

	if err := s.claimRepo.Create(ctx, claim); err != nil {
		return nil, err
	}

	return claim, nil
}

func (s *customerService) ListMyClaims(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessClaim, error) {
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

	return s.claimRepo.ListByClaimant(ctx, actor.ID, limit, offset)
}

func (s *customerService) GetMyClaim(ctx context.Context, actor *domain.User, claimID domain.ClaimID) (*domain.BusinessClaim, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	claim, err := s.claimRepo.GetByID(ctx, claimID)
	if err != nil {
		return nil, err
	}

	if claim.ClaimantUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	return claim, nil
}

func (s *customerService) CreateSubmission(ctx context.Context, actor *domain.User, input CreateSubmissionInput) (*domain.BusinessSubmission, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	name := strings.TrimSpace(input.BusinessName)
	if len(name) < 2 || len(name) > 150 {
		return nil, domain.ErrInvalidInput
	}

	area := strings.TrimSpace(input.Area)
	if len(area) < 2 || len(area) > 100 {
		return nil, domain.ErrInvalidInput
	}

	category := domain.BusinessCategory(strings.ToLower(strings.TrimSpace(string(input.Category))))
	switch category {
	case domain.CategoryRestaurant, domain.CategoryCafe, domain.CategoryDrinks:
	default:
		return nil, domain.ErrInvalidInput
	}

	payload := input.Payload
	if payload == nil {
		payload = make(map[string]any)
	}

	// Initial business submission supports exactly ONE business image
	var mediaID *domain.MediaID
	if input.MediaID != nil && strings.TrimSpace(string(*input.MediaID)) != "" {
		mediaID = input.MediaID
	} else if rawMediaID, ok := payload["media_id"].(string); ok && strings.TrimSpace(rawMediaID) != "" {
		mid := domain.MediaID(strings.TrimSpace(rawMediaID))
		mediaID = &mid
	}

	if mediaID == nil {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "business image is required for submission", nil)
	}

	var med *domain.Media
	if mediaID != nil {
		// Verify image exists, is finalized/READY, and belongs to this submitter
		var err error
		med, err = s.mediaRepo.GetByID(ctx, *mediaID)
		if err != nil {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "submitted business image not found", err)
		}
		if med.Status != domain.MediaStatusReady {
			return nil, domain.NewDomainError(domain.ErrInvalidInput, "submitted business image is not ready; upload must complete first", nil)
		}
		if med.CreatedByUserID != actor.ID && !actor.IsAdmin() {
			return nil, domain.ErrForbidden
		}
	}

	sub := &domain.BusinessSubmission{
		SubmitterUserID:  actor.ID,
		BusinessName:     name,
		Category:         category,
		Area:             area,
		SubmittedPayload: payload,
		Status:           domain.SubmissionStatusPending,
	}
	if med != nil {
		sub.CoverMediaID = &med.ID
	}

	if err := s.submissionRepo.Create(ctx, sub); err != nil {
		return nil, err
	}

	// Link media to this submission
	if med != nil {
		if err := s.mediaRepo.AssociateWithSubmission(ctx, med.ID, sub.ID); err != nil {
			return nil, err
		}

		if s.storage != nil {
			coverURL := s.storage.PublicURL(med.ObjectKey)
			sub.CoverImageURL = &coverURL
		}
	}

	return sub, nil
}

func (s *customerService) ListMySubmissions(ctx context.Context, actor *domain.User, limit, offset int) ([]domain.BusinessSubmission, error) {
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

	subs, err := s.submissionRepo.ListBySubmitter(ctx, actor.ID, limit, offset)
	if err != nil {
		return nil, err
	}

	if s.storage != nil && s.mediaRepo != nil {
		for i := range subs {
			if subs[i].CoverMediaID != nil {
				if med, err := s.mediaRepo.GetByID(ctx, *subs[i].CoverMediaID); err == nil && med != nil {
					u := s.storage.PublicURL(med.ObjectKey)
					subs[i].CoverImageURL = &u
				}
			}
		}
	}

	return subs, nil
}

func (s *customerService) GetMySubmission(ctx context.Context, actor *domain.User, submissionID domain.SubmissionID) (*domain.BusinessSubmission, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	sub, err := s.submissionRepo.GetByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	if sub.SubmitterUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	if s.storage != nil && s.mediaRepo != nil && sub.CoverMediaID != nil {
		if med, err := s.mediaRepo.GetByID(ctx, *sub.CoverMediaID); err == nil && med != nil {
			u := s.storage.PublicURL(med.ObjectKey)
			sub.CoverImageURL = &u
		}
	}

	return sub, nil
}
