package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/astraframework/astra/config"
	"github.com/jackc/pgx/v5/pgxpool"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// DB is the database service holding the Gorm connection pool and the raw pgx pool.
type DB struct {
	Orm    *gorm.DB
	Pool   *pgxpool.Pool
	config config.DatabaseConfig
}

// Name returns the service name.
func (d *DB) Name() string {
	return "db"
}

// Start initializes the database connection pool using GORM and pgxpool.
func (d *DB) Start(ctx context.Context) error {
	ormDB, pool, err := Connect(ctx, d.config)
	if err != nil {
		return err
	}
	d.Orm = ormDB
	d.Pool = pool
	return nil
}

// Stop closes the underlying database connection pool.
func (d *DB) Stop(ctx context.Context) error {
	if d.Orm != nil {
		sqlDB, err := d.Orm.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
	return nil
}

// New creates a new DB service.
func New(cfg config.DatabaseConfig) *DB {
	return &DB{
		config: cfg,
	}
}

// Connect creates standalone gorm.DB and pgxpool.Pool instances.
func Connect(ctx context.Context, cfg config.DatabaseConfig) (*gorm.DB, *pgxpool.Pool, error) {
	// 1. Initialize raw pgxpool (required by migrations and seeders)
	pool, err := pgxpool.New(ctx, cfg.URL)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to pgxpool: %w", err)
	}

	// 2. Initialize GORM with retry logic for NeonDB cold starts
	var ormDB *gorm.DB


	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Warn), // Customize as needed
	}

	for attempt := 1; attempt <= 3; attempt++ {
		ormDB, err = gorm.Open(postgres.Open(cfg.URL), gormConfig)
		if err == nil {
			var sqlDB interface{ Ping() error }
			sqlDB, err = ormDB.DB()
			if err == nil {
				if pingErr := sqlDB.Ping(); pingErr == nil {
					break
				} else {
					err = pingErr
				}
			}
		}
		slog.Warn("postgres cold start, retrying...", "attempt", attempt, "error", err)
		time.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
	}

	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to postgres after 3 attempts: %w", err)
	}

	// Apply connection pool settings
	sqlDB, err := ormDB.DB()
	if err != nil {
		return nil, nil, err
	}
	
	// Default to reasonable values if config provides 0
	maxConns := int(cfg.MaxConns)
	if maxConns <= 0 {
		maxConns = 25
	}
	
	sqlDB.SetMaxOpenConns(maxConns)
	sqlDB.SetMaxIdleConns(int(cfg.MinConns))
	sqlDB.SetConnMaxIdleTime(cfg.MaxIdle)

	slog.Info("✓ Postgres connected via GORM")
	return ormDB, pool, nil
}
