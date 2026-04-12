# 06. Observability

Operational confidence comes from two things: knowing what the app is doing right now and degrading cleanly when a dependency starts failing. Astra treats logging, tracing, and resilience as one system.

## Why observability and resilience belong together

If the logs are clean but the system falls over under pressure, you still have a production problem. If the system is resilient but opaque, you cannot tell what failed. Astra gives you both sides: structured telemetry for diagnosis and controlled failure behavior for survival.

## Structured JSON logging

Astra builds on the standard `log/slog` package and emits JSON logs by default. The logger is meant to be contextual, which means request IDs, trace IDs, and redacted sensitive fields should follow the request automatically.

The engine exposes `WithContext(ctx, logger)` to attach request-scoped identifiers, and `NewRedactingHandler` to mask values like tokens and passwords before they leave the process.

> [!TIP]
> Use structured logs everywhere. Free-form strings make correlation harder exactly when you need it most.

## OpenTelemetry

OpenTelemetry is wired through the HTTP middleware layer and the supporting providers. That means trace context follows the request through your handlers, database calls, queues, and downstream service clients.

The practical outcome is that you can answer two questions quickly:

1. Where did the request spend time?
2. Which dependency caused the failure?

## Distributed circuit breakers

Astra includes both local and Redis-backed circuit breakers in `pkg/observability/fault_tolerance`.

Use the local breaker when the protection is process-local. Use the distributed breaker when multiple app instances need to share the same open/half-open state.

That distinction matters in production because the breaker state should be visible to every instance that is calling the same dependency.

## Rate limiting

Astra’s HTTP rate limiter uses Redis-backed sliding-window or token-bucket logic. It is designed to be middleware, not a custom ad hoc check in every handler.

Because the limiter is middleware, the response headers are consistent and the rejection path is standardized. That makes the behavior easier to reason about in clients and easier to observe in logs.

> [!WARNING]
> Keep rate limiting close to the edge of the system. If you wait until deep inside the handler chain, you have already spent resources you were trying to protect.

## Copy-Paste Example

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "time"

    "github.com/redis/go-redis/v9"
    "github.com/shauryagautam/Astra/pkg/engine"
    astrahttp "github.com/shauryagautam/Astra/pkg/engine/http"
    "github.com/shauryagautam/Astra/pkg/observability/fault_tolerance"
)

func main() {
    baseLogger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
    secureLogger := slog.New(engine.NewRedactingHandler(baseLogger.Handler()))

    ctx := context.WithValue(context.Background(), engine.RequestIDKey, "req-123")
    ctx = context.WithValue(ctx, engine.TraceIDKey, "trace-456")
    _ = engine.WithContext(ctx, secureLogger)

    breaker := fault_tolerance.NewDistributedCircuitBreaker(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), "email-service", fault_tolerance.DistributedCircuitBreakerOptions{
        MaxFailures:  3,
        ResetTimeout:  30 * time.Second,
    })

    _ = breaker.Execute(ctx, func() error {
        return nil
    })

    limitMiddleware, _ := astrahttp.RateLimit(redis.NewClient(&redis.Options{Addr: "localhost:6379"}), 100, time.Minute)
    _ = limitMiddleware
}
```

---

**Next Chapter: [07. Testing](./07-testing.md)**
