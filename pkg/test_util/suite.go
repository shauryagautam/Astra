package test_util

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	redisclient "github.com/redis/go-redis/v9"
	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/database"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/modules/redis"
	"github.com/testcontainers/testcontainers-go/wait"
)

// Suite is a base testing suite providing real service containers.
type Suite struct {
	suite.Suite
	App         *engine.App
	Postgres    *postgres.PostgresContainer
	Redis       *redis.RedisContainer
	Ctx         context.Context
	DB          *database.DB
	RedisClient redisclient.UniversalClient

	activeTx   database.Transaction
	originalDB *database.DB
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
	if s.App == nil {
		cfg := &config.AstraConfig{}
		env := &config.Config{}
		logger := slog.Default()
		s.App = engine.New(cfg, env, logger)
	}

	// 4. Initialize Encryption for tests
	err = database.InitializeEncryption("astra-test-key-32-chars-long-!!!")
	require.NoError(s.T(), err)

	// 5. Register services
	ormCfg := database.Config{
		Driver: "postgres",
		DSN:    pgConn,
	}
	db, err := database.Open(ormCfg)
	require.NoError(s.T(), err)
	if db != nil {
		s.DB = db
	}

	// Register Redis
	if redisURL != "" {
		s.RedisClient = redisclient.NewUniversalClient(&redisclient.UniversalOptions{
			Addrs: []string{fmt.Sprintf("%s:%s", redisHost, redisPort.Port())},
		})
	}
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

	if s.DB == nil {
		return
	}

	tx, err := s.DB.Begin(s.Ctx)
	s.NoError(err)
	if err != nil {
		return
	}

	s.activeTx = tx
	s.originalDB = s.DB

	// Replace the registered db with the transaction-scoped one.
	s.DB = s.DB.WithTx(tx)
}

// TearDownTest rolls back the transaction after each test,
// restoring the database to its pristine state.
func (s *Suite) TearDownTest() {
	if s.activeTx != nil {
		_ = s.activeTx.Rollback()
		s.activeTx = nil
	}
	if s.originalDB != nil {
		s.DB = s.originalDB
		s.originalDB = nil
	}
}
