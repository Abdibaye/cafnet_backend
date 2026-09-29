package repository

import (
	"context"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	GetByID(ctx context.Context, id domain.UserID) (*domain.User, error)
	GetByPhone(ctx context.Context, phone string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
}

type BusinessRepository interface {
	Create(ctx context.Context, business *domain.Business) error
	GetByID(ctx context.Context, id domain.BusinessID) (*domain.Business, error)
	GetBySlug(ctx context.Context, slug string) (*domain.Business, error)
	Update(ctx context.Context, business *domain.Business) error
	SetPublicationStatus(ctx context.Context, id domain.BusinessID, status domain.PublicationStatus) error
	SetVerificationStatus(ctx context.Context, id domain.BusinessID, status domain.VerificationStatus) error
	SetClaimStatus(ctx context.Context, id domain.BusinessID, status domain.ClaimStatus) error
	List(ctx context.Context, filter domain.BusinessFilter) ([]domain.Business, error)
	Search(ctx context.Context, query string, limit, offset int) ([]domain.SearchResult, error)
	GetHours(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessHours, error)
	SetHours(ctx context.Context, businessID domain.BusinessID, hours []domain.BusinessHours) error
	GetImages(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessImage, error)
	SetImages(ctx context.Context, businessID domain.BusinessID, images []domain.BusinessImage) error
	SoftDelete(ctx context.Context, id domain.BusinessID) error
}

type OwnershipRepository interface {
	AddManager(ctx context.Context, manager *domain.BusinessManager) error
	RemoveManager(ctx context.Context, businessID domain.BusinessID, userID domain.UserID) error
	IsManager(ctx context.Context, businessID domain.BusinessID, userID domain.UserID) (bool, error)
	GetManagers(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessManager, error)
	GetManagedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error)
}

type ClaimRepository interface {
	Create(ctx context.Context, claim *domain.BusinessClaim) error
	GetByID(ctx context.Context, id domain.ClaimID) (*domain.BusinessClaim, error)
	GetPendingForBusiness(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessClaim, error)
	ListPending(ctx context.Context, limit, offset int) ([]domain.BusinessClaim, error)
	ListByClaimant(ctx context.Context, claimantID domain.UserID, limit, offset int) ([]domain.BusinessClaim, error)
	UpdateStatus(ctx context.Context, id domain.ClaimID, status domain.ClaimWorkflowStatus, reviewedBy *domain.UserID, reason *string) error
}

type SubmissionRepository interface {
	Create(ctx context.Context, sub *domain.BusinessSubmission) error
	GetByID(ctx context.Context, id domain.SubmissionID) (*domain.BusinessSubmission, error)
	ListPending(ctx context.Context, limit, offset int) ([]domain.BusinessSubmission, error)
	ListBySubmitter(ctx context.Context, submitterID domain.UserID, limit, offset int) ([]domain.BusinessSubmission, error)
	UpdateStatus(ctx context.Context, id domain.SubmissionID, status domain.SubmissionWorkflowStatus, createdBusinessID *domain.BusinessID, reviewedBy *domain.UserID, reason *string) error
}

type MenuRepository interface {
	CreateCategory(ctx context.Context, cat *domain.MenuCategory) error
	GetCategoryByID(ctx context.Context, id domain.CategoryID) (*domain.MenuCategory, error)
	UpdateCategory(ctx context.Context, cat *domain.MenuCategory) error
	DeleteCategory(ctx context.Context, id domain.CategoryID) error
	ReorderCategories(ctx context.Context, businessID domain.BusinessID, categoryIDs []domain.CategoryID) error
	ListCategories(ctx context.Context, businessID domain.BusinessID) ([]domain.MenuCategory, error)

	CreateItem(ctx context.Context, item *domain.MenuItem) error
	GetItemByID(ctx context.Context, id domain.ItemID) (*domain.MenuItem, error)
	UpdateItem(ctx context.Context, item *domain.MenuItem) error
	DeleteItem(ctx context.Context, id domain.ItemID) error
	ReorderItems(ctx context.Context, categoryID domain.CategoryID, itemIDs []domain.ItemID) error
	ListItemsByCategory(ctx context.Context, categoryID domain.CategoryID) ([]domain.MenuItem, error)

	GetFullMenu(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMenu, error)
}

