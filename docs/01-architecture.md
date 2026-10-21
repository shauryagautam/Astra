# Architecture

> Source files reviewed for this document: `pkg/engine/app.go`,
> `pkg/engine/component.go`, `pkg/engine/interfaces.go`,
> `pkg/engine/http/server.go`, `pkg/engine/runtime/wire_components.go`,
> `wire.go`, `internal/scaffold/tpl/*.tmpl`, and `ARCHITECTURE.md`.

Astra is a set of ordinary Go packages. An application owns its composition
root, constructs dependencies, and decides which packages to use.

## Design principles

1. **Explicit composition.** A constructor signature is the dependency graph.
2. **No runtime service locator.** `engine.App` manages lifecycles; it does not
   resolve application services by name.
3. **Context everywhere.** Long-running work receives a context and must honor
   cancellation and deadlines.
4. **Small interfaces at boundaries.** Packages accept the narrow interface
   needed for a behavior, or a concrete dependency where that is clearer.
5. **Standard library compatibility.** HTTP code uses `http.Handler`,
   `context.Context`, `database/sql`, and `log/slog` instead of replacing them.
6. **Independent packages.** Database, cache, auth, queue, and observability
   packages can be used without starting the kernel.

## Package boundaries

| Package | Responsibility |
| --- | --- |
| `pkg/engine` | Application lifecycle, configuration contracts, health checks |
| `pkg/engine/http` | Router, server, request context, middleware, HTTP helpers |
| `pkg/engine/config` | `.env`, YAML, TOML, environment, and typed configuration |
| `pkg/database` | SQL pool wrapper, model metadata, query builder, relations, transactions |
| `pkg/database/migration` | SQL migration discovery, checksums, locking, rollback |
| `pkg/identity/auth` | JWT, password hashing, sessions, OAuth2, TOTP |
| `pkg/identity/rbac` | Roles, permissions, policies, and access decisions |
| `pkg/queue` | Redis-backed queues, workers, retries, delayed jobs, scheduling |
| `pkg/cache` | In-memory and Redis stores, locks, and cache interfaces |
| `pkg/observability` | Health, audit, metrics, tracing, and fault tolerance |
| `pkg/storage` | Local and S3 storage implementations |
| `pkg/mail` | Mail interfaces and concrete mailers |
| `pkg/graphql` | GraphQL schema/handler helpers |
| `pkg/ws` | WebSocket support |
| `cmd/astra` | Scaffolding and development operations |

## Lifecycle kernel

The managed resource contract is deliberately small:

```go
type Component interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

Create an application with `engine.New` or `engine.NewWithContext`. The latter
is useful when a supervisor supplies a parent context:

```go
app := engine.NewWithContext(parent, cfg, env, logger)

if err := app.RegisterComponent(databaseComponent); err != nil {
    return err
}
if err := app.RegisterComponent(httpServer); err != nil {
    return err
}
return app.Run()
```

Components start serially in registration order. Register dependencies before
the components that depend on them. A component must not return from `Start`
until it is ready, and must tolerate a partially completed start. Shutdown
runs components in reverse order and joins component and hook errors with
`errors.Join`.

`Boot` starts components and hooks. `Run` calls `Boot`, waits for the lifecycle
context, and calls `Shutdown`. `Start` is an alias for `Run`; it does not start
a separate mode. The kernel installs `signal.NotifyContext` for interrupt and
`SIGTERM`, while the parent context remains the cancellation boundary.

Startup and shutdown use internal bounded contexts. Components must still
select on the context they receive; the manager cannot interrupt code that
ignores it.

## Wire

Wire is compile-time constructor composition. It generates ordinary Go code;
there is no reflection-based runtime container.

```go
//go:build wireinject

func InitializeApp() (*engine.App, func(), error) {
    wire.Build(
        NewConfig,
        NewLogger,
        engine.New,
    )
    return nil, nil, nil
}
```

Keep the Wire injector in the application. Use `wire.Bind` only when a
constructor returns a concrete type that must satisfy an interface. Register
lifecycle components explicitly after construction, or include a constructor
that returns the component in the application graph.

## Health checks

`App.RegisterHealthCheck` stores named checks. The HTTP package provides
`HealthHandler`, which runs all checks concurrently and returns HTTP 200 when
all pass or HTTP 503 when one fails. `ReadyHandler` is a simple `"OK"`
endpoint and does not perform dependency checks.

## CLI scaffold

The CLI creates a small root-level application:

```text
main.go
wire.go
routes.go
database/migrations/
database/data/
```

`handler/`, `model/`, and `middleware/` are optional starter directories.
There is no filename-based discovery and no required `app/` or
`internal/routes/` tree.
