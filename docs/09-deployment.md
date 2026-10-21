# Deployment

> Source files reviewed for this document: `internal/scaffold/tpl/Dockerfile.tmpl`,
> `internal/scaffold/tpl/fly.toml.tmpl`, `examples/*/Dockerfile`,
> `pkg/engine/app.go`, `pkg/engine/config/schema.go`,
> `pkg/engine/http/server.go`, and `SECURITY.md`.

## Build

Use a reproducible multi-stage build for a service:

```dockerfile
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/service .

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/service /service
ENTRYPOINT ["/service"]
```

The exact base image and CGO setting depend on the database drivers and
operating-system requirements of the application. Verify the generated binary
in the target image.

## Environment

At minimum, production configuration normally includes:

- `APP_ENV=production`
- `APP_KEY` with at least 32 characters
- `JWT_SECRET` with at least 32 characters, unless derived from `APP_KEY`
- `DATABASE_URL`
- `DB_CONNECTION` or `DB_DRIVER`
- `APP_MAX_BODY_SIZE` greater than zero
- `PORT` and `HOST` as required by the platform

The full typed configuration surface is documented in
[API reference](10-api-reference.md). Supply secrets from the platform secret
manager rather than committing `.env`.

Run configuration validation before starting:

```go
if err := cfg.Validate(); err != nil {
    return err
}
if cfg.App.Environment == "production" {
    if err := cfg.ValidateProduction(); err != nil {
        return err
    }
}
```

## Database migrations

Run migrations as a deployment step or a controlled release job:

```bash
astra migrate up
```

The runner uses advisory locks for PostgreSQL and MySQL. SQLite does not
provide the same cross-process advisory lock behavior. Keep migration files
immutable after they have run; checksum verification rejects modifications.

## Kubernetes and graceful shutdown

Pass the pod's cancellation context to `engine.NewWithContext`. Register the
HTTP server and other resources as components. Kubernetes should send
`SIGTERM`, allow a termination grace period longer than the application's
shutdown timeout, and stop routing traffic before termination.

Each component must honor the context passed to `Stop`. Queue workers drain
in-flight jobs until the shutdown context expires. Database pools, Redis
clients, schedulers, and servers should be stopped in reverse dependency order.

The engine installs signal handling for `SIGTERM` and interrupt. External
supervisors still control the process lifetime; do not assume signal handling
replaces readiness or pre-stop coordination.

## Health endpoints

Expose liveness and readiness separately. `ReadyHandler` returns `"OK"` and
does not check dependencies. Use `HealthHandler` or
`observability/health.HealthChecker` for dependency-aware status and return
503 when a required dependency fails.

## Docker Compose and Fly.io

The repository includes deployment templates and example Compose/Fly files.
They are starting points, not a guarantee that your production topology is
correct. Review ports, health probes, secret injection, TLS termination,
database backups, Redis persistence, and resource limits.

## Production checklist

- [ ] Validate configuration before accepting requests.
- [ ] Set `APP_MAX_BODY_SIZE`, timeouts, and rate limits.
- [ ] Use TLS at the edge and secure cookie settings.
- [ ] Run migrations from a controlled release process.
- [ ] Configure database pool limits for the deployment size.
- [ ] Configure structured log collection and trace/metric exporters.
- [ ] Protect `/metrics`, dashboards, and administrative endpoints.
- [ ] Use readiness checks that reflect actual dependencies.
- [ ] Test graceful shutdown under active HTTP and queue load.
- [ ] Back up databases and define restore procedures.
- [ ] Rotate JWT and application keys with an explicit migration plan.
