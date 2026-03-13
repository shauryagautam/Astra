package testing

import (
	"context"
	"fmt"
	"time"

	"github.com/astraframework/astra/core"
	ormdb "github.com/astraframework/astra/orm"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Suite is a base testing suite providing real service containers.
type Suite struct {
	suite.Suite
	App      *core.App
	Postgres *postgres.PostgresContainer
	Redis    *redis.RedisContainer
	Ctx      context.Context

	activeTx   ormdb.Transaction
	originalDB *ormdb.DB
}

// SetupSuite starts the required containers and initializes the Astra app.
func (s *Suite) SetupSuite() {
	s.Ctx = context.Background()

	// 1. Start Postgres
	pgContainer, err := postgres.RunContainer(s.Ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		postgres.WithDatabase("astra_test"),
		postgres.WithUsername("astra"),
		postgres.WithPassword("astra"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	s.NoError(err)
	s.Postgres = pgContainer

	pgConn, err := pgContainer.ConnectionString(s.Ctx, "sslmode=disable")
	s.NoError(err)

	// 2. Start Redis
	redisContainer, err := redis.RunContainer(s.Ctx,
		testcontainers.WithImage("redis:7-alpine"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("Ready to accept connections"),
		),
	)
	s.NoError(err)
	s.Redis = redisContainer

	redisHost, err := redisContainer.Host(s.Ctx)
	s.NoError(err)
	redisPort, err := redisContainer.MappedPort(s.Ctx, "6379")
	s.NoError(err)
	redisURL := fmt.Sprintf("redis://%s:%s", redisHost, redisPort.Port())

	// 3. Initialize Astra App
	s.Ctx = context.WithValue(s.Ctx, "DATABASE_URL", pgConn)
	s.Ctx = context.WithValue(s.Ctx, "REDIS_URL", redisURL)

	// Here the user would typically call a helper to create their app with these URLs
}

// TearDownSuite stops all containers.
func (s *Suite) TearDownSuite() {
	if s.Postgres != nil {
		s.NoError(s.Postgres.Terminate(s.Ctx))
	}
	if s.Redis != nil {
		s.NoError(s.Redis.Terminate(s.Ctx))
	}
}

// SetupTest starts a database transaction before each test.
// It temporarily replaces the App's "db" with a transaction-scoped DB.
func (s *Suite) SetupTest() {
	if s.App == nil {
		return
	}

	dbRaw := s.App.Get("db")
	if dbRaw == nil {
		return
	}

	db, ok := dbRaw.(*ormdb.DB)
	if !ok {
		return
	}

	tx, err := db.Begin(s.Ctx)
	s.NoError(err)
	if err != nil {
		return
	}

	s.activeTx = tx
	s.originalDB = db

	// Replace the registered db with the transaction-scoped one.
	txDB := db.WithTx(tx)
	s.App.Register("db", txDB)
}

// TearDownTest rolls back the transaction after each test,
// restoring the database to its pristine state.
func (s *Suite) TearDownTest() {
	if s.activeTx != nil {
		_ = s.activeTx.Rollback()
		s.activeTx = nil
	}
	if s.originalDB != nil && s.App != nil {
		s.App.Register("db", s.originalDB)
		s.originalDB = nil
	}
}
