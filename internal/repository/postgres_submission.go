package repository

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresSubmissionRepository struct {
	pool *db.Pool
}

func NewSubmissionRepository(pool *db.Pool) SubmissionRepository {
	return &postgresSubmissionRepository{pool: pool}
}

func (r *postgresSubmissionRepository) Create(ctx context.Context, sub *domain.BusinessSubmission) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO business_submissions (
			id, submitter_user_id, business_name, category, area,
			submitted_payload, status, submitted_at, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, NOW(), NOW(), NOW()
		)
		RETURNING id, submitted_at, created_at, updated_at;
	`

	if sub.Status == "" {
		sub.Status = domain.SubmissionStatusPending
	}
	if sub.SubmittedPayload == nil {
		sub.SubmittedPayload = make(map[string]any)
	}

	err := conn.QueryRow(ctx, query,
		sub.ID,
		sub.SubmitterUserID,
		sub.BusinessName,
		sub.Category,
		sub.Area,
		sub.SubmittedPayload,
		sub.Status,
	).Scan(&sub.ID, &sub.SubmittedAt, &sub.CreatedAt, &sub.UpdatedAt)

	return MapPostgresError(err, "BusinessSubmission")
}

func (r *postgresSubmissionRepository) GetByID(ctx context.Context, id domain.SubmissionID) (*domain.BusinessSubmission, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, submitter_user_id, business_name, category, area,
			submitted_payload, status, created_business_id, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_submissions
		WHERE id = $1;
	`

	var s domain.BusinessSubmission
	err := conn.QueryRow(ctx, query, id).Scan(
		&s.ID, &s.SubmitterUserID, &s.BusinessName, &s.Category, &s.Area,
		&s.SubmittedPayload, &s.Status, &s.CreatedBusinessID, &s.SubmittedAt,
		&s.ReviewedAt, &s.ReviewedBy, &s.RejectionReason, &s.CreatedAt, &s.UpdatedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "BusinessSubmission")
	}
	return &s, nil
}

func (r *postgresSubmissionRepository) ListPending(ctx context.Context, limit, offset int) ([]domain.BusinessSubmission, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, submitter_user_id, business_name, category, area,
			submitted_payload, status, created_business_id, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_submissions
		WHERE status = 'PENDING'
		ORDER BY submitted_at ASC
		LIMIT $1 OFFSET $2;
	`

	if limit <= 0 {
		limit = 50
	}

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessSubmission")
	}
	defer rows.Close()

	var submissions []domain.BusinessSubmission
	for rows.Next() {
		var s domain.BusinessSubmission
		if err := rows.Scan(
			&s.ID, &s.SubmitterUserID, &s.BusinessName, &s.Category, &s.Area,
			&s.SubmittedPayload, &s.Status, &s.CreatedBusinessID, &s.SubmittedAt,
			&s.ReviewedAt, &s.ReviewedBy, &s.RejectionReason, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, MapPostgresError(err, "BusinessSubmission")
		}
		submissions = append(submissions, s)
	}
	return submissions, nil
}

func (r *postgresSubmissionRepository) ListBySubmitter(ctx context.Context, submitterID domain.UserID, limit, offset int) ([]domain.BusinessSubmission, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, submitter_user_id, business_name, category, area,
			submitted_payload, status, created_business_id, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_submissions
		WHERE submitter_user_id = $1
		ORDER BY submitted_at DESC
		LIMIT $2 OFFSET $3;
	`

	if limit <= 0 {
		limit = 50
	}

	rows, err := conn.Query(ctx, query, submitterID, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessSubmission")
	}
	defer rows.Close()

	var submissions []domain.BusinessSubmission
	for rows.Next() {
		var s domain.BusinessSubmission
		if err := rows.Scan(
			&s.ID, &s.SubmitterUserID, &s.BusinessName, &s.Category, &s.Area,
			&s.SubmittedPayload, &s.Status, &s.CreatedBusinessID, &s.SubmittedAt,
			&s.ReviewedAt, &s.ReviewedBy, &s.RejectionReason, &s.CreatedAt, &s.UpdatedAt,
		); err != nil {
			return nil, MapPostgresError(err, "BusinessSubmission")
		}
		submissions = append(submissions, s)
	}
	return submissions, nil
}

func (r *postgresSubmissionRepository) UpdateStatus(ctx context.Context, id domain.SubmissionID, status domain.SubmissionWorkflowStatus, createdBusinessID *domain.BusinessID, reviewedBy *domain.UserID, reason *string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE business_submissions
		SET
			status = $1,
			created_business_id = $2,
			reviewed_at = NOW(),
			reviewed_by = $3,
			rejection_reason = $4,
			updated_at = NOW()
		WHERE id = $5;
	`

	tag, err := conn.Exec(ctx, query, status, createdBusinessID, reviewedBy, reason, id)
	if err != nil {
		return MapPostgresError(err, "BusinessSubmission")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
