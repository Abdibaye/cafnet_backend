package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
	"github.com/Abdibaye/cafnet_backend/internal/storage"
)

// OwnershipWorkflowService coordinates atomic claim and submission approval lifecycles.
type OwnershipWorkflowService interface {
	ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error
	RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error
	ApproveSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.Business, error)
	RejectSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID, reason string) error
}

type ownershipWorkflowService struct {
	authzService    AuthorizationService
	claimRepo       repository.ClaimRepository
	submissionRepo  repository.SubmissionRepository
	businessRepo    repository.BusinessRepository
	ownershipRepo   repository.OwnershipRepository
	adminActionRepo repository.AdminActionRepository
	mediaRepo       repository.MediaRepository
	storage         storage.ObjectStorage
	txMgr           repository.TxManager
}

// NewOwnershipWorkflowService creates a workflow service bound by transaction management.
func NewOwnershipWorkflowService(
	authzService AuthorizationService,
	claimRepo repository.ClaimRepository,
	submissionRepo repository.SubmissionRepository,
	businessRepo repository.BusinessRepository,
	ownershipRepo repository.OwnershipRepository,
	adminActionRepo repository.AdminActionRepository,
	mediaRepo repository.MediaRepository,
	objStorage storage.ObjectStorage,
	txMgr repository.TxManager,
) OwnershipWorkflowService {
	return &ownershipWorkflowService{
		authzService:    authzService,
		claimRepo:       claimRepo,
		submissionRepo:  submissionRepo,
		businessRepo:    businessRepo,
		ownershipRepo:   ownershipRepo,
		adminActionRepo: adminActionRepo,
		mediaRepo:       mediaRepo,
		storage:         objStorage,
		txMgr:           txMgr,
	}
}

func (s *ownershipWorkflowService) ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error {
	// 1. Authorize: Only platform administrators can review claims
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewClaims); err != nil {
		return err
	}

	claim, err := s.claimRepo.GetByID(ctx, claimID)
	if err != nil {
		return err
	}

	// 2. Prevent administrative self-escalation: claimant cannot approve their own claim
	if claim.ClaimantUserID == adminUser.ID {
		return domain.ErrSelfApprovalForbidden
	}

	if claim.Status != domain.ClaimStatusPending {
		return domain.ErrIllegalStateTransition
	}

	// 3. Atomically approve claim, grant manager role, update business status, and audit log
	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Recheck claim status within transaction
		freshClaim, err := s.claimRepo.GetByID(txCtx, claimID)
		if err != nil {
			return err
		}
		if freshClaim.Status != domain.ClaimStatusPending {
			return domain.ErrIllegalStateTransition
		}

		// Row-level lock target business to safely prevent race conditions
		business, err := s.businessRepo.GetByIDForUpdate(txCtx, freshClaim.BusinessID)
		if err != nil {
			return err
		}
		if business.ClaimStatus != domain.ClaimStatusUnclaimed {
			return domain.NewDomainError(domain.ErrConflict, "business is already claimed and managed", nil)
		}

		// Recheck active managers to enforce single-owner invariant
		existingManagers, err := s.ownershipRepo.GetManagers(txCtx, freshClaim.BusinessID)
		if err != nil {
			return err
		}
		if len(existingManagers) > 0 {
			return domain.NewDomainError(domain.ErrConflict, "business already has active managers", nil)
		}

		// Update claim status
		if err := s.claimRepo.UpdateStatus(txCtx, claimID, domain.ClaimStatusApproved, &adminUser.ID, notes); err != nil {
			return err
		}

		// Grant business manager relationship
		mgr := &domain.BusinessManager{
			BusinessID:  freshClaim.BusinessID,
			UserID:      freshClaim.ClaimantUserID,
			ManagerRole: domain.ManagerRolePrimaryOwner,
			GrantedBy:   &adminUser.ID,
		}
		if err := s.ownershipRepo.AddManager(txCtx, mgr); err != nil {
			return err
		}

		// Update business claim_status to CLAIMED
		if err := s.businessRepo.SetClaimStatus(txCtx, freshClaim.BusinessID, domain.ClaimStatusClaimed); err != nil {
			return err
		}

		// Record immutable administrative audit trail
		action := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionClaimApproved,
			TargetEntityType: "business_claim",
			TargetEntityID:   string(claimID),
			Metadata: map[string]any{
				"business_id":      freshClaim.BusinessID,
				"claimant_user_id": freshClaim.ClaimantUserID,
				"notes":            notes,
			},
		}
		return s.adminActionRepo.Record(txCtx, action)
	})
}

func (s *ownershipWorkflowService) RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewClaims); err != nil {
		return err
	}

	if strings.TrimSpace(reason) == "" {
		return domain.ErrInvalidInput
	}

	claim, err := s.claimRepo.GetByID(ctx, claimID)
	if err != nil {
		return err
	}

	if claim.Status != domain.ClaimStatusPending {
		return domain.ErrIllegalStateTransition
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.claimRepo.UpdateStatus(txCtx, claimID, domain.ClaimStatusRejected, &adminUser.ID, &reason); err != nil {
			return err
		}

		action := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionClaimRejected,
			TargetEntityType: "business_claim",
			TargetEntityID:   string(claimID),
			Metadata: map[string]any{
				"business_id":      claim.BusinessID,
				"claimant_user_id": claim.ClaimantUserID,
				"reason":           reason,
			},
		}
		return s.adminActionRepo.Record(txCtx, action)
	})
}

