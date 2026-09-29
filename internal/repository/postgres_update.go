package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresUpdateRepository struct {
	pool *db.Pool
}

func NewUpdateRepository(pool *db.Pool) UpdateRepository {
	return &postgresUpdateRepository{pool: pool}
}

func (r *postgresUpdateRepository) Create(ctx context.Context, u *domain.BusinessUpdate) error {
	conn := GetDB(ctx, r.pool.Pool)

	if u.Status == "" {
		u.Status = domain.UpdateStatusPublished
	}

	var pubAt *time.Time
	if !u.PublishedAt.IsZero() {
		pubAt = &u.PublishedAt
	}

	query := `
		INSERT INTO business_updates (
			id, business_id, author_user_id, update_type, title, description,
			status, event_date, event_time, price_text, offer_details, valid_until,
			expires_at, published_at, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12,
			$13, COALESCE($14, NOW()), NOW(), NOW()
		)
		RETURNING id, published_at, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		u.ID,
		u.BusinessID,
		u.AuthorUserID,
		u.UpdateType,
		u.Title,
		u.Description,
		u.Status,
		u.EventDate,
		u.EventTime,
		u.PriceText,
		u.OfferDetails,
		u.ValidUntil,
		u.ExpiresAt,
		pubAt,
	).Scan(&u.ID, &u.PublishedAt, &u.CreatedAt, &u.UpdatedAt)

	return MapPostgresError(err, "BusinessUpdate")
}

func (r *postgresUpdateRepository) GetByID(ctx context.Context, id domain.UpdateID) (*domain.BusinessUpdate, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, author_user_id, update_type, title, description,
			status, event_date, event_time, price_text, offer_details, valid_until,
			expires_at, published_at, created_at, updated_at, deleted_at
		FROM business_updates
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var u domain.BusinessUpdate
	err := conn.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.BusinessID,
		&u.AuthorUserID,
		&u.UpdateType,
		&u.Title,
		&u.Description,
		&u.Status,
		&u.EventDate,
		&u.EventTime,
		&u.PriceText,
		&u.OfferDetails,
		&u.ValidUntil,
		&u.ExpiresAt,
		&u.PublishedAt,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "BusinessUpdate")
	}

	return &u, nil
}

func (r *postgresUpdateRepository) ListActive(ctx context.Context, businessID *domain.BusinessID, limit, offset int) ([]domain.BusinessUpdate, error) {
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

	var conditions []string
	var args []any
	argIdx := 1

	conditions = append(conditions, "status = 'PUBLISHED'")
	conditions = append(conditions, "deleted_at IS NULL")
	conditions = append(conditions, "(expires_at IS NULL OR expires_at > NOW())")

	if businessID != nil {
		conditions = append(conditions, fmt.Sprintf("business_id = $%d", argIdx))
		args = append(args, *businessID)
		argIdx++
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT
			id, business_id, author_user_id, update_type, title, description,
			status, event_date, event_time, price_text, offer_details, valid_until,
			expires_at, published_at, created_at, updated_at, deleted_at
		FROM business_updates
		WHERE %s
		ORDER BY published_at DESC
		LIMIT $%d OFFSET $%d;
	`, strings.Join(conditions, " AND "), argIdx, argIdx+1)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessUpdate")
	}
	defer rows.Close()

	var updates []domain.BusinessUpdate
	for rows.Next() {
		var u domain.BusinessUpdate
		if err := rows.Scan(
			&u.ID,
			&u.BusinessID,
			&u.AuthorUserID,
			&u.UpdateType,
			&u.Title,
			&u.Description,
			&u.Status,
			&u.EventDate,
			&u.EventTime,
			&u.PriceText,
			&u.OfferDetails,
			&u.ValidUntil,
			&u.ExpiresAt,
			&u.PublishedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.DeletedAt,
		); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}

	return updates, nil
}

func (r *postgresUpdateRepository) ListByBusiness(ctx context.Context, businessID domain.BusinessID, includeExpired bool) ([]domain.BusinessUpdate, error) {
	conn := GetDB(ctx, r.pool.Pool)

	var condition string
	if includeExpired {
		condition = "business_id = $1 AND deleted_at IS NULL"
	} else {
		condition = "business_id = $1 AND deleted_at IS NULL AND status = 'PUBLISHED' AND (expires_at IS NULL OR expires_at > NOW())"
	}

	query := fmt.Sprintf(`
		SELECT
			id, business_id, author_user_id, update_type, title, description,
			status, event_date, event_time, price_text, offer_details, valid_until,
			expires_at, published_at, created_at, updated_at, deleted_at
		FROM business_updates
		WHERE %s
		ORDER BY published_at DESC;
	`, condition)

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessUpdate")
	}
	defer rows.Close()

	var updates []domain.BusinessUpdate
	for rows.Next() {
		var u domain.BusinessUpdate
		if err := rows.Scan(
			&u.ID,
			&u.BusinessID,
			&u.AuthorUserID,
			&u.UpdateType,
			&u.Title,
			&u.Description,
			&u.Status,
			&u.EventDate,
			&u.EventTime,
			&u.PriceText,
			&u.OfferDetails,
			&u.ValidUntil,
			&u.ExpiresAt,
			&u.PublishedAt,
			&u.CreatedAt,
			&u.UpdatedAt,
			&u.DeletedAt,
		); err != nil {
			return nil, err
		}
		updates = append(updates, u)
	}

	return updates, nil
}

func (r *postgresUpdateRepository) Update(ctx context.Context, u *domain.BusinessUpdate) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE business_updates
		SET
			update_type = $1,
			title = $2,
			description = $3,
			status = $4,
			event_date = $5,
			event_time = $6,
			price_text = $7,
			offer_details = $8,
			valid_until = $9,
			expires_at = $10,
			updated_at = NOW()
		WHERE id = $11 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		u.UpdateType,
		u.Title,
		u.Description,
		u.Status,
		u.EventDate,
		u.EventTime,
		u.PriceText,
		u.OfferDetails,
		u.ValidUntil,
		u.ExpiresAt,
		u.ID,
	).Scan(&u.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrNotFound
		}
		return MapPostgresError(err, "BusinessUpdate")
	}

	return nil
}

func (r *postgresUpdateRepository) Delete(ctx context.Context, id domain.UpdateID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE business_updates
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "BusinessUpdate")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
