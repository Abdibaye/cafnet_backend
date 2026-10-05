package repository

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresDishRepository struct {
	pool *db.Pool
}

// NewDishRepository constructs a postgres-backed DishRepository.
func NewDishRepository(pool *db.Pool) DishRepository {
	return &postgresDishRepository{pool: pool}
}

func (r *postgresDishRepository) Create(ctx context.Context, dish *domain.CanonicalDish) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO canonical_dishes (
			id, name, slug, description, cuisine, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, NULLIF($5, ''), NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		dish.ID,
		dish.Name,
		dish.Slug,
		dish.Description,
		dish.Cuisine,
	).Scan(&dish.ID, &dish.CreatedAt, &dish.UpdatedAt)

	return MapPostgresError(err, "CanonicalDish")
}

func (r *postgresDishRepository) GetByID(ctx context.Context, id domain.DishID) (*domain.CanonicalDish, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, name, slug, description, COALESCE(cuisine, ''), created_at, updated_at, deleted_at
		FROM canonical_dishes
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var dish domain.CanonicalDish
	err := conn.QueryRow(ctx, query, id).Scan(
		&dish.ID,
		&dish.Name,
		&dish.Slug,
		&dish.Description,
		&dish.Cuisine,
		&dish.CreatedAt,
		&dish.UpdatedAt,
		&dish.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "CanonicalDish")
	}

	aliases, err := r.ListAliases(ctx, dish.ID)
	if err == nil {
		dish.Aliases = aliases
	}

	return &dish, nil
}

func (r *postgresDishRepository) GetBySlug(ctx context.Context, slug string) (*domain.CanonicalDish, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, name, slug, description, COALESCE(cuisine, ''), created_at, updated_at, deleted_at
		FROM canonical_dishes
		WHERE slug = $1 AND deleted_at IS NULL;
	`

	var dish domain.CanonicalDish
	err := conn.QueryRow(ctx, query, slug).Scan(
		&dish.ID,
		&dish.Name,
		&dish.Slug,
		&dish.Description,
		&dish.Cuisine,
		&dish.CreatedAt,
		&dish.UpdatedAt,
		&dish.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "CanonicalDish")
	}

	aliases, err := r.ListAliases(ctx, dish.ID)
	if err == nil {
		dish.Aliases = aliases
	}

	return &dish, nil
}

func (r *postgresDishRepository) Update(ctx context.Context, dish *domain.CanonicalDish) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE canonical_dishes
		SET
			name = $1,
			slug = $2,
			description = $3,
			cuisine = NULLIF($4, ''),
			updated_at = NOW()
		WHERE id = $5 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		dish.Name,
		dish.Slug,
		dish.Description,
		dish.Cuisine,
		dish.ID,
	).Scan(&dish.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrNotFound
		}
		return MapPostgresError(err, "CanonicalDish")
	}

	return nil
}

func (r *postgresDishRepository) Delete(ctx context.Context, id domain.DishID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM canonical_dishes
		WHERE id = $1;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "CanonicalDish")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresDishRepository) List(ctx context.Context, query string, limit, offset int) ([]domain.CanonicalDish, error) {
	conn := GetDB(ctx, r.pool.Pool)

	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	var sql string
	var args []interface{}

	trimmed := strings.TrimSpace(query)
	if trimmed != "" {
		sql = `
			SELECT
				id, name, slug, description, COALESCE(cuisine, ''), created_at, updated_at, deleted_at
			FROM canonical_dishes
			WHERE deleted_at IS NULL
				AND (name ILIKE '%' || $1 || '%' OR COALESCE(cuisine, '') ILIKE '%' || $1 || '%' OR slug ILIKE '%' || $1 || '%')
			ORDER BY name ASC
			LIMIT $2 OFFSET $3;
		`
		args = []interface{}{trimmed, limit, offset}
	} else {
		sql = `
			SELECT
				id, name, slug, description, COALESCE(cuisine, ''), created_at, updated_at, deleted_at
			FROM canonical_dishes
			WHERE deleted_at IS NULL
			ORDER BY name ASC
			LIMIT $1 OFFSET $2;
		`
		args = []interface{}{limit, offset}
	}

	rows, err := conn.Query(ctx, sql, args...)
	if err != nil {
		return nil, MapPostgresError(err, "CanonicalDish")
	}
	defer rows.Close()

	var dishes []domain.CanonicalDish
	for rows.Next() {
		var dish domain.CanonicalDish
		if err := rows.Scan(
			&dish.ID,
			&dish.Name,
			&dish.Slug,
			&dish.Description,
			&dish.Cuisine,
			&dish.CreatedAt,
			&dish.UpdatedAt,
			&dish.DeletedAt,
		); err != nil {
			return nil, err
		}
		dishes = append(dishes, dish)
	}

	return dishes, nil
}

func (r *postgresDishRepository) AddAlias(ctx context.Context, alias *domain.DishAlias) error {
	conn := GetDB(ctx, r.pool.Pool)

	norm := domain.NormalizeAlias(alias.Alias)
	if norm == "" {
		return domain.ErrInvalidInput
	}
	alias.NormalizedAlias = norm

	query := `
		INSERT INTO dish_aliases (
			id, canonical_dish_id, alias, normalized_alias, language, created_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, NULLIF($5, ''), NOW()
		)
		RETURNING id, created_at;
	`

	var lang *string
	if alias.Language != nil && *alias.Language != "" {
		lang = alias.Language
	}

	err := conn.QueryRow(ctx, query,
		alias.ID,
		alias.CanonicalDishID,
		alias.Alias,
		alias.NormalizedAlias,
		lang,
	).Scan(&alias.ID, &alias.CreatedAt)

	return MapPostgresError(err, "DishAlias")
}

func (r *postgresDishRepository) GetAliasByID(ctx context.Context, id domain.AliasID) (*domain.DishAlias, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, canonical_dish_id, alias, normalized_alias, language, created_at
		FROM dish_aliases
		WHERE id = $1;
	`

	var a domain.DishAlias
	err := conn.QueryRow(ctx, query, id).Scan(
		&a.ID,
		&a.CanonicalDishID,
		&a.Alias,
		&a.NormalizedAlias,
		&a.Language,
		&a.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "DishAlias")
	}

	return &a, nil
}

