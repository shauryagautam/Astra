# Observability

> Source files reviewed for this document: `pkg/engine/logger.go`,
> `pkg/observability/trace/trace.go`, `pkg/observability/metrics/metrics.go`,
> `pkg/observability/health/health.go`, `pkg/observability/audit/audit.go`,
> `pkg/engine/http/prometheus.go`, `pkg/engine/telemetry/*`,
> `pkg/engine/http/middleware_otel.go`, and observability tests.

## Structured logs

Use `log/slog` and include stable fields such as request ID, user ID, route,
duration, and error. The runtime Wire set creates a JSON logger. The engine
also provides a redacting handler:

```go
handler := engine.NewRedactingHandler(
    slog.NewJSONHandler(os.Stdout, nil),
    "password", "token", "authorization",
)
logger := slog.New(handler)
```

Do not emit credentials, raw session values, or unbounded request bodies.

## Tracing

`pkg/observability/trace.Init` configures an OTLP trace exporter and returns a
shutdown function:

```go
shutdown, err := trace.Init(
    ctx,
    os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
    "orders-api",
)
if err != nil {
    return err
}
defer shutdown(context.Background())
```

`StartSpan` and the HTTP/queue instrumentation use the global OpenTelemetry
provider. Configure the SDK once in the application composition root.

## Metrics

`pkg/observability/metrics.Init` configures OTLP metrics. The HTTP package
provides `PrometheusHandler`, which returns a standard `http.Handler` backed by
the Prometheus client library:

```go
r.Handle(http.MethodGet, "/metrics", http.PrometheusHandler())
```

The database package can observe SQL pool metrics through `ObservePool`.
Queue workers expose in-process counters through `Worker.Metrics`.

## Health and readiness

The engine-level `HealthCheck` interface can be registered with `App`.
Alternatively, `pkg/observability/health.HealthChecker` is a standalone
registry that produces a `HealthReport`. It includes component statuses and
helpers such as `DiskSpaceCheck` and `LatencyCheck`.

For HTTP:

```go
r.Get("/health", http.HealthHandler(app.GetHealthChecks()))
r.Get("/ready", http.ReadyHandler())
```

`HealthHandler` runs checks concurrently and returns 503 if any check fails.
`ReadyHandler` only returns `"OK"`; it is a liveness-style endpoint unless the
application replaces it with dependency-aware readiness logic.

## Audit logging

`observability/audit.NewAuditor` writes structured audit events to a configured
path. `AuditEvent` and database audit entries carry operation metadata. Treat
audit logs as sensitive operational data and protect their storage and access.

## Development dashboard

`pkg/engine/telemetry.Dashboard` maintains a bounded in-memory ring buffer for
requests, logs, SQL queries, jobs, mail, Redis commands, and events. It is
appropriate for development diagnostics. It is not a durable metrics store or
an authorization system; protect any dashboard endpoint and do not rely on it
for production retention.

## Failure tolerance

`pkg/observability/fault_tolerance` includes local and Redis-backed circuit
breakers. `CircuitBreaker.Execute` wraps a fallible operation and
`DistributedCircuitBreaker` shares state across instances. Choose thresholds
and reset timeouts based on the downstream dependency rather than defaults.