func (s *ownershipWorkflowService) ApproveSubmission(
	ctx context.Context,
	adminUser *domain.User,
	submissionID domain.SubmissionID,
) (*domain.Business, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewSubmissions); err != nil {
		return nil, err
	}

	sub, err := s.submissionRepo.GetByID(ctx, submissionID)
	if err != nil {
		return nil, err
	}

	// Prevent self-approval of business submissions
	if sub.SubmitterUserID == adminUser.ID {
		return nil, domain.ErrSelfApprovalForbidden
	}

	if sub.Status != domain.SubmissionStatusPending {
		return nil, domain.ErrIllegalStateTransition
	}

	var createdBusiness *domain.Business

	err = s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		// Generate slug from business name + timestamp
		cleanName := strings.ToLower(strings.ReplaceAll(sub.BusinessName, " ", "-"))
		slug := fmt.Sprintf("%s-%d", cleanName, time.Now().UnixNano()%1000000)

		cuisine, _ := sub.SubmittedPayload["cuisine"].(string)
		if cuisine == "" {
			cuisine = "Ethiopian"
		}
		phone, _ := sub.SubmittedPayload["phone"].(string)
		if phone == "" {
			phone = "+251900000000"
		}
		desc, _ := sub.SubmittedPayload["description"].(string)
		if desc == "" {
			desc = sub.BusinessName + " in " + sub.Area
		}

		b := &domain.Business{
			Name:                 sub.BusinessName,
			Slug:                 slug,
			Category:             sub.Category,
			Cuisine:              cuisine,
			Description:          desc,
			ShortDescription:     desc,
			LocationName:         sub.Area,
			Neighborhood:         sub.Area,
			Phone:                phone,
			PriceRange:           "$$",
			Tags:                 []string{},
			PaymentMethods:       []string{"Cash"},
			ReservationAvailable: true,
			PublicationStatus:    domain.PublicationStatusPublished,
			VerificationStatus:   domain.VerificationStatusVerified,
			ClaimStatus:          domain.ClaimStatusClaimed,
			CreatedByUserID:      &sub.SubmitterUserID,
		}

		if err := s.businessRepo.Create(txCtx, b); err != nil {
			return err
		}

		// Assign submitter as primary manager
		mgr := &domain.BusinessManager{
			BusinessID:  b.ID,
			UserID:      sub.SubmitterUserID,
			ManagerRole: domain.ManagerRolePrimaryOwner,
			GrantedBy:   &adminUser.ID,
		}
		if err := s.ownershipRepo.AddManager(txCtx, mgr); err != nil {
			return err
		}

		// Update submission status to APPROVED
		if err := s.submissionRepo.UpdateStatus(txCtx, submissionID, domain.SubmissionStatusApproved, &b.ID, &adminUser.ID, nil); err != nil {
			return err
		}

		// Transition submission cover image to canonical business cover
		var coverMedia *domain.Media
		if sub.CoverMediaID != nil && s.mediaRepo != nil {
			m, err := s.mediaRepo.GetByID(txCtx, *sub.CoverMediaID)
			if err == nil && m != nil {
				coverMedia = m
			}
		}
		if coverMedia == nil && s.mediaRepo != nil {
			m, err := s.mediaRepo.GetBySubmissionID(txCtx, submissionID)
			if err == nil && m != nil {
				coverMedia = m
			}
		}

		if coverMedia != nil && s.mediaRepo != nil {
			_ = s.mediaRepo.AssociateWithBusiness(txCtx, coverMedia.ID, b.ID, domain.MediaTypeBusinessCover, 0)
			if s.storage != nil {
				pubURL := s.storage.PublicURL(coverMedia.ObjectKey)
				b.CoverImage = &pubURL
				_ = s.businessRepo.SetImages(txCtx, b.ID, []domain.BusinessImage{
					{
						BusinessID: b.ID,
						ImageURL:   pubURL,
						AltText:    &b.Name,
						SortOrder:  0,
					},
				})
			}
		}

		// Record audit log
		action := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionBusinessApproved,
			TargetEntityType: "business_submission",
			TargetEntityID:   string(submissionID),
			Metadata: map[string]any{
				"business_id":       b.ID,
				"submitter_user_id": sub.SubmitterUserID,
			},
		}
		if err := s.adminActionRepo.Record(txCtx, action); err != nil {
			return err
		}

		createdBusiness = b
		return nil
	})

	if err != nil {
		return nil, err
	}

	return createdBusiness, nil
}

func (s *ownershipWorkflowService) RejectSubmission(
	ctx context.Context,
	adminUser *domain.User,
	submissionID domain.SubmissionID,
	reason string,
) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewSubmissions); err != nil {
		return err
	}

	if strings.TrimSpace(reason) == "" {
		return domain.ErrInvalidInput
	}

	sub, err := s.submissionRepo.GetByID(ctx, submissionID)
	if err != nil {
		return err
	}

	if sub.Status != domain.SubmissionStatusPending {
		return domain.ErrIllegalStateTransition
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.submissionRepo.UpdateStatus(txCtx, submissionID, domain.SubmissionStatusRejected, nil, &adminUser.ID, &reason); err != nil {
			return err
		}

		action := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionBusinessRejected,
			TargetEntityType: "business_submission",
			TargetEntityID:   string(submissionID),
			Metadata: map[string]any{
				"submitter_user_id": sub.SubmitterUserID,
				"reason":            reason,
			},
		}
		return s.adminActionRepo.Record(txCtx, action)
	})
}
