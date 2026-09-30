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
	// Stats
	GetAdminStats(ctx context.Context, adminUser *domain.User) (*domain.AdminStats, error)

	// Submissions
	ListSubmissions(ctx context.Context, adminUser *domain.User, status *domain.SubmissionWorkflowStatus, limit, offset int) ([]domain.BusinessSubmission, error)
	ListPendingSubmissions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessSubmission, error)
	GetSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.BusinessSubmission, error)
	ApproveSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.Business, error)
	RejectSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID, reason string) error

	// Claims
	ListClaims(ctx context.Context, adminUser *domain.User, status *domain.ClaimWorkflowStatus, limit, offset int) ([]domain.BusinessClaim, error)
	ListPendingClaims(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessClaim, error)
	GetClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID) (*domain.BusinessClaim, error)
	ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error
	RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error

	// Reports & Moderation
	ListReviewReports(ctx context.Context, adminUser *domain.User, status *domain.ReportStatus, limit, offset int) ([]domain.ReviewReport, error)
	ListPendingReviewReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.ReviewReport, error)
	GetReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID) (*domain.ReviewReport, error)
	GetReview(ctx context.Context, adminUser *domain.User, reviewID domain.ReviewID) (*domain.Review, error)
	ResolveReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, action *string) error
	ListBusinessReports(ctx context.Context, adminUser *domain.User, status *domain.ReportStatus, limit, offset int) ([]domain.BusinessReport, error)
	ListPendingBusinessReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessReport, error)
	GetBusinessReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID) (*domain.BusinessReport, error)
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

func (s *adminService) GetAdminStats(ctx context.Context, adminUser *domain.User) (*domain.AdminStats, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewSubmissions); err != nil {
		return nil, err
	}
	return s.adminActionRepo.GetStats(ctx)
}

func (s *adminService) ListSubmissions(ctx context.Context, adminUser *domain.User, status *domain.SubmissionWorkflowStatus, limit, offset int) ([]domain.BusinessSubmission, error) {
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

	return s.submissionRepo.List(ctx, status, limit, offset)
}

func (s *adminService) ListPendingSubmissions(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessSubmission, error) {
	status := domain.SubmissionStatusPending
	return s.ListSubmissions(ctx, adminUser, &status, limit, offset)
}

func (s *adminService) GetSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.BusinessSubmission, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewSubmissions); err != nil {
		return nil, err
	}
	return s.submissionRepo.GetByID(ctx, submissionID)
}

func (s *adminService) ApproveSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID) (*domain.Business, error) {
	return s.ownershipWorkflow.ApproveSubmission(ctx, adminUser, submissionID)
}

func (s *adminService) RejectSubmission(ctx context.Context, adminUser *domain.User, submissionID domain.SubmissionID, reason string) error {
	return s.ownershipWorkflow.RejectSubmission(ctx, adminUser, submissionID, reason)
}

func (s *adminService) ListClaims(ctx context.Context, adminUser *domain.User, status *domain.ClaimWorkflowStatus, limit, offset int) ([]domain.BusinessClaim, error) {
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

	return s.claimRepo.List(ctx, status, limit, offset)
}

func (s *adminService) ListPendingClaims(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessClaim, error) {
	status := domain.ClaimStatusPending
	return s.ListClaims(ctx, adminUser, &status, limit, offset)
}

func (s *adminService) GetClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID) (*domain.BusinessClaim, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminReviewClaims); err != nil {
		return nil, err
	}
	return s.claimRepo.GetByID(ctx, claimID)
}

func (s *adminService) ApproveClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, notes *string) error {
	return s.ownershipWorkflow.ApproveClaim(ctx, adminUser, claimID, notes)
}

func (s *adminService) RejectClaim(ctx context.Context, adminUser *domain.User, claimID domain.ClaimID, reason string) error {
	return s.ownershipWorkflow.RejectClaim(ctx, adminUser, claimID, reason)
}

func (s *adminService) ListReviewReports(ctx context.Context, adminUser *domain.User, status *domain.ReportStatus, limit, offset int) ([]domain.ReviewReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
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

	return s.reportRepo.ListReviewReports(ctx, status, limit, offset)
}

func (s *adminService) ListPendingReviewReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.ReviewReport, error) {
	status := domain.ReportStatusPending
	return s.ListReviewReports(ctx, adminUser, &status, limit, offset)
}

func (s *adminService) GetReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID) (*domain.ReviewReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return nil, err
	}
	return s.reportRepo.GetReviewReportByID(ctx, reportID)
}

func (s *adminService) GetReview(ctx context.Context, adminUser *domain.User, reviewID domain.ReviewID) (*domain.Review, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return nil, err
	}
	return s.reviewRepo.GetByID(ctx, reviewID)
}

func (s *adminService) ResolveReviewReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID, status domain.ReportStatus, action *string) error {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReviews); err != nil {
		return err
	}

	// The review_reports table enforces CHECK (admin_action IS NULL OR admin_action IN ('dismissed', 'removed'))
	var dbAction *string
	if status == domain.ReportStatusDismissed {
		act := "dismissed"
		dbAction = &act
	} else if status == domain.ReportStatusActioned {
		act := "removed"
		if action != nil && (*action == "dismissed" || *action == "removed") {
			act = *action
		}
		dbAction = &act
	}

	return s.txMgr.WithinTransaction(ctx, func(txCtx context.Context) error {
		if err := s.reportRepo.ResolveReviewReport(txCtx, reportID, status, &adminUser.ID, dbAction); err != nil {
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

func (s *adminService) ListBusinessReports(ctx context.Context, adminUser *domain.User, status *domain.ReportStatus, limit, offset int) ([]domain.BusinessReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReports); err != nil {
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

	return s.reportRepo.ListBusinessReports(ctx, status, limit, offset)
}

func (s *adminService) ListPendingBusinessReports(ctx context.Context, adminUser *domain.User, limit, offset int) ([]domain.BusinessReport, error) {
	status := domain.ReportStatusPending
	return s.ListBusinessReports(ctx, adminUser, &status, limit, offset)
}

func (s *adminService) GetBusinessReport(ctx context.Context, adminUser *domain.User, reportID domain.ReportID) (*domain.BusinessReport, error) {
	if err := s.authzService.AuthorizeAdmin(ctx, adminUser, domain.PermissionAdminModerateReports); err != nil {
		return nil, err
	}
	return s.reportRepo.GetBusinessReportByID(ctx, reportID)
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
