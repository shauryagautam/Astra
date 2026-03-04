package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the database service holding the connection pool.
type DB struct {
	Pool   *pgxpool.Pool
	config config.DatabaseConfig
}

// Name returns the service name.
func (d *DB) Name() string {
	return "db"
}

// Start initializes the database connection pool.
func (d *DB) Start(ctx context.Context) error {
	cfg, err := pgxpool.ParseConfig(d.config.URL)
	if err != nil {
		return fmt.Errorf("db: failed to parse connection string: %w", err)
	}

	cfg.MaxConns = d.config.MaxConns
	cfg.MinConns = d.config.MinConns
	cfg.MaxConnIdleTime = d.config.MaxIdle

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return fmt.Errorf("db: failed to create connection pool: %w", err)
	}

	// Ping to verify connection
	if err := pool.Ping(ctx); err != nil {
		return fmt.Errorf("db: failed to ping database: %w", err)
	}

	d.Pool = pool
	return nil
}

// Stop closes the database connection pool.
func (d *DB) Stop(ctx context.Context) error {
	if d.Pool != nil {
		d.Pool.Close()
	}
	return nil
}

// New creates a new DB service.
func New(cfg config.DatabaseConfig) *DB {
	return &DB{
		config: cfg,
	}
}

// Connect creates a standalone connection pool (useful for CLI and auto-loading).
func Connect(cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.URL)
	if err != nil {
		return nil, err
	}

	// NeonDB serverless tuning
	poolCfg.MaxConns = cfg.MaxConns
	poolCfg.MinConns = cfg.MinConns
	poolCfg.MaxConnIdleTime = cfg.MaxIdle
	poolCfg.HealthCheckPeriod = 60 * time.Second

	// Retry on cold start (NeonDB wakes up in ~500ms)
	var pool *pgxpool.Pool
	ctx := context.Background()

	for attempt := 1; attempt <= 3; attempt++ {
		pool, err = pgxpool.NewWithConfig(ctx, poolCfg)
		if err == nil {
			if pingErr := pool.Ping(ctx); pingErr == nil {
				break
			} else {
				err = pingErr
			}
		}
		slog.Warn("postgres cold start, retrying...", "attempt", attempt, "error", err)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to connect to postgres after 3 attempts: %w", err)
	}

	slog.Info("✓ Postgres connected", "host", poolCfg.ConnConfig.Host)
	return pool, nil
}
