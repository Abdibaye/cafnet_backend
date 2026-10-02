package repository

import (
	"context"
	"fmt"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresMediaRepository struct {
	pool *db.Pool
}

// NewMediaRepository constructs a PostgreSQL backed MediaRepository.
func NewMediaRepository(pool *db.Pool) MediaRepository {
	return &postgresMediaRepository{pool: pool}
}

func (r *postgresMediaRepository) Create(ctx context.Context, media *domain.Media) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO media (
			id, created_by_user_id, business_id, menu_item_id, submission_id,
			media_type, object_key, content_type, file_size, status, sort_order
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8, $9, $10, $11
		)
		RETURNING id, created_at, updated_at;
	`

	var rawID string
	var subID *string
	if media.SubmissionID != nil {
		s := string(*media.SubmissionID)
		subID = &s
	}
	var menuItemID *string
	if media.MenuItemID != nil {
		m := string(*media.MenuItemID)
		menuItemID = &m
	}

	err := conn.QueryRow(
		ctx, query,
		string(media.ID), media.CreatedByUserID, media.BusinessID, menuItemID, subID,
		string(media.MediaType), media.ObjectKey, media.ContentType, media.FileSize, string(media.Status), media.SortOrder,
	).Scan(&rawID, &media.CreatedAt, &media.UpdatedAt)
	if err != nil {
		return MapPostgresError(err, "Media")
	}

	media.ID = domain.MediaID(rawID)
	return nil
}

func (r *postgresMediaRepository) GetByID(ctx context.Context, id domain.MediaID) (*domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE id = $1;
	`

	var m domain.Media
	var rawID, creatorID string
	var bizID *domain.BusinessID
	var itemID, subID *string
	var mediaType, status string

	err := conn.QueryRow(ctx, query, string(id)).Scan(
		&rawID, &creatorID, &bizID, &itemID, &subID,
		&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	m.ID = domain.MediaID(rawID)
	m.CreatedByUserID = domain.UserID(creatorID)
	m.BusinessID = bizID
	if itemID != nil {
		t := domain.ItemID(*itemID)
		m.MenuItemID = &t
	}
	if subID != nil {
		s := domain.SubmissionID(*subID)
		m.SubmissionID = &s
	}
	m.MediaType = domain.MediaType(mediaType)
	m.Status = domain.MediaStatus(status)

	return &m, nil
}

func (r *postgresMediaRepository) GetByObjectKey(ctx context.Context, objectKey string) (*domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE object_key = $1;
	`

	var m domain.Media
	var rawID, creatorID string
	var bizID *domain.BusinessID
	var itemID, subID *string
	var mediaType, status string

	err := conn.QueryRow(ctx, query, objectKey).Scan(
		&rawID, &creatorID, &bizID, &itemID, &subID,
		&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	m.ID = domain.MediaID(rawID)
	m.CreatedByUserID = domain.UserID(creatorID)
	m.BusinessID = bizID
	if itemID != nil {
		t := domain.ItemID(*itemID)
		m.MenuItemID = &t
	}
	if subID != nil {
		s := domain.SubmissionID(*subID)
		m.SubmissionID = &s
	}
	m.MediaType = domain.MediaType(mediaType)
	m.Status = domain.MediaStatus(status)

	return &m, nil
}

func (r *postgresMediaRepository) UpdateStatus(ctx context.Context, id domain.MediaID, status domain.MediaStatus, fileSize int64) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE media
		SET status = $1, file_size = CASE WHEN $2 > 0 THEN $2 ELSE file_size END, updated_at = NOW()
		WHERE id = $3;
	`

	tag, err := conn.Exec(ctx, query, string(status), fileSize, string(id))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}
	return nil
}

func (r *postgresMediaRepository) SetSortOrder(ctx context.Context, id domain.MediaID, sortOrder int) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE media
		SET sort_order = $1, updated_at = NOW()
		WHERE id = $2;
	`

	tag, err := conn.Exec(ctx, query, sortOrder, string(id))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}
	return nil
}

func (r *postgresMediaRepository) ListByBusiness(ctx context.Context, businessID domain.BusinessID) ([]domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE business_id = $1 AND status = 'READY'
		ORDER BY CASE WHEN media_type = 'BUSINESS_COVER' THEN 0 ELSE 1 END, sort_order ASC, created_at ASC;
	`

	return r.queryMediaList(ctx, conn, query, businessID)
}

