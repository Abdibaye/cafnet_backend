package service

import (
	"context"
	"strings"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/internal/repository"
)

// CreateReviewReportInput defines the payload for reporting an inappropriate or misleading review.
type CreateReviewReportInput struct {
	ReviewID    domain.ReviewID           `json:"review_id"`
	Reason      domain.ReviewReportReason `json:"reason"`
	Explanation string                    `json:"explanation"`
}

// CreateBusinessReportInput defines the payload for reporting inaccurate business data or policy violations.
type CreateBusinessReportInput struct {
	BusinessID domain.BusinessID           `json:"business_id"`
	Reason     domain.BusinessReportReason `json:"reason"`
	Details    *string                     `json:"details"`
}

// ReportService coordinates customer submission of review and venue reports.
type ReportService interface {
	CreateReviewReport(ctx context.Context, actor *domain.User, input CreateReviewReportInput) (*domain.ReviewReport, error)
	CreateBusinessReport(ctx context.Context, actor *domain.User, input CreateBusinessReportInput) (*domain.BusinessReport, error)
	GetReviewReport(ctx context.Context, actor *domain.User, id domain.ReportID) (*domain.ReviewReport, error)
	GetBusinessReport(ctx context.Context, actor *domain.User, id domain.ReportID) (*domain.BusinessReport, error)
}

type reportService struct {
	reportRepo   repository.ReportRepository
	reviewRepo   repository.ReviewRepository
	businessRepo repository.BusinessRepository
}

// NewReportService constructs a ReportService.
func NewReportService(
	reportRepo repository.ReportRepository,
	reviewRepo repository.ReviewRepository,
	businessRepo repository.BusinessRepository,
) ReportService {
	return &reportService{
		reportRepo:   reportRepo,
		reviewRepo:   reviewRepo,
		businessRepo: businessRepo,
	}
}

func (s *reportService) CreateReviewReport(ctx context.Context, actor *domain.User, input CreateReviewReportInput) (*domain.ReviewReport, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if strings.TrimSpace(string(input.Reason)) == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "report reason is required", nil)
	}

	// Verify target review exists
	_, err := s.reviewRepo.GetByID(ctx, input.ReviewID)
	if err != nil {
		return nil, err
	}

	report := &domain.ReviewReport{
		ReviewID:       input.ReviewID,
		ReporterUserID: actor.ID,
		Reason:         input.Reason,
		Explanation:    input.Explanation,
		Status:         domain.ReportStatusPending,
	}

	if err := s.reportRepo.CreateReviewReport(ctx, report); err != nil {
		return nil, err
	}

	return report, nil
}

func (s *reportService) CreateBusinessReport(ctx context.Context, actor *domain.User, input CreateBusinessReportInput) (*domain.BusinessReport, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	if strings.TrimSpace(string(input.Reason)) == "" {
		return nil, domain.NewDomainError(domain.ErrInvalidInput, "report reason is required", nil)
	}

	// Verify target business exists
	_, err := s.businessRepo.GetByID(ctx, input.BusinessID)
	if err != nil {
		return nil, err
	}

	report := &domain.BusinessReport{
		BusinessID:     input.BusinessID,
		ReporterUserID: actor.ID,
		Reason:         input.Reason,
		Details:        input.Details,
		Status:         domain.ReportStatusPending,
	}

	if err := s.reportRepo.CreateBusinessReport(ctx, report); err != nil {
		return nil, err
	}

	return report, nil
}

func (s *reportService) GetReviewReport(ctx context.Context, actor *domain.User, id domain.ReportID) (*domain.ReviewReport, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	rep, err := s.reportRepo.GetReviewReportByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if rep.ReporterUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	return rep, nil
}

func (s *reportService) GetBusinessReport(ctx context.Context, actor *domain.User, id domain.ReportID) (*domain.BusinessReport, error) {
	if actor == nil {
		return nil, domain.ErrUnauthenticated
	}

	rep, err := s.reportRepo.GetBusinessReportByID(ctx, id)
	if err != nil {
		return nil, err
	}

	if rep.ReporterUserID != actor.ID && !actor.IsAdmin() {
		return nil, domain.ErrForbidden
	}

	return rep, nil
}
