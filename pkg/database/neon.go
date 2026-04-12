package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"
)

// NeonDialect extends PostgresDialect with serverless-specific optimizations.
// It handles Neon's "cold start" behavior by implementing automatic retries
// for common network errors that occur when compute units spin up.
type NeonDialect struct {
	PostgresDialect
}

// Name returns the name of the dialect.
func (d NeonDialect) Name() string { return "neon" }

// ConfigurePool applies Neon-specific connection pool settings.
// It optimizes for serverless environments by limiting total connections
// and reducing idle time to facilitate scale-to-zero.
func (d NeonDialect) ConfigurePool(db *sql.DB) {
	// Neon serverless recommendations
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(10 * time.Second)
	db.SetConnMaxLifetime(30 * time.Minute)
}

// neonConn wraps a database connection and adds cold-start resilience logic.
type neonConn struct {
	inner Connection
}

func (c *neonConn) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	var res sql.Result
	var err error
	err = retry(ctx, func() error {
		res, err = c.inner.Exec(ctx, query, args...)
		return err
	})
	return res, err
}

func (c *neonConn) Query(ctx context.Context, query string, args ...any) (Rows, error) {
	var res Rows
	var err error
	err = retry(ctx, func() error {
		res, err = c.inner.Query(ctx, query, args...)
		return err
	})
	return res, err
}

func (c *neonConn) QueryRow(ctx context.Context, query string, args ...any) Row {
	// QueryRow doesn't return an error immediately. We must handle retries in Scan.
	return &neonRow{
		inner: c.inner.QueryRow(ctx, query, args...),
		c:     c,
		ctx:   ctx,
		query: query,
		args:  args,
	}
}

func (c *neonConn) Begin(ctx context.Context) (Transaction, error) {
	var tx Transaction
	var err error
	err = retry(ctx, func() error {
		tx, err = c.inner.Begin(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	// Note: We don't wrap the transaction itself for retries because 
	// retrying a transaction block is unsafe without knowing it's idempotent.
	return tx, nil
}

func (c *neonConn) Close() error {
	return c.inner.Close()
}

// neonRow wraps a sql.Row to allow retries during scanning if the connection was dropped.
type neonRow struct {
	inner Row
	c     *neonConn
	ctx   context.Context
	query string
	args  []any
}

func (r *neonRow) Scan(dest ...any) error {
	return retry(r.ctx, func() error {
		err := r.inner.Scan(dest...)
		if err != nil && isColdStartError(err) {
			// If scan fails due to a cold start, re-execute the query row.
			r.inner = r.c.inner.QueryRow(r.ctx, r.query, r.args...)
			return err
		}
		return err
	})
}

// retry implements exponential backoff for database operations.
func retry(ctx context.Context, fn func() error) error {
	var lastErr error
	backoff := 500 * time.Millisecond
	maxAttempts := 3

	for i := 0; i < maxAttempts; i++ {
		err := fn()
		if err == nil {
			return nil
		}

		if !isColdStartError(err) {
			return err
		}

		lastErr = err
		
		// Wait before retrying
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(backoff):
			backoff *= 2
		}
	}

	return fmt.Errorf("neon: cold-start failure after %d attempts: %w", maxAttempts, lastErr)
}

// isColdStartError identifies errors that typically occur during a Neon cold start.
func isColdStartError(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	
	// Common Postgres/Network error patterns for dropped connections during spin-up
	if strings.Contains(msg, "connection refused") ||
		strings.Contains(msg, "eof") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "io timeout") ||
		strings.Contains(msg, "bad connection") {
		return true
	}

	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return true
	}

	return false
}
