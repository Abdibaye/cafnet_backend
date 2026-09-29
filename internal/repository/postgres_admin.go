package repository

import (
	"context"
	"encoding/json"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresAdminActionRepository struct {
	pool *db.Pool
}

func NewAdminActionRepository(pool *db.Pool) AdminActionRepository {
	return &postgresAdminActionRepository{pool: pool}
}

func (r *postgresAdminActionRepository) Record(ctx context.Context, action *domain.AdminAction) error {
	conn := GetDB(ctx, r.pool.Pool)

	metaJSON, err := json.Marshal(action.Metadata)
	if err != nil {
		return domain.ErrInvalidInput
	}
	if action.Metadata == nil {
		metaJSON = []byte("{}")
	}

	query := `
		INSERT INTO admin_actions (
			id, admin_user_id, action_type, target_entity_type,
			target_entity_id, metadata, created_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4,
			$5, $6, NOW()
		)
		RETURNING id, created_at;
	`

	err = conn.QueryRow(ctx, query,
		action.ID,
		action.AdminUserID,
		action.ActionType,
		action.TargetEntityType,
		action.TargetEntityID,
		metaJSON,
	).Scan(&action.ID, &action.CreatedAt)

	return MapPostgresError(err, "AdminAction")
}

func (r *postgresAdminActionRepository) List(ctx context.Context, limit, offset int) ([]domain.AdminAction, error) {
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
			id, admin_user_id, action_type, target_entity_type,
			target_entity_id, metadata, created_at
		FROM admin_actions
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2;
	`

	rows, err := conn.Query(ctx, query, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "AdminAction")
	}
	defer rows.Close()

	var actions []domain.AdminAction
	for rows.Next() {
		var action domain.AdminAction
		var rawMeta []byte

		if err := rows.Scan(
			&action.ID,
			&action.AdminUserID,
			&action.ActionType,
			&action.TargetEntityType,
			&action.TargetEntityID,
			&rawMeta,
			&action.CreatedAt,
		); err != nil {
			return nil, err
		}

		if len(rawMeta) > 0 {
			if err := json.Unmarshal(rawMeta, &action.Metadata); err != nil {
				return nil, err
			}
		}

		actions = append(actions, action)
	}

	return actions, nil
}
