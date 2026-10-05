package repository

import (
	"context"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresMenuRepository struct {
	pool *db.Pool
}

func NewMenuRepository(pool *db.Pool) MenuRepository {
	return &postgresMenuRepository{pool: pool}
}

func (r *postgresMenuRepository) CreateCategory(ctx context.Context, cat *domain.MenuCategory) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO menu_categories (
			id, business_id, name, sort_order, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		cat.ID,
		cat.BusinessID,
		cat.Name,
		cat.SortOrder,
	).Scan(&cat.ID, &cat.CreatedAt, &cat.UpdatedAt)

	return MapPostgresError(err, "MenuCategory")
}

func (r *postgresMenuRepository) GetCategoryByID(ctx context.Context, id domain.CategoryID) (*domain.MenuCategory, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, name, sort_order, created_at, updated_at, deleted_at
		FROM menu_categories
		WHERE id = $1 AND deleted_at IS NULL;
	`

	var cat domain.MenuCategory
	err := conn.QueryRow(ctx, query, id).Scan(
		&cat.ID,
		&cat.BusinessID,
		&cat.Name,
		&cat.SortOrder,
		&cat.CreatedAt,
		&cat.UpdatedAt,
		&cat.DeletedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "MenuCategory")
	}

	return &cat, nil
}

func (r *postgresMenuRepository) UpdateCategory(ctx context.Context, cat *domain.MenuCategory) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE menu_categories
		SET name = $1, sort_order = $2, updated_at = NOW()
		WHERE id = $3 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		cat.Name,
		cat.SortOrder,
		cat.ID,
	).Scan(&cat.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrNotFound
		}
		return MapPostgresError(err, "MenuCategory")
	}

	return nil
}

func (r *postgresMenuRepository) DeleteCategory(ctx context.Context, id domain.CategoryID) error {
	conn := GetDB(ctx, r.pool.Pool)

	// Soft delete category and its associated items atomically
	queryItems := `UPDATE menu_items SET deleted_at = NOW() WHERE category_id = $1 AND deleted_at IS NULL;`
	if _, err := conn.Exec(ctx, queryItems, id); err != nil {
		return MapPostgresError(err, "MenuItem")
	}

	queryCat := `
		UPDATE menu_categories
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, queryCat, id)
	if err != nil {
		return MapPostgresError(err, "MenuCategory")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresMenuRepository) ReorderCategories(ctx context.Context, businessID domain.BusinessID, categoryIDs []domain.CategoryID) error {
	conn := GetDB(ctx, r.pool.Pool)

	for order, id := range categoryIDs {
		query := `
			UPDATE menu_categories
			SET sort_order = $1, updated_at = NOW()
			WHERE id = $2 AND business_id = $3 AND deleted_at IS NULL;
		`
		if _, err := conn.Exec(ctx, query, order, id, businessID); err != nil {
			return MapPostgresError(err, "MenuCategory")
		}
	}

	return nil
}

func (r *postgresMenuRepository) ListCategories(ctx context.Context, businessID domain.BusinessID) ([]domain.MenuCategory, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			id, business_id, name, sort_order, created_at, updated_at, deleted_at
		FROM menu_categories
		WHERE business_id = $1 AND deleted_at IS NULL
		ORDER BY sort_order ASC, name ASC;
	`

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "MenuCategory")
	}
	defer rows.Close()

	var categories []domain.MenuCategory
	for rows.Next() {
		var cat domain.MenuCategory
		if err := rows.Scan(
			&cat.ID,
			&cat.BusinessID,
			&cat.Name,
			&cat.SortOrder,
			&cat.CreatedAt,
			&cat.UpdatedAt,
			&cat.DeletedAt,
		); err != nil {
			return nil, err
		}
		categories = append(categories, cat)
	}

	return categories, nil
}

func (r *postgresMenuRepository) CreateItem(ctx context.Context, item *domain.MenuItem) error {
	conn := GetDB(ctx, r.pool.Pool)

	var imgID *string
	if item.ImageMediaID != nil {
		s := string(*item.ImageMediaID)
		imgID = &s
	}

	var canonID *string
	if item.CanonicalDishID != nil && *item.CanonicalDishID != "" {
		s := string(*item.CanonicalDishID)
		canonID = &s
	}

	query := `
		INSERT INTO menu_items (
			id, category_id, business_id, name, description,
			price_etb_minor, sort_order, is_available, image_media_id, canonical_dish_id, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8, $9, $10, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		item.ID,
		item.CategoryID,
		item.BusinessID,
		item.Name,
		item.Description,
		item.PriceETBMinor,
		item.SortOrder,
		item.IsAvailable,
		imgID,
		canonID,
	).Scan(&item.ID, &item.CreatedAt, &item.UpdatedAt)

	return MapPostgresError(err, "MenuItem")
}

