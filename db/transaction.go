package db

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// transactionKey is the context key for the current transaction.
type transactionKey struct{}

// WithTransaction executes a function within a database transaction.
func WithTransaction(ctx context.Context, pool *pgxpool.Pool, fn func(ctx context.Context) error) error {
	// If already in a transaction, just use it
	if _, ok := ctx.Value(transactionKey{}).(pgx.Tx); ok {
		return fn(ctx)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("db: failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	txCtx := context.WithValue(ctx, transactionKey{}, tx)

	if err := fn(txCtx); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("db: failed to commit transaction: %w", err)
	}

	return nil
}

// GetTx returns the transaction from the context, or nil if none exists.
func GetTx(ctx context.Context) pgx.Tx {
	tx, _ := ctx.Value(transactionKey{}).(pgx.Tx)
	return tx
}
