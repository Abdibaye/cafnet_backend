package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresBusinessRepository struct {
	pool *db.Pool
}

func NewBusinessRepository(pool *db.Pool) BusinessRepository {
	return &postgresBusinessRepository{pool: pool}
}

func (r *postgresBusinessRepository) Create(ctx context.Context, b *domain.Business) error {
	conn := GetDB(ctx, r.pool.Pool)

	if b.PublicationStatus == "" {
		b.PublicationStatus = domain.PublicationStatusUnpublished
	}
	if b.VerificationStatus == "" {
		b.VerificationStatus = domain.VerificationStatusUnverified
	}
	if b.ClaimStatus == "" {
		b.ClaimStatus = domain.ClaimStatusUnclaimed
	}
	if b.PriceRange == "" {
		b.PriceRange = "$$"
	}
	if b.Tags == nil {
		b.Tags = []string{}
	}
	if b.PaymentMethods == nil {
		b.PaymentMethods = []string{"Cash"}
	}
	if b.CreationSource == "" {
		b.CreationSource = domain.CreationSourceOwnerSubmission
	}

	query := `
		INSERT INTO businesses (
			name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11,
			$12, $13, $14, $15, $16, $17,
			$18, $19, $20, $21, $22,
			$23, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		b.Name,
		b.Slug,
		b.Category,
		b.Cuisine,
		b.Description,
		b.ShortDescription,
		b.LocationName,
		b.Neighborhood,
		b.AddressLine,
		b.Latitude,
		b.Longitude,
		b.Phone,
		b.Website,
		b.PriceRange,
		b.Tags,
		b.PaymentMethods,
		b.ReservationAvailable,
		b.PublicationStatus,
		b.VerificationStatus,
		b.ClaimStatus,
		b.CreationSource,
		b.IsFeatured,
		b.CreatedByUserID,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)

	return MapPostgresError(err, "Business")
}

func (r *postgresBusinessRepository) GetByID(ctx context.Context, id domain.BusinessID) (*domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at, deleted_at
		FROM businesses
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var b domain.Business
	err := conn.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
		&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
		&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
		&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
		&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	return &b, nil
}

func (r *postgresBusinessRepository) GetByIDForUpdate(ctx context.Context, id domain.BusinessID) (*domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at, deleted_at
		FROM businesses
		WHERE id = $1 AND deleted_at IS NULL
		FOR UPDATE;
	`

	var b domain.Business
	err := conn.QueryRow(ctx, query, id).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
		&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
		&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
		&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
		&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	return &b, nil
}

