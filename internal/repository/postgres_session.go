package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresSessionRepository struct {
	pool *db.Pool
}

func NewSessionRepository(pool *db.Pool) SessionRepository {
	return &postgresSessionRepository{pool: pool}
}

func (r *postgresSessionRepository) Create(ctx context.Context, session *domain.Session) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO sessions (
			id, user_id, token_hash, user_agent, ip_address, expires_at, created_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, NOW()
		)
		RETURNING id, created_at;
	`

	err := conn.QueryRow(ctx, query,
		session.ID,
		session.UserID,
		session.TokenHash,
		session.UserAgent,
		session.IPAddress,
		session.ExpiresAt,
	).Scan(&session.ID, &session.CreatedAt)

	return MapPostgresError(err, "Session")
}

func (r *postgresSessionRepository) GetByTokenHash(ctx context.Context, tokenHash string) (*domain.Session, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, user_id, token_hash, user_agent, ip_address,
			expires_at, last_used_at, revoked_at, created_at
		FROM sessions
		WHERE token_hash = $1;
	`

	var s domain.Session
	err := conn.QueryRow(ctx, query, tokenHash).Scan(
		&s.ID,
		&s.UserID,
		&s.TokenHash,
		&s.UserAgent,
		&s.IPAddress,
		&s.ExpiresAt,
		&s.LastUsedAt,
		&s.RevokedAt,
		&s.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrSessionNotFound
		}
		return nil, MapPostgresError(err, "Session")
	}

	return &s, nil
}

func (r *postgresSessionRepository) UpdateLastUsed(ctx context.Context, id string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE sessions
		SET last_used_at = NOW()
		WHERE id = $1;
	`

	_, err := conn.Exec(ctx, query, id)
	return MapPostgresError(err, "Session")
}

func (r *postgresSessionRepository) Revoke(ctx context.Context, id string) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE sessions
		SET revoked_at = NOW()
		WHERE id = $1 AND revoked_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "Session")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrSessionNotFound
	}

	return nil
}

func (r *postgresSessionRepository) RevokeAllForUser(ctx context.Context, userID domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE sessions
		SET revoked_at = NOW()
		WHERE user_id = $1 AND revoked_at IS NULL;
	`

	_, err := conn.Exec(ctx, query, userID)
	return MapPostgresError(err, "Session")
}

func (r *postgresSessionRepository) DeleteExpired(ctx context.Context) (int64, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM sessions
		WHERE expires_at < NOW();
	`

	cmd, err := conn.Exec(ctx, query)
	if err != nil {
		return 0, MapPostgresError(err, "Session")
	}

	return cmd.RowsAffected(), nil
}
