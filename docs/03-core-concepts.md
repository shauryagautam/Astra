# Core concepts

> Source files reviewed for this document: `pkg/engine/app.go`,
> `pkg/engine/component.go`, `pkg/engine/config/config.go`,
> `pkg/engine/config/env.go`, `pkg/engine/logger.go`,
> `pkg/engine/http/context.go`, `pkg/engine/http/middleware_chain.go`,
> `pkg/engine/http/errors_handler.go`, and `pkg/engine/http/router.go`.

## Application lifecycle

`engine.App` is a lifecycle manager, not a dependency registry. Register
components explicitly with `RegisterComponent`. `OnStart` and `OnStop` hooks
are available for small application-level coordination, but a component is
usually clearer when a resource owns startup and shutdown.

`Boot` is idempotence-protected: a second startup attempt returns an error.
`Shutdown` is safe for concurrent callers; callers observe the same result.
Shutdown errors are logged and combined with `errors.Join`.

## Context

The application lifecycle context is available from `App.BaseContext`. A
component should use the context passed to `Start` and `Stop`, not a background
context:

```go
func (c *Worker) Start(ctx context.Context) error {
    c.started = true
    go c.loop(ctx)
    return nil
}

func (c *Worker) Stop(ctx context.Context) error {
    select {
    case c.stop <- struct{}{}:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

HTTP request contexts are exposed through `*http.Context`. Use `c.Ctx()` for
database, Redis, outbound HTTP, and other cancellable work.

## Configuration

`pkg/engine/config` provides two complementary APIs:

- `config.Load` builds a key/value `Config` from TOML, YAML, `.env`, and
  process environment values.
- `config.LoadFromEnv` maps environment values into `AstraConfig`.

`Config.String`, `Int`, `Int32`, `Bool`, and `Duration` apply defaults. The
environment helpers (`EnvGet`, `EnvGetOrFail`, and typed variants) read
directly from `os.Getenv`.

Call `AstraConfig.Validate` at startup. Do not log `Config.Raw()` or secrets;
`MaskSecrets` exists for diagnostics.

## Logging

Astra uses `log/slog`. The runtime Wire set provides a JSON logger whose level
is debug when `APP_DEBUG` is true. `engine.NewRedactingHandler` can redact
configured attribute keys, and `engine.WithContext` attaches a logger to a
context.

Prefer structured fields:

```go
logger.ErrorContext(ctx, "database query failed",
    "table", "users",
    "error", err,
)
```

Do not use `log.Fatal` in libraries. Return errors to the composition root.

## HTTP handlers and middleware

`pkg/engine/http.Router` provides convenience methods such as `Get`, `Post`,
and `Handle`, while remaining compatible with `net/http`. Middleware is
ordinary handler composition. Available middleware includes request IDs,
recovery, logging, compression, CORS, CSRF, rate limiting, timeouts,
authentication, RBAC, sessions, upload limits, validation, tracing, and
security headers.

Register middleware explicitly and in the order you want it applied:

```go
r := http.NewRouter(cfg, logger)
r.Use(http.SecureHeaders(cfg.App.Environment == "production"))
r.Use(http.Recovery())
r.Get("/users/{id}", showUser)
```

Check the exact function signature in `pkg/engine/http`; several middleware
functions require a configuration or injected dependency.

## Error handling

Handlers return `error`. The router/server error middleware translates errors
according to the configured behavior. Library code should wrap errors with
operation context:

```go
return fmt.Errorf("load user %d: %w", id, err)
```

The framework generally returns errors from constructors and terminal
operations. A fluent database builder records validation errors and reports
them when a terminal method executes.

## Events

`pkg/engine/event.Emitter` supports typed or payload events and listener
registration. The package also exposes a default emitter for compatibility;
prefer constructing an emitter and injecting it when isolation matters.
