package service

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// AuthorizationService is the authoritative security boundary evaluating operation permissions.
type AuthorizationService interface {
	// Business management ReBAC predicates
	CanManageBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID) (bool, error)
	AuthorizeBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID, perm domain.Permission) error
	CanViewBusinessManagement(ctx context.Context, user *domain.User, businessID domain.BusinessID) error
	CanEditBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID) error
	CanManageMenu(ctx context.Context, user *domain.User, businessID domain.BusinessID) error
	CanPublishUpdate(ctx context.Context, user *domain.User, businessID domain.BusinessID) error
	CanManageReservations(ctx context.Context, user *domain.User, businessID domain.BusinessID) error

	// Platform admin RBAC predicates
	AuthorizeAdmin(ctx context.Context, user *domain.User, perm domain.Permission) error
	CanReviewBusinessSubmissions(user *domain.User) bool
	CanReviewClaims(user *domain.User) bool
	CanModerateReviews(user *domain.User) bool
	CanResolveBusinessReports(user *domain.User) bool

	CanManageTaxonomy(user *domain.User) error
	AuthorizeMenuItemMapping(ctx context.Context, user *domain.User, businessID domain.BusinessID) error

	// Customer resource ownership predicates
	AuthorizeUserResource(user *domain.User, targetUserID domain.UserID) error
	AuthorizeReviewModification(user *domain.User, reviewAuthorID domain.UserID) error
	AuthorizeReservationCancellation(user *domain.User, reservationCustomerUserID domain.UserID, status domain.ReservationStatus) error
}

type authorizationService struct {
	ownershipRepo repository.OwnershipRepository
	businessRepo  repository.BusinessRepository
}

// NewAuthorizationService constructs a resource-aware AuthorizationService.
func NewAuthorizationService(
	ownershipRepo repository.OwnershipRepository,
	businessRepo repository.BusinessRepository,
) AuthorizationService {
	return &authorizationService{
		ownershipRepo: ownershipRepo,
		businessRepo:  businessRepo,
	}
}

// CanManageBusiness evaluates the formal predicate:
// can_manage_business(U, B) <=> (U.role == 'ADMIN') || exists M in BusinessManagers(user_id=U.id, business_id=B.id)
func (s *authorizationService) CanManageBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID) (bool, error) {
	if user == nil {
		return false, domain.ErrUnauthenticated
	}

	// 1. Platform Admin override: governance staff have platform oversight
	if user.IsAdmin() {
		return true, nil
	}

	// 2. Relationship check: must possess an approved active management record
	isManager, err := s.ownershipRepo.IsManager(ctx, businessID, user.ID)
	if err != nil {
		return false, err
	}

	return isManager, nil
}

func (s *authorizationService) AuthorizeBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID, perm domain.Permission) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}

	canManage, err := s.CanManageBusiness(ctx, user, businessID)
	if err != nil {
		return err
	}
	if !canManage {
		return domain.ErrNotBusinessManager
	}

	// Verify target business exists and is not soft-deleted
	_, err = s.businessRepo.GetByID(ctx, businessID)
	if err != nil {
		return err
	}

	return nil
}

func (s *authorizationService) CanViewBusinessManagement(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	return s.AuthorizeBusiness(ctx, user, businessID, domain.PermissionBusinessViewManagement)
}

func (s *authorizationService) CanEditBusiness(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	return s.AuthorizeBusiness(ctx, user, businessID, domain.PermissionBusinessEdit)
}

func (s *authorizationService) CanManageMenu(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	return s.AuthorizeBusiness(ctx, user, businessID, domain.PermissionBusinessManageMenu)
}

func (s *authorizationService) CanPublishUpdate(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	return s.AuthorizeBusiness(ctx, user, businessID, domain.PermissionBusinessManageUpdates)
}

func (s *authorizationService) CanManageReservations(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	return s.AuthorizeBusiness(ctx, user, businessID, domain.PermissionBusinessManageReservations)
}

func (s *authorizationService) AuthorizeAdmin(ctx context.Context, user *domain.User, perm domain.Permission) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}

	if !user.IsAdmin() {
		return domain.ErrForbidden
	}

	return nil
}

func (s *authorizationService) CanReviewBusinessSubmissions(user *domain.User) bool {
	return user != nil && user.IsAdmin()
}

func (s *authorizationService) CanReviewClaims(user *domain.User) bool {
	return user != nil && user.IsAdmin()
}

func (s *authorizationService) CanModerateReviews(user *domain.User) bool {
	return user != nil && user.IsAdmin()
}

func (s *authorizationService) CanResolveBusinessReports(user *domain.User) bool {
	return user != nil && user.IsAdmin()
}

func (s *authorizationService) AuthorizeUserResource(user *domain.User, targetUserID domain.UserID) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}

	if user.ID != targetUserID {
		return domain.ErrForbidden
	}

	return nil
}

func (s *authorizationService) AuthorizeReviewModification(user *domain.User, reviewAuthorID domain.UserID) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}

	// Only author can edit or delete their own review (admins remove reviews via moderation action)
	if user.ID != reviewAuthorID {
		return domain.ErrForbidden
	}

	return nil
}

func (s *authorizationService) AuthorizeReservationCancellation(
	user *domain.User,
	reservationCustomerUserID domain.UserID,
	status domain.ReservationStatus,
) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}

	if user.ID != reservationCustomerUserID {
		return domain.ErrForbidden
	}

	// Must be in a legal cancellable state (PENDING or CONFIRMED)
	if status != domain.ReservationStatusPending && status != domain.ReservationStatusConfirmed {
		return domain.ErrIllegalStateTransition
	}

	return nil
}

func (s *authorizationService) CanManageTaxonomy(user *domain.User) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}
	if !user.IsAdmin() {
		return domain.ErrForbidden
	}
	return nil
}

func (s *authorizationService) AuthorizeMenuItemMapping(ctx context.Context, user *domain.User, businessID domain.BusinessID) error {
	if user == nil {
		return domain.ErrUnauthenticated
	}
	if user.IsAdmin() {
		return nil
	}
	return s.CanManageMenu(ctx, user, businessID)
}
