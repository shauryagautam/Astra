package database

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

type txKey struct{}
type txIDKey struct{}

// WithContext returns a new context with the transaction DB instance attached.
func WithContext(ctx context.Context, db *DB) context.Context {
	return context.WithValue(ctx, txKey{}, db)
}

// FromContext retrieves the transaction DB instance from the context if it exists.
func FromContext(ctx context.Context) (*DB, bool) {
	db, ok := ctx.Value(txKey{}).(*DB)
	return db, ok
}

// Transaction executes a function within a transaction.
// It automatically rolls back on error or panic, and commits on success.
// Supports nested transactions using SAVEPOINTs.
// The transaction-aware DB instance is injected into the context passed to fn.
func (db *DB) Transaction(ctx context.Context, fn func(txCtx context.Context) error) error {
	// Check if we are already in a transaction (either via db instance or context)
	currentDB := db
	if ctxDB, ok := FromContext(ctx); ok {
		currentDB = ctxDB
	}

	if currentDB.inTx {
		// Use SAVEPOINT for nested transaction
		spID := uuid.NewString()
		spName := "astra_sp_" + strings.ReplaceAll(spID, "-", "_")

		if _, err := currentDB.Exec(ctx, "SAVEPOINT "+spName); err != nil {
			return fmt.Errorf("orm: failed to create savepoint: %w", err)
		}

		// Create a shallow clone for the nested transaction
		nestedDB := *currentDB
		txDB := &nestedDB

		// Generate a unique sub-transaction ID for auditing
		txID := "subtx_" + strings.ReplaceAll(uuid.NewString(), "-", "")
		txCtx := context.WithValue(ctx, txIDKey{}, txID)
		txCtx = WithContext(txCtx, txDB)

		defer func() {
			if r := recover(); r != nil {
				_, _ = txDB.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
				panic(r)
			}
		}()

		if err := fn(txCtx); err != nil {
			_, _ = txDB.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
			return err
		}

		if _, err := txDB.Exec(ctx, "RELEASE SAVEPOINT "+spName); err != nil {
			return fmt.Errorf("orm: failed to release savepoint: %w", err)
		}
		return nil
	}

	connTx, err := db.conn.Begin(ctx)
	if err != nil {
		return err
	}

	// Generate a unique transaction ID for auditing
	txID := "tx_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	
	// Create a new DB instance sharing the same dialect and auditor but using the transaction connection
	txDB := &DB{
		conn:    connTx,
		dialect: db.dialect,
		auditor: db.auditor,
		pool:    db.pool,
		inTx:    true,
	}

	// Inject txDB and txID into context
	txCtx := context.WithValue(ctx, txIDKey{}, txID)
	txCtx = WithContext(txCtx, txDB)

	defer func() {
		if r := recover(); r != nil {
			_ = connTx.Rollback()
			panic(r) // Re-panic after rollback
		}
	}()

	if err := fn(txCtx); err != nil {
		_ = connTx.Rollback()
		return err
	}

	return connTx.Commit()
}
