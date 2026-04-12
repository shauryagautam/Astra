package database

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTransaction_ContextAware(t *testing.T) {
	ctx := context.Background()
	db, err := Open(Config{
		Driver: "sqlite",
		DSN:    ":memory:",
	})
	assert.NoError(t, err)
	defer db.Close()

	// Create table
	_, err = db.Exec(ctx, "CREATE TABLE users (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT, email TEXT, created_at DATETIME, updated_at DATETIME, deleted_at DATETIME)")
	assert.NoError(t, err)

	t.Run("Commit updates context-aware queries", func(t *testing.T) {
		err := db.Transaction(ctx, func(txCtx context.Context) error {
			user := &User{Name: "TxUser", Email: "tx@example.com"}
			// Query should automatically pick up the transaction from txCtx
			_, err := Query[User](db, txCtx).Create(user)
			if err != nil {
				return err
			}

			// Verify it exists within the transaction
			found, err := Query[User](db, txCtx).Where("name", "=", "TxUser").First()
			assert.NoError(t, err)
			assert.NotNil(t, found)
			return nil
		})
		assert.NoError(t, err)

		// Verify it exists after commit
		found, err := Query[User](db).Where("name", "=", "TxUser").First(ctx)
		assert.NoError(t, err)
		assert.NotNil(t, found)
	})

	t.Run("Rollback on error", func(t *testing.T) {
		err := db.Transaction(ctx, func(txCtx context.Context) error {
			user := &User{Name: "RollbackUser", Email: "rb@example.com"}
			_, err := Query[User](db, txCtx).Create(user)
			assert.NoError(t, err)

			return assert.AnError // Force rollback
		})
		assert.ErrorIs(t, err, assert.AnError)

		// Verify it does NOT exist after rollback
		_, err = Query[User](db).Where("name", "=", "RollbackUser").First(ctx)
		assert.Error(t, err)
	})

	t.Run("Nested Transactions (Savepoints)", func(t *testing.T) {
		err := db.Transaction(ctx, func(txCtx context.Context) error {
			user1 := &User{Name: "Outer", Email: "outer@example.com"}
			_, err := Query[User](db, txCtx).Create(user1)
			assert.NoError(t, err)

			// Nested transaction
			err = db.Transaction(txCtx, func(nestedCtx context.Context) error {
				user2 := &User{Name: "Inner", Email: "inner@example.com"}
				_, err := Query[User](db, nestedCtx).Create(user2)
				assert.NoError(t, err)
				return nil // Commit inner
			})
			assert.NoError(t, err)

			// Another nested transaction that fails
			_ = db.Transaction(txCtx, func(nestedCtx context.Context) error {
				user3 := &User{Name: "InnerFail", Email: "fail@example.com"}
				_, _ = Query[User](db, nestedCtx).Create(user3)
				return assert.AnError // Rollback inner only
			})

			return nil // Commit outer
		})
		assert.NoError(t, err)

		// Outer and first inner should exist
		count, _ := Query[User](db).WhereIn("name", []any{"Outer", "Inner"}).Count(ctx)
		assert.Equal(t, int64(2), count)

		// Failed inner should not exist
		count, _ = Query[User](db).Where("name", "=", "InnerFail").Count(ctx)
		assert.Equal(t, int64(0), count)
	})
}
