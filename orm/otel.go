package orm

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// tracingConn wraps a Connection to emit OpenTelemetry spans for every
// database operation. It is wired in automatically when Config.Tracer is set.
type tracingConn struct {
	inner  Connection
	tracer trace.Tracer
}

func (t *tracingConn) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	ctx, span := t.startSpan(ctx, "orm.exec", sqlStr)
	defer span.End()

	result, err := t.inner.Exec(ctx, sqlStr, args...)
	recordSpanError(span, err)
	return result, err
}

func (t *tracingConn) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	ctx, span := t.startSpan(ctx, "orm.query", sqlStr)
	defer span.End()

	rows, err := t.inner.Query(ctx, sqlStr, args...)
	recordSpanError(span, err)
	return rows, err
}

func (t *tracingConn) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	_, span := t.startSpan(ctx, "orm.queryrow", sqlStr)
	defer span.End()
	return t.inner.QueryRow(ctx, sqlStr, args...)
}

func (t *tracingConn) Begin(ctx context.Context) (Transaction, error) {
	ctx, span := t.tracer.Start(ctx, "orm.begin")
	defer span.End()
	tx, err := t.inner.Begin(ctx)
	recordSpanError(span, err)
	if err != nil {
		return nil, err
	}
	// Wrap the transaction so its queries are also traced.
	return &tracingTx{inner: tx, tracer: t.tracer}, nil
}

func (t *tracingConn) Close() error { return t.inner.Close() }

func (t *tracingConn) startSpan(ctx context.Context, name, sqlStr string) (context.Context, trace.Span) {
	return t.tracer.Start(ctx, name,
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(
			attribute.String("db.system", "sql"),
			attribute.String("db.statement", truncateSQL(sqlStr, 512)),
		),
	)
}

// tracingTx wraps a Transaction to emit spans.
type tracingTx struct {
	inner  Transaction
	tracer trace.Tracer
}

func (t *tracingTx) Exec(ctx context.Context, sqlStr string, args ...any) (sql.Result, error) {
	_, span := t.tracer.Start(ctx, "orm.tx.exec", trace.WithAttributes(
		attribute.String("db.statement", truncateSQL(sqlStr, 512)),
	))
	defer span.End()
	result, err := t.inner.Exec(ctx, sqlStr, args...)
	recordSpanError(span, err)
	return result, err
}

func (t *tracingTx) Query(ctx context.Context, sqlStr string, args ...any) (Rows, error) {
	_, span := t.tracer.Start(ctx, "orm.tx.query", trace.WithAttributes(
		attribute.String("db.statement", truncateSQL(sqlStr, 512)),
	))
	defer span.End()
	rows, err := t.inner.Query(ctx, sqlStr, args...)
	recordSpanError(span, err)
	return rows, err
}

func (t *tracingTx) QueryRow(ctx context.Context, sqlStr string, args ...any) Row {
	return t.inner.QueryRow(ctx, sqlStr, args...)
}

func (t *tracingTx) Begin(ctx context.Context) (Transaction, error) {
	return nil, fmt.Errorf("orm: nested transactions not supported")
}

func (t *tracingTx) Commit() error   { return t.inner.Commit() }
func (t *tracingTx) Rollback() error { return t.inner.Rollback() }
func (t *tracingTx) Close() error    { return t.inner.Close() }

// ─── Helpers ──────────────────────────────────────────────────────────────────

func recordSpanError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func truncateSQL(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "…"
}

// ensure time package is used (avoids accidental import removal).
var _ = time.Now
