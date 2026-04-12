package database

import (
	"context"
	"database/sql"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type tracingConn struct {
	inner  Connection
	tracer trace.Tracer
}

func (c *tracingConn) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	ctx, span := c.tracer.Start(ctx, "db.exec", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	res, err := c.inner.Exec(ctx, sqlStr, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}

func (c *tracingConn) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	ctx, span := c.tracer.Start(ctx, "db.query", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	rows, err := c.inner.Query(ctx, sqlStr, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return rows, err
}

func (c *tracingConn) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	ctx, span := c.tracer.Start(ctx, "db.query_row", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	return c.inner.QueryRow(ctx, sqlStr, args...)
}

func (c *tracingConn) Begin(ctx context.Context) (Transaction, error) {
	ctx, span := c.tracer.Start(ctx, "db.begin")
	defer span.End()

	tx, err := c.inner.Begin(ctx)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}

	return &tracingTx{inner: tx, tracer: c.tracer}, nil
}

func (c *tracingConn) Close() error {
	return c.inner.Close()
}

type tracingTx struct {
	inner  Transaction
	tracer trace.Tracer
}

func (t *tracingTx) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	ctx, span := t.tracer.Start(ctx, "db.tx.exec", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	res, err := t.inner.Exec(ctx, sqlStr, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return res, err
}

func (t *tracingTx) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	ctx, span := t.tracer.Start(ctx, "db.tx.query", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	rows, err := t.inner.Query(ctx, sqlStr, args...)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return rows, err
}

func (t *tracingTx) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	ctx, span := t.tracer.Start(ctx, "db.tx.query_row", trace.WithAttributes(
		attribute.String("db.statement", sqlStr),
	))
	defer span.End()

	return t.inner.QueryRow(ctx, sqlStr, args...)
}

func (t *tracingTx) Begin(ctx context.Context) (Transaction, error) {
	return t.inner.Begin(ctx)
}

func (t *tracingTx) Close() error {
	return t.inner.Close()
}

func (t *tracingTx) Commit() error {
	_, span := t.tracer.Start(context.Background(), "db.tx.commit")
	defer span.End()

	err := t.inner.Commit()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}

func (t *tracingTx) Rollback() error {
	_, span := t.tracer.Start(context.Background(), "db.tx.rollback")
	defer span.End()

	err := t.inner.Rollback()
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
	return err
}