func (r *postgresBusinessRepository) GetBySlug(ctx context.Context, slug string) (*domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at, deleted_at
		FROM businesses
		WHERE slug = $1 AND deleted_at IS NULL;
	`

	var b domain.Business
	err := conn.QueryRow(ctx, query, slug).Scan(
		&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
		&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
		&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
		&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
		&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
	)

	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	return &b, nil
}

func (r *postgresBusinessRepository) Update(ctx context.Context, b *domain.Business) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE businesses
		SET
			name = $1,
			slug = $2,
			category = $3,
			cuisine = $4,
			description = $5,
			short_description = $6,
			location_name = $7,
			neighborhood = $8,
			address_line = $9,
			latitude = $10,
			longitude = $11,
			phone = $12,
			website = $13,
			price_range = $14,
			tags = $15,
			payment_methods = $16,
			reservation_available = $17,
			is_featured = $18,
			updated_at = NOW()
		WHERE id = $19 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		b.Name,
		b.Slug,
		b.Category,
		b.Cuisine,
		b.Description,
		b.ShortDescription,
		b.LocationName,
		b.Neighborhood,
		b.AddressLine,
		b.Latitude,
		b.Longitude,
		b.Phone,
		b.Website,
		b.PriceRange,
		b.Tags,
		b.PaymentMethods,
		b.ReservationAvailable,
		b.IsFeatured,
		b.ID,
	).Scan(&b.UpdatedAt)

	return MapPostgresError(err, "Business")
}

func (r *postgresBusinessRepository) SetPublicationStatus(ctx context.Context, id domain.BusinessID, status domain.PublicationStatus) error {
	conn := GetDB(ctx, r.pool.Pool)
	query := `UPDATE businesses SET publication_status = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL;`
	tag, err := conn.Exec(ctx, query, status, id)
	if err != nil {
		return MapPostgresError(err, "Business")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresBusinessRepository) SetVerificationStatus(ctx context.Context, id domain.BusinessID, status domain.VerificationStatus) error {
	conn := GetDB(ctx, r.pool.Pool)
	query := `UPDATE businesses SET verification_status = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL;`
	tag, err := conn.Exec(ctx, query, status, id)
	if err != nil {
		return MapPostgresError(err, "Business")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresBusinessRepository) SetClaimStatus(ctx context.Context, id domain.BusinessID, status domain.ClaimStatus) error {
	conn := GetDB(ctx, r.pool.Pool)
	query := `UPDATE businesses SET claim_status = $1, updated_at = NOW() WHERE id = $2 AND deleted_at IS NULL;`
	tag, err := conn.Exec(ctx, query, status, id)
	if err != nil {
		return MapPostgresError(err, "Business")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresBusinessRepository) List(ctx context.Context, filter domain.BusinessFilter) ([]domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	var sb strings.Builder
	sb.WriteString(`
		SELECT
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at, deleted_at
		FROM businesses
		WHERE 1=1
	`)

	var args []any
	argIdx := 1

	if !filter.IncludeDeleted {
		sb.WriteString(" AND deleted_at IS NULL")
	}
	if filter.Category != nil {
		sb.WriteString(fmt.Sprintf(" AND category = $%d", argIdx))
		args = append(args, *filter.Category)
		argIdx++
	}
	if filter.Neighborhood != nil {
		sb.WriteString(fmt.Sprintf(" AND neighborhood = $%d", argIdx))
		args = append(args, *filter.Neighborhood)
		argIdx++
	}
	if filter.PublicationStatus != nil {
		sb.WriteString(fmt.Sprintf(" AND publication_status = $%d", argIdx))
		args = append(args, *filter.PublicationStatus)
		argIdx++
	}
	if filter.VerificationStatus != nil {
		sb.WriteString(fmt.Sprintf(" AND verification_status = $%d", argIdx))
		args = append(args, *filter.VerificationStatus)
		argIdx++
	}
	if filter.ClaimStatus != nil {
		sb.WriteString(fmt.Sprintf(" AND claim_status = $%d", argIdx))
		args = append(args, *filter.ClaimStatus)
		argIdx++
	}
	if filter.CreationSource != nil {
		sb.WriteString(fmt.Sprintf(" AND creation_source = $%d", argIdx))
		args = append(args, *filter.CreationSource)
		argIdx++
	}

	sb.WriteString(" ORDER BY is_featured DESC, name ASC")

	if filter.Limit > 0 {
		sb.WriteString(fmt.Sprintf(" LIMIT $%d", argIdx))
		args = append(args, filter.Limit)
		argIdx++
	}
	if filter.Offset > 0 {
		sb.WriteString(fmt.Sprintf(" OFFSET $%d", argIdx))
		args = append(args, filter.Offset)
	}

	rows, err := conn.Query(ctx, sb.String(), args...)
	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	defer rows.Close()

	var result []domain.Business
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
			&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
			&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
			&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
			&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
		); err != nil {
			return nil, MapPostgresError(err, "Business")
		}
		result = append(result, b)
	}

	return result, nil
}

func (r *postgresBusinessRepository) Search(ctx context.Context, query string, limit, offset int) ([]domain.SearchResult, error) {
	conn := GetDB(ctx, r.pool.Pool)

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return nil, nil
	}

	// Prepare search query supporting both tsvector prefix matching and trigram similarity
	sql := `
		SELECT
			id, name, slug, category, cuisine, description, short_description,
			location_name, neighborhood, address_line, latitude, longitude,
			phone, website, price_range, tags, payment_methods, reservation_available,
			publication_status, verification_status, claim_status, creation_source, is_featured,
			created_by_user_id, created_at, updated_at, deleted_at,
			(
				ts_rank(search_vector, plainto_tsquery('simple', $1)) * 2.0 +
				similarity(name, $1) * 3.0 +
				similarity(neighborhood, $1) * 1.5 +
				(CASE WHEN EXISTS (
					SELECT 1 FROM menu_items mi
					LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
					LEFT JOIN dish_aliases da ON da.canonical_dish_id = cd.id
					WHERE mi.business_id = businesses.id
					  AND mi.deleted_at IS NULL
					  AND (
						  LOWER(mi.name) = LOWER($1)
						  OR LOWER(mi.name) LIKE '%' || LOWER($1) || '%'
						  OR mi.name % $1
						  OR LOWER(cd.name) = LOWER($1)
						  OR LOWER(cd.slug) = LOWER($1)
						  OR da.normalized_alias = LOWER($1)
					  )
				) THEN 3.0 ELSE 0.0 END)
			) AS rank
		FROM businesses
		WHERE
			deleted_at IS NULL
			AND publication_status = 'PUBLISHED'
			AND (
				search_vector @@ plainto_tsquery('simple', $1)
				OR name % $1
				OR neighborhood % $1
				OR EXISTS (
					SELECT 1 FROM menu_items mi
					LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
					LEFT JOIN dish_aliases da ON da.canonical_dish_id = cd.id
					WHERE mi.business_id = businesses.id
					  AND mi.deleted_at IS NULL
					  AND (
						  LOWER(mi.name) = LOWER($1)
						  OR LOWER(mi.name) LIKE '%' || LOWER($1) || '%'
						  OR mi.name % $1
						  OR LOWER(cd.name) = LOWER($1)
						  OR LOWER(cd.slug) = LOWER($1)
						  OR da.normalized_alias = LOWER($1)
					  )
				)
			)
		ORDER BY rank DESC, is_featured DESC, name ASC
		LIMIT $2 OFFSET $3;
	`

	if limit <= 0 {
		limit = 20
	}

	rows, err := conn.Query(ctx, sql, trimmed, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	defer rows.Close()

	var results []domain.SearchResult
	for rows.Next() {
		var res domain.SearchResult
		b := &res.Business
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
			&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
			&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
			&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
			&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
			&res.Rank,
		); err != nil {
			return nil, MapPostgresError(err, "Business")
		}
		results = append(results, res)
	}

	return results, nil
}

func (r *postgresBusinessRepository) GetHours(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessHours, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, business_id, day_of_week, opens_at::text, closes_at::text, is_closed
		FROM business_hours
		WHERE business_id = $1
		ORDER BY day_of_week ASC;
	`

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessHours")
	}
	defer rows.Close()

	var hours []domain.BusinessHours
	for rows.Next() {
		var h domain.BusinessHours
		if err := rows.Scan(&h.ID, &h.BusinessID, &h.DayOfWeek, &h.OpensAt, &h.ClosesAt, &h.IsClosed); err != nil {
			return nil, MapPostgresError(err, "BusinessHours")
		}
		hours = append(hours, h)
	}
	return hours, nil
}

