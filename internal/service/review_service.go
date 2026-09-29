package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// CreateReviewInput defines verified dining feedback payload.
type CreateReviewInput struct {
	ReservationID domain.ReservationID `json:"reservation_id"`
	Rating        int16                `json:"rating"` // 1 - 5
	ReviewText    string               `json:"review_text"`
}

// UpdateReviewInput defines mutable review feedback fields for the author.
type UpdateReviewInput struct {
	Rating     int16  `json:"rating"` // 1 - 5
	ReviewText string `json:"review_text"`
}

// ReviewService orchestrates verified dining reviews and helpful engagement.
type ReviewService interface {
	CreateReview(ctx context.Context, actor *domain.User, input CreateReviewInput) (*domain.Review, error)
	UpdateReview(ctx context.Context, actor *domain.User, reviewID domain.ReviewID, input UpdateReviewInput) (*domain.Review, error)
	DeleteReview(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error
	ListBusinessReviews(ctx context.Context, businessID domain.BusinessID, limit, offset int) ([]domain.Review, error)
	ListMyReviews(ctx context.Context, actor *domain.User) ([]domain.Review, error)
	GetReviewSummary(ctx context.Context, businessID domain.BusinessID) (*domain.ReviewSummary, error)
	MarkHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error
	UnmarkHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error
	HasVotedHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) (bool, error)
}

type reviewService struct {
	reviewRepo      repository.ReviewRepository
	reservationRepo repository.ReservationRepository
	authzService    AuthorizationService
	txMgr           repository.TxManager
}

// NewReviewService constructs a ReviewService.
func NewReviewService(
	reviewRepo repository.ReviewRepository,
	reservationRepo repository.ReservationRepository,
	authzService AuthorizationService,
	txMgr repository.TxManager,
) ReviewService {
	return &reviewService{
		reviewRepo:      reviewRepo,
		reservationRepo: reservationRepo,
		authzService:    authzService,
		txMgr:           txMgr,
	}
}

func (s *reviewService) CreateReview(ctx context.Context, actor *domain.User, input CreateReviewInput) (*domain.Review, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if input.Rating < 1 || input.Rating > 5 {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "rating must be an integer between 1 and 5", nil)
	}

	if strings.TrimSpace(input.ReviewText) == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "review text is required", nil)
	}

	// 1. Authoritative verification: Retrieve corresponding reservation
	res, err := s.reservationRepo.GetByID(ctx, input.ReservationID)
	if err != nil {
		return nil, err
	}

	// 2. Ownership check: Diner must match reservation customer
	if res.CustomerUserID != actor.ID {
		return nil, domain.ErrForbidden
	}

	// 3. Verified Visit invariant: reservation must be COMPLETED
	if res.Status != domain.ReservationStatusCompleted {
		return nil, domain.NewDomainError(
			domain.ErrIllegalStateTransition,
			"reviews can only be submitted for completed dining visits",
			nil,
		)
	}

	// 4. Duplicate check: A single completed reservation cannot be reviewed multiple times
	existingReview, err := s.reviewRepo.GetByReservationID(ctx, res.ID)
	if err == nil && existingReview != nil {
		return nil, domain.NewDomainError(
			domain.ErrConflict,
			"a verified review has already been submitted for this completed visit",
			nil,
		)
	}

	var review *domain.Review
	err = s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		rev := &domain.Review{
			BusinessID:      res.BusinessID,
			CustomerUserID:  actor.ID,
			ReservationID:   res.ID,
			Rating:          input.Rating,
			ReviewText:      strings.TrimSpace(input.ReviewText),
			IsVerifiedVisit: true, // Authoritatively verified via completed reservation
			Status:          domain.ReviewStatusVisible,
		}

		if err := s.reviewRepo.Create(txCtx, rev); err != nil {
			return err
		}

		review = rev
		return nil
	})

	if err != nil {
		return nil, err
	}

	return review, nil
}

func (s *reviewService) UpdateReview(ctx context.Context, actor *domain.User, reviewID domain.ReviewID, input UpdateReviewInput) (*domain.Review, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if input.Rating < 1 || input.Rating > 5 {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "rating must be between 1 and 5", nil)
	}

	if strings.TrimSpace(input.ReviewText) == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "review text is required", nil)
	}

	rev, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		return nil, err
	}

	// Authorize: Only the review author can edit their own review
	if err := s.authzService.AuthorizeReviewModification(actor, rev.CustomerUserID); err != nil {
		return nil, err
	}

	if err := s.reviewRepo.Update(ctx, reviewID, input.Rating, strings.TrimSpace(input.ReviewText)); err != nil {
		return nil, err
	}

	rev.Rating = input.Rating
	rev.ReviewText = strings.TrimSpace(input.ReviewText)
	return rev, nil
}

func (s *reviewService) DeleteReview(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error {
	rev, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		return err
	}

	// Authorize: Review author (or platform admin via moderation)
	if err := s.authzService.AuthorizeReviewModification(actor, rev.CustomerUserID); err != nil {
		return err
	}

	return s.reviewRepo.SoftDelete(ctx, reviewID)
}

func (s *reviewService) ListBusinessReviews(ctx context.Context, businessID domain.BusinessID, limit, offset int) ([]domain.Review, error) {
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	return s.reviewRepo.ListForBusiness(ctx, businessID, limit, offset)
}

func (s *reviewService) ListMyReviews(ctx context.Context, actor *domain.User) ([]domain.Review, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	return s.reviewRepo.ListForCustomer(ctx, actor.ID)
}

func (s *reviewService) GetReviewSummary(ctx context.Context, businessID domain.BusinessID) (*domain.ReviewSummary, error) {
	return s.reviewRepo.GetSummary(ctx, businessID)
}

func (s *reviewService) MarkHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	// Verify review exists
	_, err := s.reviewRepo.GetByID(ctx, reviewID)
	if err != nil {
		return err
	}

	vote := &domain.ReviewHelpfulVote{
		ReviewID:       reviewID,
		CustomerUserID: actor.ID,
	}

	return s.reviewRepo.AddHelpfulVote(ctx, vote)
}

func (s *reviewService) UnmarkHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) error {
	if actor == nil {
		return domain.ErrUnauthenticated
	}

	return s.reviewRepo.RemoveHelpfulVote(ctx, reviewID, actor.ID)
}

func (s *reviewService) HasVotedHelpful(ctx context.Context, actor *domain.User, reviewID domain.ReviewID) (bool, error) {
	if actor == nil {
		return false, domain.ErrUnauthenticated
	}

	return s.reviewRepo.HasVotedHelpful(ctx, reviewID, actor.ID)
}
