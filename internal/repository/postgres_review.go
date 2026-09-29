package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresReviewRepository struct {
	pool *db.Pool
}

func NewReviewRepository(pool *db.Pool) ReviewRepository {
	return &postgresReviewRepository{pool: pool}
}

func (r *postgresReviewRepository) Create(ctx context.Context, rev *domain.Review) error {
	conn := GetDB(ctx, r.pool.Pool)

	if rev.Status == "" {
		rev.Status = domain.ReviewStatusVisible
	}

	query := `
		INSERT INTO reviews (
			id, business_id, customer_user_id, reservation_id, rating,
			review_text, is_verified_visit, status, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		rev.ID,
		rev.BusinessID,
		rev.CustomerUserID,
		rev.ReservationID,
		rev.Rating,
		rev.ReviewText,
		rev.IsVerifiedVisit,
		rev.Status,
	).Scan(&rev.ID, &rev.CreatedAt, &rev.UpdatedAt)

	return MapPostgresError(err, "Review")
}

const reviewSelectCols = `
	r.id, r.business_id, r.customer_user_id, r.reservation_id, r.rating,
	r.review_text, r.is_verified_visit, r.status,
	(SELECT COUNT(*) FROM review_helpful_votes WHERE review_id = r.id) AS helpful_count,
	r.created_at, r.updated_at, r.deleted_at
`

func scanReview(row pgx.Row) (*domain.Review, error) {
	var rev domain.Review
	err := row.Scan(
		&rev.ID,
		&rev.BusinessID,
		&rev.CustomerUserID,
		&rev.ReservationID,
		&rev.Rating,
		&rev.ReviewText,
		&rev.IsVerifiedVisit,
		&rev.Status,
		&rev.HelpfulCount,
		&rev.CreatedAt,
		&rev.UpdatedAt,
		&rev.DeletedAt,
	)
	if err != nil {
		return nil, err
	}
	return &rev, nil
}

func (r *postgresReviewRepository) GetByID(ctx context.Context, id domain.ReviewID) (*domain.Review, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `SELECT ` + reviewSelectCols + ` FROM reviews r WHERE r.id = $1 AND r.deleted_at IS NULL;`

	rev, err := scanReview(conn.QueryRow(ctx, query, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "Review")
	}

	return rev, nil
}

func (r *postgresReviewRepository) GetByReservationID(ctx context.Context, resID domain.ReservationID) (*domain.Review, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `SELECT ` + reviewSelectCols + ` FROM reviews r WHERE r.reservation_id = $1 AND r.deleted_at IS NULL;`

	rev, err := scanReview(conn.QueryRow(ctx, query, resID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "Review")
	}

	return rev, nil
}

func (r *postgresReviewRepository) ListForBusiness(ctx context.Context, businessID domain.BusinessID, limit, offset int) ([]domain.Review, error) {
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
		SELECT ` + reviewSelectCols + `
		FROM reviews r
		WHERE r.business_id = $1 AND r.status = 'VISIBLE' AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC
		LIMIT $2 OFFSET $3;
	`

	rows, err := conn.Query(ctx, query, businessID, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "Review")
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var rev domain.Review
		if err := rows.Scan(
			&rev.ID,
			&rev.BusinessID,
			&rev.CustomerUserID,
			&rev.ReservationID,
			&rev.Rating,
			&rev.ReviewText,
			&rev.IsVerifiedVisit,
			&rev.Status,
			&rev.HelpfulCount,
			&rev.CreatedAt,
			&rev.UpdatedAt,
			&rev.DeletedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, rev)
	}

	return reviews, nil
}

