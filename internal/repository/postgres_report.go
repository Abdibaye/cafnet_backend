package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresReportRepository struct {
	pool *db.Pool
}

func NewReportRepository(pool *db.Pool) ReportRepository {
	return &postgresReportRepository{pool: pool}
}

func (r *postgresReportRepository) CreateReviewReport(ctx context.Context, rep *domain.ReviewReport) error {
	conn := GetDB(ctx, r.pool.Pool)

	if rep.Status == "" {
		rep.Status = domain.ReportStatusPending
	}

	query := `
		INSERT INTO review_reports (
			id, review_id, reporter_user_id, reason, explanation,
			status, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		rep.ID,
		rep.ReviewID,
		rep.ReporterUserID,
		rep.Reason,
		rep.Explanation,
		rep.Status,
	).Scan(&rep.ID, &rep.CreatedAt, &rep.UpdatedAt)

	return MapPostgresError(err, "ReviewReport")
}

func (r *postgresReportRepository) GetReviewReportByID(ctx context.Context, id domain.ReportID) (*domain.ReviewReport, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, review_id, reporter_user_id, reason, explanation,
			status, reviewed_at, reviewed_by, admin_action, created_at, updated_at
		FROM review_reports
		WHERE id = $1;
	`

	var rep domain.ReviewReport
	err := conn.QueryRow(ctx, query, id).Scan(
		&rep.ID,
		&rep.ReviewID,
		&rep.ReporterUserID,
		&rep.Reason,
		&rep.Explanation,
		&rep.Status,
		&rep.ReviewedAt,
		&rep.ReviewedBy,
		&rep.AdminAction,
		&rep.CreatedAt,
		&rep.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "ReviewReport")
	}

	return &rep, nil
}

func (r *postgresReportRepository) ListPendingReviewReports(ctx context.Context, limit, offset int) ([]domain.ReviewReport, error) {
	conn := GetDB(ctx, r.pool.Pool)

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT
			id, review_id, reporter_user_id, reason, explanation,
			status, reviewed_at, reviewed_by, admin_action, created_at, updated_at
		FROM review_reports
		WHERE status = 'PENDING'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "ReviewReport")
	}
	defer rows.Close()

	var reports []domain.ReviewReport
	for rows.Next() {
		var rep domain.ReviewReport
		if err := rows.Scan(
			&rep.ID,
			&rep.ReviewID,
			&rep.ReporterUserID,
			&rep.Reason,
			&rep.Explanation,
			&rep.Status,
			&rep.ReviewedAt,
			&rep.ReviewedBy,
			&rep.AdminAction,
			&rep.CreatedAt,
			&rep.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}

	return reports, nil
}

func (r *postgresReportRepository) ResolveReviewReport(ctx context.Context, id domain.ReportID, status domain.ReportStatus, reviewedBy *domain.UserID, action *string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE review_reports
		SET status = $1, reviewed_at = NOW(), reviewed_by = $2, admin_action = $3, updated_at = NOW()
		WHERE id = $4;
	`

	cmd, err := conn.Exec(ctx, query, status, reviewedBy, action, id)
	if err != nil {
		return MapPostgresError(err, "ReviewReport")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReportRepository) CreateBusinessReport(ctx context.Context, rep *domain.BusinessReport) error {
	conn := GetDB(ctx, r.pool.Pool)

	if rep.Status == "" {
		rep.Status = domain.ReportStatusPending
	}

	query := `
		INSERT INTO business_reports (
			id, business_id, reporter_user_id, reason, details,
			status, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		rep.ID,
		rep.BusinessID,
		rep.ReporterUserID,
		rep.Reason,
		rep.Details,
		rep.Status,
	).Scan(&rep.ID, &rep.CreatedAt, &rep.UpdatedAt)

	return MapPostgresError(err, "BusinessReport")
}

func (r *postgresReportRepository) GetBusinessReportByID(ctx context.Context, id domain.ReportID) (*domain.BusinessReport, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, reporter_user_id, reason, details,
			status, reviewed_at, reviewed_by, admin_notes, created_at, updated_at
		FROM business_reports
		WHERE id = $1;
	`

	var rep domain.BusinessReport
	err := conn.QueryRow(ctx, query, id).Scan(
		&rep.ID,
		&rep.BusinessID,
		&rep.ReporterUserID,
		&rep.Reason,
		&rep.Details,
		&rep.Status,
		&rep.ReviewedAt,
		&rep.ReviewedBy,
		&rep.AdminNotes,
		&rep.CreatedAt,
		&rep.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "BusinessReport")
	}

	return &rep, nil
}

func (r *postgresReportRepository) ListPendingBusinessReports(ctx context.Context, limit, offset int) ([]domain.BusinessReport, error) {
	conn := GetDB(ctx, r.pool.Pool)

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := `
		SELECT
			id, business_id, reporter_user_id, reason, details,
			status, reviewed_at, reviewed_by, admin_notes, created_at, updated_at
		FROM business_reports
		WHERE status = 'PENDING'
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessReport")
	}
	defer rows.Close()

	var reports []domain.BusinessReport
	for rows.Next() {
		var rep domain.BusinessReport
		if err := rows.Scan(
			&rep.ID,
			&rep.BusinessID,
			&rep.ReporterUserID,
			&rep.Reason,
			&rep.Details,
			&rep.Status,
			&rep.ReviewedAt,
			&rep.ReviewedBy,
			&rep.AdminNotes,
			&rep.CreatedAt,
			&rep.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reports = append(reports, rep)
	}

	return reports, nil
}

func (r *postgresReportRepository) ResolveBusinessReport(ctx context.Context, id domain.ReportID, status domain.ReportStatus, reviewedBy *domain.UserID, notes *string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE business_reports
		SET status = $1, reviewed_at = NOW(), reviewed_by = $2, admin_notes = $3, updated_at = NOW()
		WHERE id = $4;
	`

	cmd, err := conn.Exec(ctx, query, status, reviewedBy, notes, id)
	if err != nil {
		return MapPostgresError(err, "BusinessReport")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
