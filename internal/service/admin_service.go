package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// AdminService orchestrates administrative governance, submissions, claims, moderation, and auditing.
type AdminService interface {
	// Submissions
	ListPendingSubmissions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessSubmission, error)
	ApproveSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.Business, error)
	RejectSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID, reason string) error

	// Claims
	ListPendingClaims(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessClaim, error)
	ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error
	RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error

	// Reports & Moderation
	ListPendingReviewReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.ReviewReport, error)
	ResolveReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, action *string) error
	ListPendingBusinessReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessReport, error)
	ResolveBusinessReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, notes *string) error
	ModerateReview(ctx context.Context, adminUser *domain.User, reviewID domain.ReviewID, status domain.ReviewStatus, reason string) error

	// Business Badging & Publication
	SetBusinessPublicationStatus(ctx context.Context, adminUser *domain.User, businessID domain.BusinessID, status domain.PublicationStatus) error
	SetBusinessVerificationStatus(ctx context.Context, adminUser *domain.User, businessID domain.BusinessID, status domain.VerificationStatus) error

	// Audit Trail
	ListAdminActions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.AdminAction, error)
}

type adminService struct {
	ownershipWorkflow OwnershipWorkflowService
	authzService      AuthorizationService
	submissionRepo    repository.SubmissionRepository
	claimRepo         repository.ClaimRepository
	businessRepo      repository.BusinessRepository
	reviewRepo        repository.ReviewRepository
	reportRepo        repository.ReportRepository
	adminActionRepo   repository.AdminActionRepository
	txMgr             repository.TxManager
}

// NewAdminService constructs an AdminService coordinating governance workflows.
func NewAdminService(
	ownershipWorkflow OwnershipWorkflowService,
	authzService AuthorizationService,
	submissionRepo repository.SubmissionRepository,
	claimRepo repository.ClaimRepository,
	businessRepo repository.BusinessRepository,
	reviewRepo repository.ReviewRepository,
	reportRepo repository.ReportRepository,
	adminActionRepo repository.AdminActionRepository,
	txMgr repository.TxManager,
) AdminService {
	return &adminService{
		ownershipWorkflow: ownershipWorkflow,
		authzService:      authzService,
		submissionRepo:    submissionRepo,
		claimRepo:         claimRepo,
		businessRepo:      businessRepo,
		reviewRepo:        reviewRepo,
		reportRepo:        reportRepo,
		adminActionRepo:   adminActionRepo,
		txMgr:             txMgr,
	}
}

func (s *adminService) ListPendingSubmissions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessSubmission, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewSubmissions); err != nil {
		return nil, err
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

	return s.submissionRepo.ListPending(ctx, limit, offset)
}

func (s *adminService) ApproveSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.Business, error) {
	return s.ownershipWorkflow.ApproveSubmission(ctx, adminUser, submissionID)
}

func (s *adminService) RejectSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID, reason string) error {
	return s.ownershipWorkflow.RejectSubmission(ctx, adminUser, submissionID, reason)
}

func (s *adminService) ListPendingClaims(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessClaim, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewClaims); err != nil {
		return nil, err
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

	return s.claimRepo.ListPending(ctx, limit, offset)
}

func (s *adminService) ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error {
	return s.ownershipWorkflow.ApproveClaim(ctx, adminUser, claimID, notes)
}

func (s *adminService) RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error {
	return s.ownershipWorkflow.RejectClaim(ctx, adminUser, claimID, reason)
}

func (s *adminService) ListPendingReviewReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.ReviewReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return nil, err
	}

	return s.reportRepo.ListPendingReviewReports(ctx, limit, offset)
}

func (s *adminService) ResolveReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, action *string) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.reportRepo.ResolveReviewReport(txCtx, reportID, status, &adminUser.ID, action); err != nil {
			return err
		}

		actionType := domain.AdminActionReviewRemoved
		if status == domain.ReportStatusDismissed {
			actionType = domain.AdminActionReviewReportDismissed
		}

		audit := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       actionType,
			TargetEntityType: "review_report",
			TargetEntityID:   string(reportID),
			Metadata: map[string]any{
				"resolution_status": status,
				"action":            action,
			},
		}
		return s.adminActionRepo.Record(txCtx, audit)
	})
}

func (s *adminService) ListPendingBusinessReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReports); err != nil {
		return nil, err
	}

	return s.reportRepo.ListPendingBusinessReports(ctx, limit, offset)
}

func (s *adminService) ResolveBusinessReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, notes *string) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReports); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.reportRepo.ResolveBusinessReport(txCtx, reportID, status, &adminUser.ID, notes); err != nil {
			return err
		}

		actionType := domain.AdminActionBusinessReportActioned
		if status == domain.ReportStatusDismissed {
			actionType = domain.AdminActionBusinessReportDismissed
		}

		audit := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       actionType,
			TargetEntityType: "business_report",
			TargetEntityID:   string(reportID),
			Metadata: map[string]any{
				"resolution_status": status,
				"notes":             notes,
			},
		}
		return s.adminActionRepo.Record(txCtx, audit)
	})
}

func (s *adminService) ModerateReview(ctx context.Context, adminUser *domain.User, reviewID domain.ReviewID, status domain.ReviewStatus, reason string) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return err
	}

	if strings.TrimSpace(reason) == "" {
		return domain.NewDomainError(domain.ErrInvalidInput, "moderation reason is required", nil)
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.reviewRepo.SetModerationStatus(txCtx, reviewID, status); err != nil {
			return err
		}

		audit := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionReviewRemoved,
			TargetEntityType: "review",
			TargetEntityID:   string(reviewID),
			Metadata: map[string]any{
				"moderation_status": status,
				"reason":            reason,
			},
		}
		return s.adminActionRepo.Record(txCtx, audit)
	})
}

func (s *adminService) SetBusinessPublicationStatus(ctx context.Context, adminUser *domain.User, businessID domain.BusinessID, status domain.PublicationStatus) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminManageBadges); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.businessRepo.SetPublicationStatus(txCtx, businessID, status); err != nil {
			return err
		}

		audit := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionPublicationStatusChanged,
			TargetEntityType: "business",
			TargetEntityID:   fmt.Sprintf("%d", businessID),
			Metadata: map[string]any{
				"publication_status": status,
			},
		}
		return s.adminActionRepo.Record(txCtx, audit)
	})
}

func (s *adminService) SetBusinessVerificationStatus(ctx context.Context, adminUser *domain.User, businessID domain.BusinessID, status domain.VerificationStatus) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminManageBadges); err != nil {
		return err
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.businessRepo.SetVerificationStatus(txCtx, businessID, status); err != nil {
			return err
		}

		audit := &domain.AdminAction{
			AdminUserID:      adminUser.ID,
			ActionType:       domain.AdminActionVerificationStatusChanged,
			TargetEntityType: "business",
			TargetEntityID:   fmt.Sprintf("%d", businessID),
			Metadata: map[string]any{
				"verification_status": status,
			},
		}
		return s.adminActionRepo.Record(txCtx, audit)
	})
}

func (s *adminService) ListAdminActions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.AdminAction, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminManageBadges); err != nil {
		return nil, err
	}

	return s.adminActionRepo.List(ctx, limit, offset)
}
