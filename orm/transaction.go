package orm

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// Transaction executes a function within a transaction.
// It automatically rolls back on error or panic, and commits on success.
// Supports nested transactions using SAVEPOINTs.
func (db *DB) Transaction(ctx context.Context, fn func(tx *DB) error) error {
	// Check if we are already in a transaction
	if tx, ok := db.conn.(Transaction); ok {
		_ = tx // keep tx reference if needed, but we use db.Exec
		// Use SAVEPOINT for nested transaction
		spID := uuid.NewString()
		spName := "astra_sp_" + strings.ReplaceAll(spID, "-", "_")

		if _, err := db.Exec(ctx, "SAVEPOINT "+spName); err != nil {
			return fmt.Errorf("orm: failed to create savepoint: %w", err)
		}

		defer func() {
			if r := recover(); r != nil {
				_, _ = db.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
				panic(r)
			}
		}()

		if err := fn(db); err != nil {
			_, _ = db.Exec(ctx, "ROLLBACK TO SAVEPOINT "+spName)
			return err
		}

		if _, err := db.Exec(ctx, "RELEASE SAVEPOINT "+spName); err != nil {
			return fmt.Errorf("orm: failed to release savepoint: %w", err)
		}
		return nil
	}

	connTx, err := db.conn.Begin(ctx)
	if err != nil {
		return err
	}

	// Create a new DB instance sharing the same dialect and auditor but using the transaction connection
	txDB := &DB{
		conn:    connTx,
		dialect: db.dialect,
		auditor: db.auditor,
		pool:    db.pool,
	}

	defer func() {
		if r := recover(); r != nil {
			_ = connTx.Rollback()
			panic(r) // Re-panic after rollback
		}
	}()

	if err := fn(txDB); err != nil {
		_ = connTx.Rollback()
		return err
	}

	return connTx.Commit()
}