func (r *postgresMenuRepository) GetItemByID(ctx context.Context, id domain.ItemID) (*domain.MenuItem, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			mi.id, mi.category_id, mi.business_id, mi.name, mi.description,
			mi.price_etb_minor, mi.sort_order, mi.is_available, mi.image_media_id,
			mi.canonical_dish_id, mi.created_at, mi.updated_at, mi.deleted_at,
			cd.id, cd.name, cd.slug, cd.description, cd.cuisine
		FROM menu_items mi
		LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
		WHERE mi.id = $1 AND mi.deleted_at IS NULL;
	`

	var item domain.MenuItem
	var imgID *string
	var canonID *string
	var cdID *string
	var cdName *string
	var cdSlug *string
	var cdDesc *string
	var cdCuisine *string

	err := conn.QueryRow(ctx, query, id).Scan(
		&item.ID,
		&item.CategoryID,
		&item.BusinessID,
		&item.Name,
		&item.Description,
		&item.PriceETBMinor,
		&item.SortOrder,
		&item.IsAvailable,
		&imgID,
		&canonID,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.DeletedAt,
		&cdID,
		&cdName,
		&cdSlug,
		&cdDesc,
		&cdCuisine,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "MenuItem")
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
		item.CanonicalDish = &domain.CanonicalDish{
			ID:   dID,
			Name: *cdName,
		}
		if cdSlug != nil {
			item.CanonicalDish.Slug = *cdSlug
		}
		if cdDesc != nil {
			item.CanonicalDish.Description = *cdDesc
		}
		if cdCuisine != nil {
			item.CanonicalDish.Cuisine = *cdCuisine
		}
	}

	return &item, nil
}

func (r *postgresMenuRepository) UpdateItem(ctx context.Context, item *domain.MenuItem) error {
	conn := GetDB(ctx, r.pool.Pool)

	var imgID *string
	if item.ImageMediaID != nil {
		s := string(*item.ImageMediaID)
		imgID = &s
	}

	var canonID *string
	if item.CanonicalDishID != nil && *item.CanonicalDishID != "" {
		s := string(*item.CanonicalDishID)
		canonID = &s
	}

	query := `
		UPDATE menu_items
		SET
			category_id = $1,
			name = $2,
			description = $3,
			price_etb_minor = $4,
			sort_order = $5,
			is_available = $6,
			image_media_id = $7,
			canonical_dish_id = $8,
			updated_at = NOW()
		WHERE id = $9 AND deleted_at IS NULL
		RETURNING updated_at;
	`

	err := conn.QueryRow(ctx, query,
		item.CategoryID,
		item.Name,
		item.Description,
		item.PriceETBMinor,
		item.SortOrder,
		item.IsAvailable,
		imgID,
		canonID,
		item.ID,
	).Scan(&item.UpdatedAt)
	if err != nil {
		if err == pgx.ErrNoRows {
			return domain.ErrNotFound
		}
		return MapPostgresError(err, "MenuItem")
	}

	return nil
}

func (r *postgresMenuRepository) DeleteItem(ctx context.Context, id domain.ItemID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE menu_items
		SET deleted_at = NOW()
		WHERE id = $1 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "MenuItem")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresMenuRepository) ReorderItems(ctx context.Context, categoryID domain.CategoryID, itemIDs []domain.ItemID) error {
	conn := GetDB(ctx, r.pool.Pool)

	for order, id := range itemIDs {
		query := `
			UPDATE menu_items
			SET sort_order = $1, updated_at = NOW()
			WHERE id = $2 AND category_id = $3 AND deleted_at IS NULL;
		`
		if _, err := conn.Exec(ctx, query, order, id, categoryID); err != nil {
			return MapPostgresError(err, "MenuItem")
		}
	}

	return nil
}

func (r *postgresMenuRepository) ListItemsByCategory(ctx context.Context, categoryID domain.CategoryID) ([]domain.MenuItem, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			mi.id, mi.category_id, mi.business_id, mi.name, mi.description,
			mi.price_etb_minor, mi.sort_order, mi.is_available, mi.image_media_id,
			mi.canonical_dish_id, mi.created_at, mi.updated_at, mi.deleted_at,
			cd.id, cd.name, cd.slug, cd.description, cd.cuisine
		FROM menu_items mi
		LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
		WHERE mi.category_id = $1 AND mi.deleted_at IS NULL
		ORDER BY mi.sort_order ASC, mi.name ASC;
	`

	rows, err := conn.Query(ctx, query, categoryID)
	if err != nil {
		return nil, MapPostgresError(err, "MenuItem")
	}
	defer rows.Close()

	var items []domain.MenuItem
	for rows.Next() {
		var item domain.MenuItem
		var imgID *string
		var canonID *string
		var cdID *string
		var cdName *string
		var cdSlug *string
		var cdDesc *string
		var cdCuisine *string

		if err := rows.Scan(
			&item.ID,
			&item.CategoryID,
			&item.BusinessID,
			&item.Name,
			&item.Description,
			&item.PriceETBMinor,
			&item.SortOrder,
			&item.IsAvailable,
			&imgID,
			&canonID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.DeletedAt,
			&cdID,
			&cdName,
			&cdSlug,
			&cdDesc,
			&cdCuisine,
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
			item.CanonicalDish = &domain.CanonicalDish{
				ID:   dID,
				Name: *cdName,
			}
			if cdSlug != nil {
				item.CanonicalDish.Slug = *cdSlug
			}
			if cdDesc != nil {
				item.CanonicalDish.Description = *cdDesc
			}
			if cdCuisine != nil {
				item.CanonicalDish.Cuisine = *cdCuisine
			}
		}
		items = append(items, item)
	}

	return items, nil
}