type UpdateRepository interface {
	Create(ctx context.Context, update *domain.BusinessUpdate) error
	GetByID(ctx context.Context, id domain.UpdateID) (*domain.BusinessUpdate, error)
	ListActive(ctx context.Context, businessID *domain.BusinessID, limit, offset int) ([]domain.BusinessUpdate, error)
	ListActiveByBusinessIDs(ctx context.Context, businessIDs []domain.BusinessID, limit, offset int) ([]domain.BusinessUpdate, error)
	ListByBusiness(ctx context.Context, businessID domain.BusinessID, includeExpired bool) ([]domain.BusinessUpdate, error)
	Update(ctx context.Context, update *domain.BusinessUpdate) error
	Delete(ctx context.Context, id domain.UpdateID) error
}

type ReservationRepository interface {
	Create(ctx context.Context, res *domain.Reservation) error
	GetByID(ctx context.Context, id domain.ReservationID) (*domain.Reservation, error)
	GetByCode(ctx context.Context, code string) (*domain.Reservation, error)
	List(ctx context.Context, filter domain.ReservationFilter) ([]domain.Reservation, error)
	Confirm(ctx context.Context, id domain.ReservationID, reviewedBy *domain.UserID) error
	Decline(ctx context.Context, id domain.ReservationID, reason string, reviewedBy *domain.UserID) error
	Cancel(ctx context.Context, id domain.ReservationID) error
	Complete(ctx context.Context, id domain.ReservationID, reviewedBy *domain.UserID) error
}

type ReviewRepository interface {
	Create(ctx context.Context, rev *domain.Review) error
	GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error)
	GetByReservationID(ctx context.Context, resID domain.ReservationID) (*domain.Review, error)
	ListForBusiness(ctx context.Context, businessID domain.BusinessID, limit, offset int) ([]domain.Review, error)
	ListForCustomer(ctx context.Context, customerID domain.UserID) ([]domain.Review, error)
	Update(ctx context.Context, id domain.ReviewID, rating int16, text string) error
	SoftDelete(ctx context.Context, id domain.ReviewID) error
	SetModerationStatus(ctx context.Context, id domain.ReviewID, status domain.ReviewStatus) error
	GetSummary(ctx context.Context, businessID domain.BusinessID) (*domain.ReviewSummary, error)

	AddHelpfulVote(ctx context.Context, vote *domain.ReviewHelpfulVote) error
	RemoveHelpfulVote(ctx context.Context, reviewID domain.ReviewID, userID domain.UserID) error
	HasVotedHelpful(ctx context.Context, reviewID domain.ReviewID, userID domain.UserID) (bool, error)
}

type ReportRepository interface {
	CreateReviewReport(ctx context.Context, rep *domain.ReviewReport) error
	GetReviewReportByID(ctx context.Context, id domain.ReportID) (*domain.ReviewReport, error)
	ListPendingReviewReports(ctx context.Context, limit, offset int) ([]domain.ReviewReport, error)
	ResolveReviewReport(ctx context.Context, id domain.ReportID, status domain.ReportStatus, reviewedBy *domain.UserID, action *string) error

	CreateBusinessReport(ctx context.Context, rep *domain.BusinessReport) error
	GetBusinessReportByID(ctx context.Context, id domain.ReportID) (*domain.BusinessReport, error)
	ListPendingBusinessReports(ctx context.Context, limit, offset int) ([]domain.BusinessReport, error)
	ResolveBusinessReport(ctx context.Context, id domain.ReportID, status domain.ReportStatus, reviewedBy *domain.UserID, notes *string) error
}

type CustomerListRepository interface {
	SaveBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error
	UnsaveBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error
	IsSaved(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) (bool, error)
	ListSavedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error)

	FollowBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error
	UnfollowBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error
	IsFollowing(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) (bool, error)
	ListFollowedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error)
}

type AdminActionRepository interface {
	Record(ctx context.Context, action *domain.AdminAction) error
	List(ctx context.Context, limit, offset int) ([]domain.AdminAction, error)
}

type SessionRepository interface {
	Create(ctx context.Context, session *domain.Session) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error)
	UpdateLastUsed(ctx context.Context, id string) error
	Revoke(ctx context.Context, id string) error
	RevokeAllForUser(ctx context.Context, userID domain.UserID) error
	DeleteExpired(ctx context.Context) (int64, error)
}

type OTPRepository interface {
	CreateChallenge(ctx context.Context, challenge *domain.OTPChallenge) error
	GetByID(ctx context.Context, id string) (*domain.OTPChallenge, error)
	GetForUpdate(ctx context.Context, id string) (*domain.OTPChallenge, error)
	IncrementAttempts(ctx context.Context, id string) error
	ConsumeChallenge(ctx context.Context, id string) error
	InvalidatePrevious(ctx context.Context, phone string) error
	GetRecentRequest(ctx context.Context, phone string, cooldown time.Duration) (*domain.OTPChallenge, error)
}
