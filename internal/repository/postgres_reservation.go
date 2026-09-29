package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

type postgresReservationRepository struct {
	pool *db.Pool
}

func NewReservationRepository(pool *db.Pool) ReservationRepository {
	return &postgresReservationRepository{pool: pool}
}

func (r *postgresReservationRepository) Create(ctx context.Context, res *domain.Reservation) error {
	conn := GetDB(ctx, r.pool.Pool)

	if res.Status == "" {
		res.Status = domain.ReservationStatusPending
	}

	query := `
		INSERT INTO reservations (
			id, reservation_code, business_id, customer_user_id, customer_name,
			customer_phone, reservation_date, reservation_time, party_size,
			special_request, status, created_at, updated_at
		) VALUES (
			COALESCE(NULLIF($1, '')::uuid, gen_random_uuid()), $2, $3, $4, $5,
			$6, $7, $8::time, $9,
			$10, $11, NOW(), NOW()
		)
		RETURNING id, created_at, updated_at;
	`

	err := conn.QueryRow(ctx, query,
		res.ID,
		res.ReservationCode,
		res.BusinessID,
		res.CustomerUserID,
		res.CustomerName,
		res.CustomerPhone,
		res.ReservationDate,
		res.ReservationTime,
		res.PartySize,
		res.SpecialRequest,
		res.Status,
	).Scan(&res.ID, &res.CreatedAt, &res.UpdatedAt)

	return MapPostgresError(err, "Reservation")
}

const reservationSelectCols = `
	id, reservation_code, business_id, customer_user_id, customer_name,
	customer_phone, reservation_date, reservation_time::text, party_size,
	special_request, status, decline_reason, created_at, confirmed_at,
	declined_at, completed_at, cancelled_at, reviewed_by_user, updated_at
`

func scanReservation(row pgx.Row) (*domain.Reservation, error) {
	var res domain.Reservation
	err := row.Scan(
		&res.ID,
		&res.ReservationCode,
		&res.BusinessID,
		&res.CustomerUserID,
		&res.CustomerName,
		&res.CustomerPhone,
		&res.ReservationDate,
		&res.ReservationTime,
		&res.PartySize,
		&res.SpecialRequest,
		&res.Status,
		&res.DeclineReason,
		&res.CreatedAt,
		&res.ConfirmedAt,
		&res.DeclinedAt,
		&res.CompletedAt,
		&res.CancelledAt,
		&res.ReviewedByUser,
		&res.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &res, nil
}

func (r *postgresReservationRepository) GetByID(ctx context.Context, id domain.ReservationID) (*domain.Reservation, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := fmt.Sprintf(`SELECT %s FROM reservations WHERE id = $1;`, reservationSelectCols)

	res, err := scanReservation(conn.QueryRow(ctx, query, id))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "Reservation")
	}

	return res, nil
}

func (r *postgresReservationRepository) GetByCode(ctx context.Context, code string) (*domain.Reservation, error) {
	conn := GetDB(ctx, r.pool.Pool)

	query := fmt.Sprintf(`SELECT %s FROM reservations WHERE reservation_code = $1;`, reservationSelectCols)

	res, err := scanReservation(conn.QueryRow(ctx, query, code))
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		return nil, MapPostgresError(err, "Reservation")
	}

	return res, nil
}

func (r *postgresReservationRepository) List(ctx context.Context, filter domain.ReservationFilter) ([]domain.Reservation, error) {
	conn := GetDB(ctx, r.pool.Pool)

	var conditions []string
	var args []any
	argIdx := 1

	if filter.BusinessID != nil {
		conditions = append(conditions, fmt.Sprintf("business_id = $%d", argIdx))
		args = append(args, *filter.BusinessID)
		argIdx++
	}

	if filter.CustomerUserID != nil {
		conditions = append(conditions, fmt.Sprintf("customer_user_id = $%d", argIdx))
		args = append(args, *filter.CustomerUserID)
		argIdx++
	}

	if filter.Status != nil {
		conditions = append(conditions, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *filter.Status)
		argIdx++
	}

	if filter.ReservationDate != nil {
		conditions = append(conditions, fmt.Sprintf("reservation_date = $%d::date", argIdx))
		args = append(args, *filter.ReservationDate)
		argIdx++
	}

	limit := filter.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	offset := filter.Offset
	if offset < 0 {
		offset = 0
	}

	whereClause := ""
	if len(conditions) > 0 {
		whereClause = "WHERE " + strings.Join(conditions, " AND ")
	}

	args = append(args, limit, offset)
	query := fmt.Sprintf(`
		SELECT %s
		FROM reservations
		%s
		ORDER BY reservation_date DESC, reservation_time DESC
		LIMIT $%d OFFSET $%d;
	`, reservationSelectCols, whereClause, argIdx, argIdx+1)

	rows, err := conn.Query(ctx, query, args...)
	if err != nil {
		return nil, MapPostgresError(err, "Reservation")
	}
	defer rows.Close()

	var reservations []domain.Reservation
	for rows.Next() {
		var res domain.Reservation
		if err := rows.Scan(
			&res.ID,
			&res.ReservationCode,
			&res.BusinessID,
			&res.CustomerUserID,
			&res.CustomerName,
			&res.CustomerPhone,
			&res.ReservationDate,
			&res.ReservationTime,
			&res.PartySize,
			&res.SpecialRequest,
			&res.Status,
			&res.DeclineReason,
			&res.CreatedAt,
			&res.ConfirmedAt,
			&res.DeclinedAt,
			&res.CompletedAt,
			&res.CancelledAt,
			&res.ReviewedByUser,
			&res.UpdatedAt,
		); err != nil {
			return nil, err
		}
		reservations = append(reservations, res)
	}

	return reservations, nil
}

func (r *postgresReservationRepository) Confirm(ctx context.Context, id domain.ReservationID, reviewedBy *domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reservations
		SET status = 'CONFIRMED', reviewed_by_user = $1, updated_at = NOW()
		WHERE id = $2;
	`

	cmd, err := conn.Exec(ctx, query, reviewedBy, id)
	if err != nil {
		return MapPostgresError(err, "Reservation")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReservationRepository) Decline(ctx context.Context, id domain.ReservationID, reason string, reviewedBy *domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reservations
		SET status = 'DECLINED', decline_reason = $1, reviewed_by_user = $2, updated_at = NOW()
		WHERE id = $3;
	`

	cmd, err := conn.Exec(ctx, query, reason, reviewedBy, id)
	if err != nil {
		return MapPostgresError(err, "Reservation")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReservationRepository) Cancel(ctx context.Context, id domain.ReservationID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reservations
		SET status = 'CANCELLED', updated_at = NOW()
		WHERE id = $1;
	`

	cmd, err := conn.Exec(ctx, query, id)
	if err != nil {
		return MapPostgresError(err, "Reservation")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}

func (r *postgresReservationRepository) Complete(ctx context.Context, id domain.ReservationID, reviewedBy *domain.UserID) error {
	conn := GetDB(ctx, r.pool.Pool)

	query := `
		UPDATE reservations
		SET status = 'COMPLETED', reviewed_by_user = $1, updated_at = NOW()
		WHERE id = $2;
	`

	cmd, err := conn.Exec(ctx, query, reviewedBy, id)
	if err != nil {
		return MapPostgresError(err, "Reservation")
	}
	if cmd.RowsAffected() == 0 {
		return domain.ErrNotFound
	}

	return nil
}
