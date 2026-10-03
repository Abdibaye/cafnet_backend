package repository

import (
	"context"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresCustomerListRepository struct {
	pool *db.Pool
}

func NewCustomerListRepository(pool *db.Pool) CustomerListRepository {
	return &postgresCustomerListRepository{pool: pool}
}

const customerListBusinessCols = `
	b.id, b.name, b.slug, b.category, b.cuisine, b.description, b.short_description,
	b.location_name, b.neighborhood, b.address_line, b.latitude, b.longitude,
	b.phone, b.website, b.price_range, b.tags, b.payment_methods, b.reservation_available,
	b.publication_status, b.verification_status, b.claim_status, b.creation_source, b.is_featured,
	b.created_by_user_id, b.created_at, b.updated_at, b.deleted_at
`

func (r *postgresCustomerListRepository) SaveBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO saved_businesses (user_id, business_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id, business_id) DO NOTHING;
	`

	_, err := conn.Exec(ctx, query, userID, businessID)
	return MapPostgresError(err, "SavedBusiness")
}

func (r *postgresCustomerListRepository) UnsaveBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM saved_businesses
		WHERE user_id = $1 AND business_id = $2;
	`

	_, err := conn.Exec(ctx, query, userID, businessID)
	return MapPostgresError(err, "SavedBusiness")
}

func (r *postgresCustomerListRepository) IsSaved(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) (bool, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT EXISTS (
			SELECT 1 FROM saved_businesses
			WHERE user_id = $1 AND business_id = $2
		);
	`

	var exists bool
	err := conn.QueryRow(ctx, query, userID, businessID).Scan(&exists)
	if err != nil {
		return false, MapPostgresError(err, "SavedBusiness")
	}

	return exists, nil
}

func (r *postgresCustomerListRepository) ListSavedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT ` + customerListBusinessCols + `
		FROM businesses b
		JOIN saved_businesses s ON s.business_id = b.id
		WHERE s.user_id = $1 AND b.deleted_at IS NULL
		ORDER BY s.created_at DESC;
	`

	rows, err := conn.Query(ctx, query, userID)
	if err != nil {
		return nil, MapPostgresError(err, "SavedBusiness")
	}
	defer rows.Close()

	var businesses []domain.Business
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(
			&b.ID,
			&b.Name,
			&b.Slug,
			&b.Category,
			&b.Cuisine,
			&b.Description,
			&b.ShortDescription,
			&b.LocationName,
			&b.Neighborhood,
			&b.AddressLine,
			&b.Latitude,
			&b.Longitude,
			&b.Phone,
			&b.Website,
			&b.PriceRange,
			&b.Tags,
			&b.PaymentMethods,
			&b.ReservationAvailable,
			&b.PublicationStatus,
			&b.VerificationStatus,
			&b.ClaimStatus,
			&b.CreationSource,
			&b.IsFeatured,
			&b.CreatedByUserID,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.DeletedAt,
		); err != nil {
			return nil, err
		}
		businesses = append(businesses, b)
	}

	return businesses, nil
}

func (r *postgresCustomerListRepository) FollowBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		INSERT INTO followed_businesses (user_id, business_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (user_id, business_id) DO NOTHING;
	`

	_, err := conn.Exec(ctx, query, userID, businessID)
	return MapPostgresError(err, "FollowedBusiness")
}

func (r *postgresCustomerListRepository) UnfollowBusiness(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		DELETE FROM followed_businesses
		WHERE user_id = $1 AND business_id = $2;
	`

	_, err := conn.Exec(ctx, query, userID, businessID)
	return MapPostgresError(err, "FollowedBusiness")
}

func (r *postgresCustomerListRepository) IsFollowing(ctx context.Context, userID domain.UserID, businessID domain.BusinessID) (bool, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT EXISTS (
			SELECT 1 FROM followed_businesses
			WHERE user_id = $1 AND business_id = $2
		);
	`

	var exists bool
	err := conn.QueryRow(ctx, query, userID, businessID).Scan(&exists)
	if err != nil {
		return false, MapPostgresError(err, "FollowedBusiness")
	}

	return exists, nil
}

func (r *postgresCustomerListRepository) ListFollowedBusinesses(ctx context.Context, userID domain.UserID) ([]domain.Business, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		SELECT ` + customerListBusinessCols + `
		FROM businesses b
		JOIN followed_businesses f ON f.business_id = b.id
		WHERE f.user_id = $1 AND b.deleted_at IS NULL
		ORDER BY f.created_at DESC;
	`

	rows, err := conn.Query(ctx, query, userID)
	if err != nil {
		return nil, MapPostgresError(err, "FollowedBusiness")
	}
	defer rows.Close()

	var businesses []domain.Business
	for rows.Next() {
		var b domain.Business
		if err := rows.Scan(
			&b.ID,
			&b.Name,
			&b.Slug,
			&b.Category,
			&b.Cuisine,
			&b.Description,
			&b.ShortDescription,
			&b.LocationName,
			&b.Neighborhood,
			&b.AddressLine,
			&b.Latitude,
			&b.Longitude,
			&b.Phone,
			&b.Website,
			&b.PriceRange,
			&b.Tags,
			&b.PaymentMethods,
			&b.ReservationAvailable,
			&b.PublicationStatus,
			&b.VerificationStatus,
			&b.ClaimStatus,
			&b.CreationSource,
			&b.IsFeatured,
			&b.CreatedByUserID,
			&b.CreatedAt,
			&b.UpdatedAt,
			&b.DeletedAt,
		); err != nil {
			return nil, err
		}
		businesses = append(businesses, b)
	}

	return businesses, nil
}
