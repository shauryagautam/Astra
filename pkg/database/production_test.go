package database

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

type productionUser struct {
	Model
	Name string `orm:"column:name"`
}

func (productionUser) TableName() string { return "production_users" }

func openProductionDB(t *testing.T) *DB {
	t.Helper()
	db, err := Open(Config{
		Driver:  "sqlite",
		DSN:     filepath.Join(t.TempDir(), "test.db"),
		MaxOpen: 1,
		MaxIdle: 1,
	})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	_, err = db.Exec(context.Background(), `CREATE TABLE production_users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		created_at DATETIME,
		updated_at DATETIME,
		deleted_at DATETIME
	)`)
	require.NoError(t, err)
	return db
}

func TestDB_ConcurrentCRUDAndPoolStats(t *testing.T) {
	db := openProductionDB(t)
	ctx := context.Background()
	const workers = 20
	const perWorker = 10
	var wg sync.WaitGroup
	errs := make(chan error, workers)

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				_, err := QueryContext[productionUser](ctx, db).Create(&productionUser{Name: "user"}, ctx)
				if err != nil {
					errs <- err
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	count, err := QueryContext[productionUser](ctx, db).Count(ctx)
	require.NoError(t, err)
	require.Equal(t, int64(workers*perWorker), count)
	require.GreaterOrEqual(t, db.Stats().MaxOpenConnections, 1)
}

func TestQueryBuilder_ExplicitValidation(t *testing.T) {
	db := openProductionDB(t)
	ctx := context.Background()

	_, err := Query[productionUser](db).Table("users; DROP TABLE production_users").Get(ctx)
	require.ErrorContains(t, err, "invalid table name")
	_, err = Query[productionUser](db).Limit(-1).Get(ctx)
	require.ErrorContains(t, err, "limit")
	_, err = Query[productionUser](db).Paginate(0, 10, ctx)
	require.ErrorContains(t, err, "page")
	_, err = Query[productionUser](db).Paginate(1, 0, ctx)
	require.ErrorContains(t, err, "per-page")
}

func TestDB_RawQueryHonorsTransactionContext(t *testing.T) {
	db := openProductionDB(t)
	ctx := context.Background()
	rollbackErr := errors.New("rollback")

	err := db.Transaction(ctx, func(txCtx context.Context) error {
		_, err := Query[productionUser](db).Create(&productionUser{Name: "inside"}, txCtx)
		if err != nil {
			return err
		}
		var users []productionUser
		if err := Raw[productionUser](db, "SELECT * FROM production_users WHERE name = ?", "inside").Scan(&users, txCtx); err != nil {
			return err
		}
		if len(users) != 1 {
			return errors.New("transaction raw query did not see uncommitted row")
		}
		return rollbackErr
	})
	require.ErrorIs(t, err, rollbackErr)

	count, err := Query[productionUser](db).Count(ctx)
	require.NoError(t, err)
	require.Zero(t, count)
}

func TestDB_TransactionContextCancellation(t *testing.T) {
	db := openProductionDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := db.Transaction(ctx, func(context.Context) error {
		t.Fatal("transaction callback should not run with canceled context")
		return nil
	})
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
}

func TestRunnerDataSetsAreExplicitAndConcurrentSafe(t *testing.T) {
	runner := NewRunner()
	var wg sync.WaitGroup
	for i := 0; i < 25; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			runner.Register(testDataSet{name: fmt.Sprintf("set-%d", i)})
		}(i)
	}
	wg.Wait()
	require.Len(t, runner.Names(), 25)
}

type testDataSet struct{ name string }

func (d testDataSet) Name() string                   { return d.name }
func (d testDataSet) Run(context.Context, *DB) error { return nil }
