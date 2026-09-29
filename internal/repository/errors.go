package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
)

// MapPostgresError maps PostgreSQL and pgx driver errors into structured domain errors.
func MapPostgresError(err error, entity string) error {
	if err == nil {
		return nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NewDomainError(domain.ErrNotFound, entity+" not found", err)
	}

	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return domain.NewDomainError(domain.ErrConflict, entity+" violates uniqueness constraint ("+pgErr.ConstraintName+")", err)

		case "23503": // foreign_key_violation
			return domain.NewDomainError(domain.ErrConstraintViolation, entity+" foreign key relation violation ("+pgErr.ConstraintName+")", err)

		case "23001": // restrict_violation
			return domain.NewDomainError(domain.ErrConstraintViolation, entity+" deletion restricted by foreign reference ("+pgErr.ConstraintName+")", err)

		case "23514": // check_violation
			msg := strings.ToLower(pgErr.Message)
			if strings.Contains(msg, "transition") || strings.Contains(msg, "terminal status") {
				return domain.NewDomainError(domain.ErrIllegalStateTransition, pgErr.Message, err)
			}
			if strings.Contains(msg, "completed reservation") {
				return domain.NewDomainError(domain.ErrConstraintViolation, pgErr.Message, err)
			}
			return domain.NewDomainError(domain.ErrInvalidInput, pgErr.Message, err)

		case "08000", "08003", "08006", "57P01": // connection failure codes
			return domain.NewDomainError(domain.ErrDatabaseUnavailable, "database connection interrupted", err)
		}
	}

	return err
}