func (r *postgresBusinessRepository) SetHours(ctx context.Context, businessID domain.BusinessID, hours []domain.BusinessHours) error {
	conn := GetDB(ctx, r.pool.Pool)

	batch := &pgx.Batch{}
	// Upsert hours for each day of week
	for _, h := range hours {
		batch.Queue(`
			INSERT INTO business_hours (business_id, day_of_week, opens_at, closes_at, is_closed)
			VALUES ($1, $2, $3, $4, $5)
			ON CONFLICT (business_id, day_of_week) DO UPDATE
			SET opens_at = EXCLUDED.opens_at,
			    closes_at = EXCLUDED.closes_at,
			    is_closed = EXCLUDED.is_closed;
		`, businessID, h.DayOfWeek, h.OpensAt, h.ClosesAt, h.IsClosed)
	}

	br := conn.(interface {
		SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
	}).SendBatch(ctx, batch)
	defer br.Close()

	for i := 0; i < len(hours); i++ {
		if _, err := br.Exec(); err != nil {
			return MapPostgresError(err, "BusinessHours")
		}
	}
	return nil
}

func (r *postgresBusinessRepository) GetImages(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessImage, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, business_id, image_url, alt_text, sort_order, created_at
		FROM business_images
		WHERE business_id = $1
		ORDER BY sort_order ASC, created_at ASC;
	`

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessImage")
	}
	defer rows.Close()

	var images []domain.BusinessImage
	for rows.Next() {
		var img domain.BusinessImage
		if err := rows.Scan(&img.ID, &img.BusinessID, &img.ImageURL, &img.AltText, &img.SortOrder, &img.CreatedAt); err != nil {
			return nil, MapPostgresError(err, "BusinessImage")
		}
		images = append(images, img)
	}
	return images, nil
}

func (r *postgresBusinessRepository) SetImages(ctx context.Context, businessID domain.BusinessID, images []domain.BusinessImage) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Replace images in transaction
	if _, err := conn.Exec(ctx, "DELETE FROM business_images WHERE business_id = $1", businessID); err != nil {
		return MapPostgresError(err, "BusinessImage")
	}

	for idx, img := range images {
		query := `
			INSERT INTO business_images (business_id, image_url, alt_text, sort_order)
			VALUES ($1, $2, $3, $4);
		`
		order := img.SortOrder
		if order == 0 {
			order = idx
		}
		if _, err := conn.Exec(ctx, query, businessID, img.ImageURL, img.AltText, order); err != nil {
			return MapPostgresError(err, "BusinessImage")
		}
	}
	return nil
}

func (r *postgresBusinessRepository) DeleteImage(ctx context.Context, businessID domain.BusinessID, imageID string) error {
	conn := GetDB(ctx, r.pool.Pool)
	query := `DELETE FROM business_images WHERE business_id = $1 AND id = $2;`
	tag, err := conn.Exec(ctx, query, businessID, imageID)
	if err != nil {
		return MapPostgresError(err, "BusinessImage")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresBusinessRepository) SoftDelete(ctx context.Context, id domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)
	query := `UPDATE businesses SET deleted_at = NOW(), publication_status = 'UNPUBLISHED' WHERE id = $1 AND deleted_at IS NULL;`
	tag, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "Business")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresBusinessRepository) Delete(ctx context.Context, id domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Clean up tables with ON DELETE RESTRICT foreign keys
	if _, err := conn.Exec(ctx, `DELETE FROM business_claims WHERE business_id = $1;`, id); err != nil {
		return MapPostgresError(err, "Business")
	}
	if _, err := conn.Exec(ctx, `DELETE FROM reservations WHERE business_id = $1;`, id); err != nil {
		return MapPostgresError(err, "Business")
	}

	// Delete business - all other associated records cascade delete via PostgreSQL FOREIGN KEY ON DELETE CASCADE
	query := `DELETE FROM businesses WHERE id = $1;`
	tag, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "Business")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