func (r *postgresMenuRepository) GetFullMenu(ctx context.Context, businessID domain.BusinessID) (*domain.BusinessMenu, error) {
	conn := GetDB(ctx, r.pool.Pool)

	categories, err := r.ListCategories(ctx, businessID)
	if err != nil {
		return nil, err
	}

	// Fetch all items for this business in one query ordered by category and sort_order
	query := `
		SELECT
			mi.id, mi.category_id, mi.business_id, mi.name, mi.description,
			mi.price_etb_minor, mi.sort_order, mi.is_available, mi.image_media_id,
			mi.canonical_dish_id, mi.created_at, mi.updated_at, mi.deleted_at,
			cd.id, cd.name, cd.slug, cd.description, cd.cuisine
		FROM menu_items mi
		LEFT JOIN canonical_dishes cd ON cd.id = mi.canonical_dish_id AND cd.deleted_at IS NULL
		WHERE mi.business_id = $1 AND mi.deleted_at IS NULL
		ORDER BY mi.sort_order ASC, mi.name ASC;
	`

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "MenuItem")
	}
	defer rows.Close()

	itemsByCategory := make(map[domain.CategoryID][]domain.MenuItem)
	for rows.Next() {
		var item domain.MenuItem
		var imgID *string
		var canonID *string
		var cdID *string
		var cdName *string
		var cdSlug *string
		var cdDesc *string
		var cdCuisine *string

		if err := rows.Scan(
			&item.ID,
			&item.CategoryID,
			&item.BusinessID,
			&item.Name,
			&item.Description,
			&item.PriceETBMinor,
			&item.SortOrder,
			&item.IsAvailable,
			&imgID,
			&canonID,
			&item.CreatedAt,
			&item.UpdatedAt,
			&item.DeletedAt,
			&cdID,
			&cdName,
			&cdSlug,
			&cdDesc,
			&cdCuisine,
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
			item.CanonicalDish = &domain.CanonicalDish{
				ID:   dID,
				Name: *cdName,
			}
			if cdSlug != nil {
				item.CanonicalDish.Slug = *cdSlug
			}
			if cdDesc != nil {
				item.CanonicalDish.Description = *cdDesc
			}
			if cdCuisine != nil {
				item.CanonicalDish.Cuisine = *cdCuisine
			}
		}
		itemsByCategory[item.CategoryID] = append(itemsByCategory[item.CategoryID], item)
	}

	result := &domain.BusinessMenu{
		BusinessID: businessID,
		Categories: make([]domain.CategoryWithItems, 0, len(categories)),
	}

	for _, cat := range categories {
		cItems := itemsByCategory[cat.ID]
		if cItems == nil {
			cItems = []domain.MenuItem{}
		}
		result.Categories = append(result.Categories, domain.CategoryWithItems{
			Category: cat,
			Items:    cItems,
		})
	}

	return result, nil
}

func (r *postgresMenuRepository) MapCanonicalDish(ctx context.Context, itemID domain.ItemID, canonicalDishID *domain.DishID) error {
	conn := GetDB(ctx, r.pool.Pool)

	var cID *string
	if canonicalDishID != nil && *canonicalDishID != "" {
		s := string(*canonicalDishID)
		cID = &s
	}

	query := `
		UPDATE menu_items
		SET canonical_dish_id = $1, updated_at = NOW()
		WHERE id = $2 AND deleted_at IS NULL;
	`

	cmd, err := conn.Exec(ctx, query, cID, itemID)
	if err != nil {
		return MapPostgresError(err, "MenuItem")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
