# Astra

> Source files reviewed for this document: `go.mod`, `cmd/astra/main.go`,
> `pkg/engine/*`, `pkg/engine/http/*`, `pkg/database/*`, `pkg/identity/*`,
> `pkg/queue/*`, `pkg/observability/*`, and the examples under `examples/`.

Astra is a collection of production-oriented Go packages for building HTTP
services with explicit composition, context-aware lifecycles, SQL access,
authentication, background jobs, and observability.

It is not a runtime service container. Dependencies are constructed by your
application, optionally with Google Wire, and passed to the handlers,
repositories, and lifecycle components that use them.

## Features

- `net/http`-compatible routing and middleware
- Lifecycle management with `Start(context.Context)` and
  `Stop(context.Context)`
- Generic SQL query builder for PostgreSQL, MySQL, and SQLite
- Explicit model metadata, relations, transactions, soft deletes, and
  migration locking
- JWT access/refresh tokens, Redis-backed sessions, OAuth2 adapters, TOTP,
  RBAC, and policy evaluation
- Redis queues, workers, delayed jobs, failed-job storage, and cron scheduling
- `slog`, OpenTelemetry tracing/metrics, Prometheus handlers, health checks,
  and an in-memory development dashboard
- Local and S3 storage, mail abstractions, GraphQL helpers, WebSockets, and
  Vite asset helpers
- `astra` CLI with scaffolding, generators, migrations, key management,
  environment checks, and an `fsnotify` development watcher

## Requirements

- Go 1.26 or newer, as declared by [`go.mod`](./go.mod)
- Redis for Redis-backed queues, sessions, locks, or OAuth state
- PostgreSQL, MySQL, or SQLite when using the database package
- Docker is useful for integration tests that use Testcontainers

## Quickstart

Install the CLI:

```bash
go install github.com/shauryagautam/Astra/cmd/astra@latest
```

Create an application:

```bash
astra new example.com/myservice --api-only
cd example.com/myservice
astra key:generate
astra dev
```

The generated project is intentionally small. It contains a composition root
(`main.go` and `wire.go`), route registration (`routes.go`), SQL migrations,
and optional starter directories. Those directories are ordinary Go packages,
not framework registries.

The scaffold's generated Wire file is application-specific. If you change the
dependency graph, install Wire and regenerate it:

```bash
go install github.com/google/wire/cmd/wire@latest
wire
```

## Application shape

```text
myservice/
├── main.go
├── wire.go
├── routes.go
├── database/
│   ├── migrations/
│   └── data/
├── handler/       # optional
├── model/         # optional
└── middleware/    # optional
```

Move or remove these directories as the service grows. Astra does not discover
handlers, models, or routes by filename.

## Documentation

- [Architecture](docs/01-architecture.md)
- [Getting started](docs/02-getting-started.md)
- [Core concepts](docs/03-core-concepts.md)
- [Database and ORM](docs/04-database-and-orm.md)
- [Authentication and security](docs/05-authentication-and-security.md)
- [Background jobs and queues](docs/06-background-jobs-and-queues.md)
- [Observability](docs/07-observability.md)
- [CLI reference](docs/08-cli-reference.md)
- [Deployment](docs/09-deployment.md)
- [API reference](docs/10-api-reference.md)
- [Contributing](docs/CONTRIBUTING.md)
- [Security policy](SECURITY.md)

The repository also contains [`ARCHITECTURE.md`](./ARCHITECTURE.md), a short
design rationale for contributors.

## Examples

- [`examples/api_only`](./examples/api_only): HTTP API, SQL model, and routes
- [`examples/ssr_auth`](./examples/ssr_auth): sessions, templates, and auth
- [`examples/orm_demo`](./examples/orm_demo): direct database usage
- [`examples/multi_database`](./examples/multi_database): multiple database
  connections

## Verification

Run the complete test suite:

```bash
go test ./...
```

Race-test lifecycle and CLI changes:

```bash
go test -race ./pkg/engine ./cmd/astra
```

## License

Astra is distributed under the MIT license. See [`LICENSE`](./LICENSE).