func (r *postgresMediaRepository) GetBusinessCover(ctx context.Context, businessID domain.BusinessID) (*domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE business_id = $1 AND media_type = 'BUSINESS_COVER' AND status = 'READY'
		ORDER BY sort_order ASC, created_at DESC
		LIMIT 1;
	`

	var m domain.Media
	var rawID, creatorID string
	var bizID *domain.BusinessID
	var itemID, subID *string
	var mediaType, status string

	err := conn.QueryRow(ctx, query, businessID).Scan(
		&rawID, &creatorID, &bizID, &itemID, &subID,
		&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	m.ID = domain.MediaID(rawID)
	m.CreatedByUserID = domain.UserID(creatorID)
	m.BusinessID = bizID
	if itemID != nil {
		t := domain.ItemID(*itemID)
		m.MenuItemID = &t
	}
	if subID != nil {
		s := domain.SubmissionID(*subID)
		m.SubmissionID = &s
	}
	m.MediaType = domain.MediaType(mediaType)
	m.Status = domain.MediaStatus(status)

	return &m, nil
}

func (r *postgresMediaRepository) ListBusinessGallery(ctx context.Context, businessID domain.BusinessID) ([]domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE business_id = $1 AND media_type = 'BUSINESS_GALLERY' AND status = 'READY'
		ORDER BY sort_order ASC, created_at ASC;
	`

	return r.queryMediaList(ctx, conn, query, businessID)
}

func (r *postgresMediaRepository) GetBySubmissionID(ctx context.Context, submissionID domain.SubmissionID) (*domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE submission_id = $1 AND status = 'READY'
		ORDER BY created_at DESC
		LIMIT 1;
	`

	var m domain.Media
	var rawID, creatorID string
	var bizID *domain.BusinessID
	var itemID, subID *string
	var mediaType, status string

	err := conn.QueryRow(ctx, query, string(submissionID)).Scan(
		&rawID, &creatorID, &bizID, &itemID, &subID,
		&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	m.ID = domain.MediaID(rawID)
	m.CreatedByUserID = domain.UserID(creatorID)
	m.BusinessID = bizID
	if itemID != nil {
		t := domain.ItemID(*itemID)
		m.MenuItemID = &t
	}
	if subID != nil {
		s := domain.SubmissionID(*subID)
		m.SubmissionID = &s
	}
	m.MediaType = domain.MediaType(mediaType)
	m.Status = domain.MediaStatus(status)

	return &m, nil
}

func (r *postgresMediaRepository) GetByMenuItemID(ctx context.Context, itemID domain.ItemID) (*domain.Media, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, created_by_user_id, business_id, menu_item_id, submission_id,
		       media_type, object_key, content_type, file_size, status, sort_order,
		       created_at, updated_at
		FROM media
		WHERE menu_item_id = $1 AND status = 'READY'
		ORDER BY created_at DESC
		LIMIT 1;
	`

	var m domain.Media
	var rawID, creatorID string
	var bizID *domain.BusinessID
	var miID, subID *string
	var mediaType, status string

	err := conn.QueryRow(ctx, query, string(itemID)).Scan(
		&rawID, &creatorID, &bizID, &miID, &subID,
		&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
		&m.CreatedAt, &m.UpdatedAt,
	)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	m.ID = domain.MediaID(rawID)
	m.CreatedByUserID = domain.UserID(creatorID)
	m.BusinessID = bizID
	if miID != nil {
		t := domain.ItemID(*miID)
		m.MenuItemID = &t
	}
	if subID != nil {
		s := domain.SubmissionID(*subID)
		m.SubmissionID = &s
	}
	m.MediaType = domain.MediaType(mediaType)
	m.Status = domain.MediaStatus(status)

	return &m, nil
}