func (r *postgresDishRepository) DeleteAlias(ctx context.Context, id domain.AliasID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM dish_aliases
		WHERE id = $1;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "DishAlias")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresDishRepository) ListAliases(ctx context.Context, dishID domain.DishID) ([]domain.DishAlias, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, canonical_dish_id, alias, normalized_alias, language, created_at
		FROM dish_aliases
		WHERE canonical_dish_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := conn.Query(ctx, query, dishID)
	if err != nil {
		return nil, MapPostgresError(err, "DishAlias")
	}
	defer rows.Close()

	var aliases []domain.DishAlias
	for rows.Next() {
		var a domain.DishAlias
		if err := rows.Scan(
			&a.ID,
			&a.CanonicalDishID,
			&a.Alias,
			&a.NormalizedAlias,
			&a.Language,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		aliases = append(aliases, a)
	}

	return aliases, nil
}

func (r *postgresDishRepository) FindCanonicalDishByQuery(ctx context.Context, query string) (*domain.CanonicalDish, error) {
	conn := GetDB(ctx, r.pool.Pool)

	norm := domain.NormalizeAlias(query)
	if norm == "" {
		return nil, nil
	}

	sql := `
		SELECT
			cd.id, cd.name, cd.slug, cd.description, COALESCE(cd.cuisine, ''), cd.created_at, cd.updated_at, cd.deleted_at
		FROM canonical_dishes cd
		LEFT JOIN dish_aliases da ON da.canonical_dish_id = cd.id
		WHERE cd.deleted_at IS NULL
		  AND (
		      LOWER(cd.name) = $1
		      OR LOWER(cd.slug) = $1
		      OR da.normalized_alias = $1
		      OR LOWER(cd.name) LIKE '%' || $1 || '%'
		      OR da.normalized_alias LIKE '%' || $1 || '%'
		  )
		ORDER BY
			CASE
				WHEN LOWER(cd.name) = $1 THEN 1
				WHEN LOWER(cd.slug) = $1 THEN 2
				WHEN da.normalized_alias = $1 THEN 3
				WHEN LOWER(cd.name) LIKE '%' || $1 || '%' THEN 4
				ELSE 5
			END ASC
		LIMIT 1;
	`

	var dish domain.CanonicalDish
	err := conn.QueryRow(ctx, sql, norm).Scan(
		&dish.ID,
		&dish.Name,
		&dish.Slug,
		&dish.Description,
		&dish.Cuisine,
		&dish.CreatedAt,
		&dish.UpdatedAt,
		&dish.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, MapPostgresError(err, "CanonicalDish")
	}

	aliases, err := r.ListAliases(ctx, dish.ID)
	if err == nil {
		dish.Aliases = aliases
	}

	return &dish, nil
}

func (r *postgresDishRepository) SearchDishes(ctx context.Context, query string, limit, offset int) ([]domain.FoodSearchResult, error) {
	conn := GetDB(ctx, r.pool.Pool)

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []domain.FoodSearchResult{}, nil
	}

	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	sql := `
		WITH q_data AS (
			SELECT $1::text AS raw_q, LOWER(TRIM($1::text)) AS norm_q
		),
		matched_canonical AS (
			SELECT
				cd.id AS cd_id,
				cd.name AS cd_name,
				cd.slug AS cd_slug,
				cd.description AS cd_description,
				cd.cuisine AS cd_cuisine,
				CASE
					WHEN LOWER(cd.name) = q.norm_q THEN 100.0
					WHEN LOWER(cd.slug) = q.norm_q THEN 95.0
					WHEN da.normalized_alias = q.norm_q THEN 90.0
					WHEN LOWER(cd.name) LIKE '%' || q.norm_q || '%' THEN 60.0
					WHEN da.normalized_alias LIKE '%' || q.norm_q || '%' THEN 55.0
					WHEN cd.name % q.raw_q THEN 40.0 + (similarity(cd.name, q.raw_q) * 10.0)
					WHEN da.normalized_alias % q.norm_q THEN 35.0 + (similarity(da.normalized_alias, q.norm_q) * 10.0)
					ELSE 0.0
				END AS c_score
			FROM q_data q
			JOIN canonical_dishes cd ON cd.deleted_at IS NULL
			LEFT JOIN dish_aliases da ON da.canonical_dish_id = cd.id
			WHERE
				LOWER(cd.name) = q.norm_q
				OR LOWER(cd.slug) = q.norm_q
				OR da.normalized_alias = q.norm_q
				OR LOWER(cd.name) LIKE '%' || q.norm_q || '%'
				OR da.normalized_alias LIKE '%' || q.norm_q || '%'
				OR cd.name % q.raw_q
				OR da.normalized_alias % q.norm_q
		),
		best_canonical AS (
			SELECT cd_id, cd_name, cd_slug, cd_description, cd_cuisine, MAX(c_score) AS c_score
			FROM matched_canonical
			GROUP BY cd_id, cd_name, cd_slug, cd_description, cd_cuisine
		)
		SELECT
			-- Business fields
			b.id, b.name, b.slug, b.category, b.cuisine, b.description, b.short_description,
			b.location_name, b.neighborhood, b.address_line, b.latitude, b.longitude,
			b.phone, b.website, b.price_range, b.tags, b.payment_methods, b.reservation_available,
			b.publication_status, b.verification_status, b.claim_status, b.creation_source, b.is_featured,
			b.created_by_user_id, b.created_at, b.updated_at, b.deleted_at,

			-- Menu Item fields
			mi.id, mi.category_id, mi.business_id, mi.name, mi.description,
			mi.price_etb_minor, mi.sort_order, mi.is_available, mi.image_media_id,
			mi.canonical_dish_id, mi.created_at, mi.updated_at, mi.deleted_at,

			-- Canonical Dish fields
			cd.id, cd.name, cd.slug, cd.description, cd.cuisine,

			-- Rating & review count
			COALESCE(r.avg_rating, 0.0) AS rating,
			COALESCE(r.rev_count, 0) AS review_count,

			-- Rank calculation
			GREATEST(
				COALESCE(bc.c_score, 0.0),
				CASE
					WHEN LOWER(mi.name) = q.norm_q THEN 85.0
					WHEN LOWER(mi.name) LIKE '%' || q.norm_q || '%' THEN 45.0
					WHEN mi.name % q.raw_q THEN 30.0 + (similarity(mi.name, q.raw_q) * 10.0)
					WHEN LOWER(mi.description) LIKE '%' || q.norm_q || '%' THEN 20.0
					ELSE 0.0
				END
			) AS rank
		FROM q_data q
		CROSS JOIN menu_items mi
		JOIN businesses b ON b.id = mi.business_id AND b.publication_status = 'PUBLISHED' AND b.deleted_at IS NULL
		JOIN menu_categories mc ON mc.id = mi.category_id AND mc.deleted_at IS NULL
		LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
		LEFT JOIN best_canonical bc ON bc.cd_id = mi.canonical_dish_id
		LEFT JOIN (
			SELECT business_id, ROUND(AVG(rating)::numeric, 1)::float8 AS avg_rating, COUNT(*)::int AS rev_count
			FROM reviews
			WHERE status = 'PUBLISHED' AND deleted_at IS NULL
			GROUP BY business_id
		) r ON r.business_id = b.id
		WHERE
			mi.deleted_at IS NULL
			AND mi.is_available = TRUE
			AND (
				bc.cd_id IS NOT NULL
				OR LOWER(mi.name) = q.norm_q
				OR LOWER(mi.name) LIKE '%' || q.norm_q || '%'
				OR mi.name % q.raw_q
				OR LOWER(mi.description) LIKE '%' || q.norm_q || '%'
			)
		ORDER BY rank DESC, b.is_featured DESC, rating DESC, mi.price_etb_minor ASC, mi.name ASC
		LIMIT $2 OFFSET $3;
	`

	rows, err := conn.Query(ctx, sql, trimmed, limit, offset)
	if err != nil {
		return nil, MapPostgresError(err, "FoodSearch")
	}
	defer rows.Close()

	var results []domain.FoodSearchResult
	for rows.Next() {
		var res domain.FoodSearchResult
		b := &res.Business
		item := &res.MenuItem

		var imgID *string
		var canonID *string
		var cdID *string
		var cdName *string
		var cdSlug *string
		var cdDesc *string
		var cdCuisine *string

		if err := rows.Scan(
			&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
			&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
			&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
			&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.CreationSource, &b.IsFeatured,
			&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,

			&item.ID, &item.CategoryID, &item.BusinessID, &item.Name, &item.Description,
			&item.PriceETBMinor, &item.SortOrder, &item.IsAvailable, &imgID,
			&canonID, &item.CreatedAt, &item.UpdatedAt, &item.DeletedAt,

			&cdID, &cdName, &cdSlug, &cdDesc, &cdCuisine,

			&res.Rating,
			&res.ReviewCount,
			&res.Rank,
		); err != nil {
			return nil, err
		}

		if imgID != nil {
			mID := domain.MediaID(*imgID)
			item.ImageMediaID = &mID
		}
		if canonID != nil {
			dID := domain.DishID(*canonID)
			item.CanonicalDishID = &dID
		}
		if cdID != nil && cdName != nil {
			dID := domain.DishID(*cdID)
			res.CanonicalDish = &domain.CanonicalDish{
				ID:   dID,
				Name: *cdName,
			}
			if cdSlug != nil {
				res.CanonicalDish.Slug = *cdSlug
			}
			if cdDesc != nil {
				res.CanonicalDish.Description = *cdDesc
			}
			if cdCuisine != nil {
				res.CanonicalDish.Cuisine = *cdCuisine
			}
			item.CanonicalDish = res.CanonicalDish
		}

		results = append(results, res)
	}

	return results, nil
}
