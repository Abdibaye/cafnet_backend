package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresOTPRepository struct {
	pool *db.Pool
}

func NewOTPRepository(pool *db.Pool) OTPRepository {
	return &postgresOTPRepository{pool: pool}
}

func (r *postgresOTPRepository) CreateChallenge(ctx context.Context, c *domain.OTPChallenge) error {
	conn := GetDB(ctx, r.pool.Pool)

	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 3
	}

	query := `
		INSERT INTO otp_challenges (
			id, phone, otp_hash, expires_at, attempts, max_attempts,
			is_consumed, ip_address, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6,
			$7, $8, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		c.ID,
		c.Phone,
		c.OTPCodeHash,
		c.ExpiresAt,
		c.Attempts,
		c.MaxAttempts,
		c.IsConsumed,
		c.IPAddress,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)

	return MapPostgresError(err, "OTPChallenge")
}

const otpChallengeCols = `
	id, phone, otp_hash, expires_at, attempts, max_attempts,
	is_consumed, consumed_at, ip_address, created_at, updated_at
`

func scanOTPChallenge(row pgx.Row) (*domain.OTPChallenge, error) {
	var c domain.OTPChallenge
	err := row.Scan(
		&c.ID,
		&c.Phone,
		&c.OTPCodeHash,
		&c.ExpiresAt,
		&c.Attempts,
		&c.MaxAttempts,
		&c.IsConsumed,
		&c.ConsumedAt,
		&c.IPAddress,
		&c.CreatedAt,
		&c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *postgresOTPRepository) GetByID(ctx context.Context, id string) (*domain.OTPChallenge, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `SELECT ` + otpChallengeCols + ` FROM otp_challenges WHERE id = $1;`

	c, err := scanOTPChallenge(conn.QueryRow(ctx, query, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "OTPChallenge")
	}

	return c, nil
}

func (r *postgresOTPRepository) GetForUpdate(ctx context.Context, id string) (*domain.OTPChallenge, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `SELECT ` + otpChallengeCols + ` FROM otp_challenges WHERE id = $1 FOR UPDATE;`

	c, err := scanOTPChallenge(conn.QueryRow(ctx, query, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "OTPChallenge")
	}

	return c, nil
}

func (r *postgresOTPRepository) IncrementAttempts(ctx context.Context, id string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE otp_challenges
		SET attempts = attempts + 1, updated_at = NOW()
		WHERE id = $1;
	`

	_, err := conn.Exec(ctx, query, id)
	return MapPostgresError(err, "OTPChallenge")
}

func (r *postgresOTPRepository) ConsumeChallenge(ctx context.Context, id string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE otp_challenges
		SET is_consumed = TRUE, consumed_at = NOW(), updated_at = NOW()
		WHERE id = $1;
	`

	_, err := conn.Exec(ctx, query, id)
	return MapPostgresError(err, "OTPChallenge")
}

func (r *postgresOTPRepository) InvalidatePrevious(ctx context.Context, phone string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE otp_challenges
		SET is_consumed = TRUE, consumed_at = NOW(), updated_at = NOW()
		WHERE phone = $1 AND is_consumed = FALSE;
	`

	_, err := conn.Exec(ctx, query, phone)
	return MapPostgresError(err, "OTPChallenge")
}

func (r *postgresOTPRepository) GetRecentRequest(ctx context.Context, phone string, cooldown time.Duration) (*domain.OTPChallenge, error) {
	conn := GetDB(ctx, r.pool.Pool)

	cutoff := time.Now().UTC().Add(-cooldown)
	query := `
		SELECT ` + otpChallengeCols + `
		FROM otp_challenges
		WHERE phone = $1 AND created_at > $2
		ORDER BY created_at DESC
		LIMIT 1;
	`

	c, err := scanOTPChallenge(conn.QueryRow(ctx, query, phone, cutoff))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil // No recent request within cooldown
		}
		return nil, MapPostgresError(err, "OTPChallenge")
	}

	return c, nil
}
