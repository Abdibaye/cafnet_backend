package repository

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresUserRepository struct {
	pool *db.Pool
}

func NewUserRepository(pool *db.Pool) UserRepository {
	return &postgresUserRepository{pool: pool}
}

func (r *postgresUserRepository) Create(ctx context.Context, user *domain.User) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO users (
			id, phone, full_name, avatar_url, role, is_active, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, $6, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		user.ID,
		user.Phone,
		user.FullName,
		user.AvatarURL,
		user.Role,
		user.IsActive,
	).Scan(&user.ID, &user.CreatedAt, &user.UpdatedAt)

	return MapPostgresError(err, "User")
}

func (r *postgresUserRepository) GetByID(ctx context.Context, id domain.UserID) (*domain.User, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, phone, full_name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var u domain.User
	err := conn.QueryRow(ctx, query, id).Scan(
		&u.ID,
		&u.Phone,
		&u.FullName,
		&u.AvatarURL,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "User")
	}
	return &u, nil
}

func (r *postgresUserRepository) GetByPhone(ctx context.Context, phone string) (*domain.User, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, phone, full_name, avatar_url, role, is_active, created_at, updated_at, deleted_at
		FROM users
		WHERE phone = $1 AND deleted_at IS NULL;
	`

	var u domain.User
	err := conn.QueryRow(ctx, query, phone).Scan(
		&u.ID,
		&u.Phone,
		&u.FullName,
		&u.AvatarURL,
		&u.Role,
		&u.IsActive,
		&u.CreatedAt,
		&u.UpdatedAt,
		&u.DeletedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "User")
	}
	return &u, nil
}

func (r *postgresUserRepository) Update(ctx context.Context, user *domain.User) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE users
		SET
			full_name = $1,
			avatar_url = $2,
			role = $3,
			is_active = $4,
			updated_at = NOW()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		user.FullName,
		user.AvatarURL,
		user.Role,
		user.IsActive,
		user.ID,
	).Scan(&user.UpdatedAt)

	return MapPostgresError(err, "User")
}