func (r *postgresMediaRepository) AssociateWithBusiness(ctx context.Context, mediaID domain.MediaID, businessID domain.BusinessID, mediaType domain.MediaType, sortOrder int) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE media
		SET business_id = $1, media_type = $2, sort_order = $3, updated_at = NOW()
		WHERE id = $4;
	`

	tag, err := conn.Exec(ctx, query, businessID, string(mediaType), sortOrder, string(mediaID))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}
	return nil
}

func (r *postgresMediaRepository) AssociateWithSubmission(ctx context.Context, mediaID domain.MediaID, submissionID domain.SubmissionID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Link in media table
	query := `
		UPDATE media
		SET submission_id = $1, updated_at = NOW()
		WHERE id = $2;
	`
	tag, err := conn.Exec(ctx, query, string(submissionID), string(mediaID))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}

	// Also link in business_submissions cover_media_id
	_, _ = conn.Exec(ctx, `UPDATE business_submissions SET cover_media_id = $1 WHERE id = $2;`, string(mediaID), string(submissionID))

	return nil
}

func (r *postgresMediaRepository) AssociateWithMenuItem(ctx context.Context, mediaID domain.MediaID, itemID domain.ItemID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Link in media table
	query := `
		UPDATE media
		SET menu_item_id = $1, updated_at = NOW()
		WHERE id = $2;
	`
	tag, err := conn.Exec(ctx, query, string(itemID), string(mediaID))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}

	// Also link in menu_items image_media_id
	_, _ = conn.Exec(ctx, `UPDATE menu_items SET image_media_id = $1 WHERE id = $2;`, string(mediaID), string(itemID))

	return nil
}

func (r *postgresMediaRepository) SetBusinessCover(ctx context.Context, businessID domain.BusinessID, mediaID domain.MediaID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Verify target media belongs to this business
	var count int
	err := conn.QueryRow(ctx, `SELECT COUNT(*) FROM media WHERE id = $1 AND business_id = $2;`, string(mediaID), businessID).Scan(&count)
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if count == 0 {
		return domain.NewDomainError(domain.ErrForbidden, "Media does not belong to this business", nil)
	}

	// Any current cover becomes a gallery image
	_, err = conn.Exec(ctx, `
		UPDATE media
		SET media_type = 'BUSINESS_GALLERY', updated_at = NOW()
		WHERE business_id = $1 AND media_type = 'BUSINESS_COVER';
	`, businessID)
	if err != nil {
		return MapPostgresError(err, "Media")
	}

	// Target media becomes BUSINESS_COVER
	_, err = conn.Exec(ctx, `
		UPDATE media
		SET media_type = 'BUSINESS_COVER', sort_order = 0, updated_at = NOW()
		WHERE id = $1;
	`, string(mediaID))
	if err != nil {
		return MapPostgresError(err, "Media")
	}

	return nil
}

func (r *postgresMediaRepository) ClearBusinessCover(ctx context.Context, businessID domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	_, err := conn.Exec(ctx, `
		UPDATE media
		SET media_type = 'BUSINESS_GALLERY', updated_at = NOW()
		WHERE business_id = $1 AND media_type = 'BUSINESS_COVER';
	`, businessID)
	return MapPostgresError(err, "Media")
}

func (r *postgresMediaRepository) ReorderBusinessGallery(ctx context.Context, businessID domain.BusinessID, mediaIDs []domain.MediaID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Security validation: verify every single media ID belongs to businessID and is BUSINESS_GALLERY
	for idx, id := range mediaIDs {
		tag, err := conn.Exec(ctx, `
			UPDATE media
			SET sort_order = $1, updated_at = NOW()
			WHERE id = $2 AND business_id = $3;
		`, idx, string(id), businessID)
		if err != nil {
			return MapPostgresError(err, "Media")
		}
		if tag.RowsAffected() == 0 {
			return domain.NewDomainError(domain.ErrForbidden, fmt.Sprintf("media ID %s does not belong to business %d", id, businessID), nil)
		}
	}
	return nil
}

func (r *postgresMediaRepository) Delete(ctx context.Context, id domain.MediaID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Clean references from business_submissions and menu_items
	_, _ = conn.Exec(ctx, `UPDATE business_submissions SET cover_media_id = NULL WHERE cover_media_id = $1;`, string(id))
	_, _ = conn.Exec(ctx, `UPDATE menu_items SET image_media_id = NULL WHERE image_media_id = $1;`, string(id))

	tag, err := conn.Exec(ctx, `DELETE FROM media WHERE id = $1;`, string(id))
	if err != nil {
		return MapPostgresError(err, "Media")
	}
	if tag.RowsAffected() == 0 {
		return domain.NewDomainError(domain.ErrNotFound, "Media not found", nil)
	}
	return nil
}

func (r *postgresMediaRepository) CountBusinessGallery(ctx context.Context, businessID domain.BusinessID) (int, error) {
	conn := GetDB(ctx, r.pool.Pool)

	var count int
	err := conn.QueryRow(ctx, `
		SELECT COUNT(*)
		FROM media
		WHERE business_id = $1 AND media_type = 'BUSINESS_GALLERY' AND status = 'READY';
	`, businessID).Scan(&count)
	if err != nil {
		return 0, MapPostgresError(err, "Media")
	}
	return count, nil
}

func (r *postgresMediaRepository) queryMediaList(ctx context.Context, conn DBTX, query string, args ...any) ([]domain.Media, error) {
	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, MapPostgresError(err, "Media")
	}
	defer rows.Close()

	var list []domain.Media
	for rows.Next() {
		var m domain.Media
		var rawID, creatorID string
		var bizID *domain.BusinessID
		var itemID, subID *string
		var mediaType, status string

		if err := rows.Scan(
			&rawID, &creatorID, &bizID, &itemID, &subID,
			&mediaType, &m.ObjectKey, &m.ContentType, &m.FileSize, &status, &m.SortOrder,
			&m.CreatedAt, &m.UpdatedAt,
		); err != nil {
			return nil, MapPostgresError(err, "Media")
		}

		m.ID = domain.MediaID(rawID)
		m.CreatedByUserID = domain.UserID(creatorID)
		m.BusinessID = bizID
		if itemID != nil {
			t := domain.ItemID(*itemID)
			m.MenuItemID = &t
		}
		if subID != nil {
			s := domain.SubmissionID(*subID)
			m.SubmissionID = &s
		}
		m.MediaType = domain.MediaType(mediaType)
		m.Status = domain.MediaStatus(status)

		list = append(list, m)
	}

	if err := rows.Err(); err != nil {
		return nil, MapPostgresError(err, "Media")
	}

	return list, nil
}