func (r *postgresReviewRepository) ListForCustomer(ctx context.Context, customerID domain.UserID) ([]domain.Review, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT ` + reviewSelectCols + `
		FROM reviews r
		WHERE r.customer_user_id = $1 AND r.deleted_at IS NULL
		ORDER BY r.created_at DESC;
	`

	rows, err := conn.Query(ctx, query, customerID)
	if err != nil {
		return nil, MapPostgresError(err, "Review")
	}
	defer rows.Close()

	var reviews []domain.Review
	for rows.Next() {
		var rev domain.Review
		if err := rows.Scan(
			&rev.ID,
			&rev.BusinessID,
			&rev.CustomerUserID,
			&rev.ReservationID,
			&rev.Rating,
			&rev.ReviewText,
			&rev.IsVerifiedVisit,
			&rev.Status,
			&rev.HelpfulCount,
			&rev.CreatedAt,
			&rev.UpdatedAt,
			&rev.DeletedAt,
		); err != nil {
			return nil, err
		}
		reviews = append(reviews, rev)
	}

	return reviews, nil
}

func (r *postgresReviewRepository) Update(ctx context.Context, id domain.ReviewID, rating int16, text string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reviews
		SET rating = $1, review_text = $2, updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, rating, text, id)
	if err != nil {
		return MapPostgresError(err, "Review")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReviewRepository) SoftDelete(ctx context.Context, id domain.ReviewID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reviews
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "Review")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReviewRepository) SetModerationStatus(ctx context.Context, id domain.ReviewID, status domain.ReviewStatus) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reviews
		SET status = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, status, id)
	if err != nil {
		return MapPostgresError(err, "Review")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReviewRepository) GetSummary(ctx context.Context, businessID domain.BusinessID) (*domain.ReviewSummary, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			COUNT(*) AS total_count,
			COALESCE(AVG(rating), 0.0)::float8 AS avg_rating,
			COUNT(*) FILTER (WHERE rating = 5) AS count_5,
			COUNT(*) FILTER (WHERE rating = 4) AS count_4,
			COUNT(*) FILTER (WHERE rating = 3) AS count_3,
			COUNT(*) FILTER (WHERE rating = 2) AS count_2,
			COUNT(*) FILTER (WHERE rating = 1) AS count_1
		FROM reviews
		WHERE business_id = $1 AND status = 'VISIBLE' AND deleted_at IS NULL;
	`

	var count int
	var avg float64
	var c5, c4, c3, c2, c1 int

	err := conn.QueryRow(ctx, query, businessID).Scan(&count, &avg, &c5, &c4, &c3, &c2, &c1)
	if err != nil {
		return nil, MapPostgresError(err, "Review")
	}

	return &domain.ReviewSummary{
		BusinessID:   businessID,
		Count:        count,
		Average:      avg,
		Distribution: [5]int{c5, c4, c3, c2, c1},
	}, nil
}

func (r *postgresReviewRepository) AddHelpfulVote(ctx context.Context, vote *domain.ReviewHelpfulVote) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO review_helpful_votes (review_id, customer_user_id, created_at)
		VALUES ($1, $2, NOW())
		RETURNING created_at;
	`

	err := conn.QueryRow(ctx, query, vote.ReviewID, vote.CustomerUserID).Scan(&vote.CreatedAt)
	return MapPostgresError(err, "ReviewHelpfulVote")
}

func (r *postgresReviewRepository) RemoveHelpfulVote(ctx context.Context, reviewID domain.ReviewID, userID domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM review_helpful_votes
		WHERE review_id = $1 AND customer_user_id = $2;
	`

	cmd, err := conn.Exec(ctx, query, reviewID, userID)
	if err != nil {
		return MapPostgresError(err, "ReviewHelpfulVote")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReviewRepository) HasVotedHelpful(ctx context.Context, reviewID domain.ReviewID, userID domain.UserID) (bool, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT EXISTS (
			SELECT 1 FROM review_helpful_votes
			WHERE review_id = $1 AND customer_user_id = $2
		);
	`

	var exists bool
	err := conn.QueryRow(ctx, query, reviewID, userID).Scan(&exists)
	if err != nil {
		return false, MapPostgresError(err, "ReviewHelpfulVote")
	}

	return exists, nil
}
