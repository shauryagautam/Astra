package testing

import (
	"context"
	"testing"

	"github.com/astraframework/astra/config"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

// NewTestDB creates a database connection that rolls back automatically.
func NewTestDB(t *testing.T, cfg config.DatabaseConfig) *pgxpool.Pool {
	pool, err := pgxpool.New(context.Background(), cfg.URL)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Close()
	})

	return pool
}

// WithTestTransaction runs a function inside a transaction that is rolled back.
func WithTestTransaction(t *testing.T, pool *pgxpool.Pool, fn func(tx pgx.Tx)) {
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	require.NoError(t, err)

	defer func() {
		_ = tx.Rollback(ctx)
	}()

	fn(tx)
}
