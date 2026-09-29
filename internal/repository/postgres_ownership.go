package repository

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresOwnershipRepository struct {
	pool *db.Pool
}

func NewOwnershipRepository(pool *db.Pool) OwnershipRepository {
	return &postgresOwnershipRepository{pool: pool}
}

func (r *postgresOwnershipRepository) AddManager(ctx context.Context, manager *domain.BusinessManager) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO business_managers (
			id, business_id, user_id, manager_role, granted_by, created_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5, NOW()
		)
		RETURNING id, created_at;
	`

	err := conn.QueryRow(ctx, query,
		manager.ID,
		manager.BusinessID,
		manager.UserID,
		manager.ManagerRole,
		manager.GrantedBy,
	).Scan(&manager.ID, &manager.CreatedAt)

	return MapPostgresError(err, "BusinessManager")
}

func (r *postgresOwnershipRepository) RemoveManager(ctx context.Context, businessID domain.BusinessID, userID domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `DELETE FROM business_managers WHERE business_id = $1 AND user_id = $2;`
	tag, err := conn.Exec(ctx, query, businessID, userID)
	if err != nil {
		return MapPostgresError(err, "BusinessManager")
	}
	if tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (r *postgresOwnershipRepository) IsManager(ctx context.Context, businessID domain.BusinessID, userID domain.UserID) (bool, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `SELECT EXISTS(SELECT 1 FROM business_managers WHERE business_id = $1 AND user_id = $2);`
	var exists bool
	err := conn.QueryRow(ctx, query, businessID, userID).Scan(&exists)
	if err != nil {
		return false, MapPostgresError(err, "BusinessManager")
	}
	return exists, nil
}

func (r *postgresOwnershipRepository) GetManagers(ctx context.Context, businessID domain.BusinessID) ([]domain.BusinessManager, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT id, business_id, user_id, manager_role, granted_by, created_at
		FROM business_managers
		WHERE business_id = $1
		ORDER BY created_at ASC;
	`

	rows, err := conn.Query(ctx, query, businessID)
	if err != nil {
		return nil, MapPostgresError(err, "BusinessManager")
	}
	defer rows.Close()

	var managers []domain.BusinessManager
	for rows.Next() {
		var m domain.BusinessManager
		if err := rows.Scan(&m.ID, &m.BusinessID, &m.UserID, &m.ManagerRole, &m.GrantedBy, &m.CreatedAt); err != nil {
			return nil, MapPostgresError(err, "BusinessManager")
		}
		managers = append(managers, m)
	}
	return managers, nil
}

func (r *postgresOwnershipRepository) GetManagedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT
			b.id, b.name, b.slug, b.category, b.cuisine, b.description, b.short_description,
			b.location_name, b.neighborhood, b.address_line, b.latitude, b.longitude,
			b.phone, b.website, b.price_range, b.tags, b.payment_methods, b.reservation_available,
			b.publication_status, b.verification_status, b.claim_status, b.is_featured,
			b.created_by_user_id, b.created_at, b.updated_at, b.deleted_at
		FROM businesses b
		INNER JOIN business_managers bm ON bm.business_id = b.id
		WHERE bm.user_id = $1 AND b.deleted_at IS NULL
		ORDER BY b.name ASC;
	`

	rows, err := conn.Query(ctx, query, userID)
	if err != nil {
		return nil, MapPostgresError(err, "Business")
	}
	defer rows.Close()

	var businesses []domain.Business
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(
			&b.ID, &b.Name, &b.Slug, &b.Category, &b.Cuisine, &b.Description, &b.ShortDescription,
			&b.LocationName, &b.Neighborhood, &b.AddressLine, &b.Latitude, &b.Longitude,
			&b.Phone, &b.Website, &b.PriceRange, &b.Tags, &b.PaymentMethods, &b.ReservationAvailable,
			&b.PublicationStatus, &b.VerificationStatus, &b.ClaimStatus, &b.IsFeatured,
			&b.CreatedByUserID, &b.CreatedAt, &b.UpdatedAt, &b.DeletedAt,
		); err != nil {
			return nil, MapPostgresError(err, "Business")
		}
		businesses = append(businesses, b)
	}
	return businesses, nil
}
