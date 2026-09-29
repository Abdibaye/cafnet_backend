package repository

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresClaimRepository struct {
	pool *db.Pool
}

func NewClaimRepository(pool *db.Pool) ClaimRepository {
	return &postgresClaimRepository{pool: pool}
}

func (r *postgresClaimRepository) Create(ctx context.Context, claim *domain.BusinessClaim) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO business_claims (
			id, business_id, claimant_user_id, status, submitted_at, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, NOW(), NOW(), NOW()
		)
		RETURNING id, submitted_at, created_at, updated_at;
	`

	if claim.Status == "" {
		claim.Status = domain.ClaimStatusPending
	}

	err := conn.QueryRow(ctx, query,
		claim.ID,
		claim.BusinessID,
		claim.ClaimantUserID,
		claim.Status,
	).Scan(&claim.ID, &claim.SubmittedAt, &claim.CreatedAt, &claim.UpdatedAt)

	return MapPostgresError(err, "BusinessClaim")
}

func (r *postgresClaimRepository) GetByID(ctx context.Context, id domain.ClaimID) (*domain.BusinessClaim, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, claimant_user_id, status, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_claims
		WHERE id = $1;
	`

	var c domain.BusinessClaim
	err := conn.QueryRow(ctx, query, id).Scan(
		&c.ID, &c.BusinessID, &c.ClaimantUserID, &c.Status, &c.SubmittedAt,
		&c.ReviewedAt, &c.ReviewedBy, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "BusinessClaim")
	}
	return &c, nil
}

func (r *postgresClaimRepository) GetPendingForBusiness(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessClaim, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, claimant_user_id, status, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_claims
		WHERE business_id = $1 AND status = 'PENDING';
	`

	var c domain.BusinessClaim
	err := conn.QueryRow(ctx, query, businessID).Scan(
		&c.ID, &c.BusinessID, &c.ClaimantUserID, &c.Status, &c.SubmittedAt,
		&c.ReviewedAt, &c.ReviewedBy, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "BusinessClaim")
	}
	return &c, nil
}

func (r *postgresClaimRepository) ListPending(ctx context.Context, limit, offset int) ([]domain.BusinessClaim, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, claimant_user_id, status, submitted_at,
			reviewed_at, reviewed_by, rejection_reason, created_at, updated_at
		FROM business_claims
		WHERE status = 'PENDING'
		ORDER BY submitted_at ASC
		LIMIT $1 OFFSET $2;
	`

	if limit <= 0 {
		limit = 50
	}

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessClaim")
	}
	defer rows.Close()

	var claims []domain.BusinessClaim
	for rows.Next() {
		var c domain.BusinessClaim
		if err := rows.Scan(
			&c.ID, &c.BusinessID, &c.ClaimantUserID, &c.Status, &c.SubmittedAt,
			&c.ReviewedAt, &c.ReviewedBy, &c.RejectionReason, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, MapPostgresError(err, "BusinessClaim")
		}
		claims = append(claims, c)
	}
	return claims, nil
}

func (r *postgresClaimRepository) UpdateStatus(ctx context.Context, id domain.ClaimID, status domain.ClaimWorkflowStatus, reviewedBy *domain.UserID, reason *string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE business_claims
		SET
			status = $1,
			reviewed_at = NOW(),
			reviewed_by = $2,
			rejection_reason = $3,
			updated_at = NOW()
		WHERE id = $4;
	`

	tag, err := conn.Exec(ctx, query, status, reviewedBy, reason, id)
	if err != nil {
		return MapPostgresError(err, "BusinessClaim")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}
