package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Abdibaye/cafnet_backend/internal/domain"
	"github.com/Abdibaye/cafnet_backend/pkg/db"
)

// DBTX defines the shared interface between *pgxpool.Pool and pgx.Tx.
type DBTX interface {
	Exec(ctx context.Context, sql string, arguments ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// TxManager manages database transaction boundaries across repositories.
type TxManager interface {
	WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error
}

type txKey struct{}

// InjectTx places an active pgx.Tx into the context.
func InjectTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txKey{}, tx)
}

// ExtractTx retrieves an active pgx.Tx from the context if present.
func ExtractTx(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txKey{}).(pgx.Tx)
	return tx, ok
}

// GetDB returns either the active transaction or the fallback connection pool.
func GetDB(ctx context.Context, pool *pgxpool.Pool) DBTX {
	if tx, ok := ExtractTx(ctx); ok {
		return tx
	}
	return pool
}

type postgresTxManager struct {
	pool *db.Pool
}

// NewTxManager initializes a transaction manager for Neon PostgreSQL.
func NewTxManager(pool *db.Pool) TxManager {
	return &postgresTxManager{pool: pool}
}

// WithinTransaction executes fn within an atomic transaction. If an active transaction
// already exists in the context, it reuses it without creating a redundant sub-transaction.
func (tm *postgresTxManager) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := ExtractTx(ctx); ok {
		return fn(ctx)
	}

	tx, err := tm.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.NewDomainError(domain.ErrDatabaseUnavailable, "failed to begin transaction", err)
	}

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	txCtx := InjectTx(ctx, tx)
	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.NewDomainError(domain.ErrTransactionFailure, "failed to commit transaction", err)
	}

	return nil
}
