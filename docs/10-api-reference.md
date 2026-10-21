# API reference

> Source files reviewed for this document: exported declarations across
> `pkg/engine`, `pkg/engine/config`, `pkg/engine/http`, `pkg/database`,
> `pkg/database/migration`, `pkg/identity/auth`, `pkg/identity/rbac`,
> `pkg/queue`, `pkg/cache`, `pkg/storage`, `pkg/mail`, and
> `pkg/observability`.

This is a package-oriented index, not a substitute for
[pkg.go.dev](https://pkg.go.dev/github.com/shauryagautam/Astra).

## `pkg/engine`

Important types and functions:

- `App`
- `Component`
- `New`, `NewWithContext`
- `RegisterComponent`, `Boot`, `Run`, `Start`, `Shutdown`
- `OnStart`, `OnStop`
- `HealthCheck`, `HealthCheckFunc`
- `RegisterHealthCheck`, `GetHealthChecks`
- `Recover`

`App` owns lifecycle state and signal-aware contexts. It does not resolve
services.

## `pkg/engine/config`

- `Config`
- `Load`
- `Get[T]`
- `String`, `Int`, `Int32`, `Bool`, `Duration`
- `LoadEnv`, `LoadEnvOverride`
- `EnvGet`, `EnvGetOrFail`, `EnvGetInt`, `EnvGetBool`, `EnvGetDuration`,
  `EnvGetFloat`
- `AstraConfig`
- `LoadFromEnv`
- `Validate`, `ValidateProduction`, `ValidateRequired`

`AstraConfig` groups `App`, `Database`, `Redis`, `Auth`, `OAuth2`, `Storage`,
`Mail`, `Queue`, `Telemetry`, `Assets`, and `WS` settings.

## `pkg/engine/http`

Major APIs include:

- `Router`, `NewRouter`
- `Server`, `NewServer`
- `Context`, `FromRequest`
- `HandlerFunc`
- route methods such as `Get`, `Post`, and `Handle`
- `SecureHeaders`, CSRF, CORS, rate-limit, timeout, recovery, auth, RBAC,
  session, upload, validation, tracing, and compression middleware
- `HealthHandler`, `ReadyHandler`, `PrometheusHandler`
- rendering, SSE, WebSocket, and client helpers

The package uses `net/http` handler values at integration boundaries.

## `pkg/database`

- `Config`, `DB`, `Open`, `New`
- `Query[T]`, `QueryContext[T]`, `NewQueryBuilder[T]`
- `QueryBuilder[T]`
- `Model`, `Relation[T]`, `HasOne`, `HasMany`, `BelongsTo`, `ManyToMany`,
  `MorphTo`, `MorphMany`
- `WithTenantID`, `TenantIDFromContext`
- `DB.Transaction`, `WithContext`, `FromContext`
- pagination, schema, audit, hooks, metrics, and scanner APIs

Use `database.Open` for the normal path and `database/migration.Runner` for
schema migration.

## `pkg/database/migration`

- `Runner`
- `MigrationRecord`
- `NewRunner`
- `Setup`, `Run`, `Rollback`, `Status`

Migration files are SQL files with `-- +migrate Up` and `-- +migrate Down`
sections.

## `pkg/identity/auth`

- `JWTManager`, `NewJWTManager`
- `IssueTokenPair`, `Verify`, `Refresh`, revocation helpers
- password hashing/checking APIs
- `SessionDriver`, `RedisSessionDriver`
- `OAuth2Provider`, `OAuth2Manager`, `OAuth2ProviderConfig`
- provider adapters under `pkg/identity/auth/providers`
- `TOTPManager`
- guards and authentication helpers

## `pkg/identity/rbac`

- `RBAC`, `NewRBAC`
- `Storage`, `Cache`, `Logger`
- `Permission`, `Role`, `UserRole`, `Policy`
- `AccessRequest`, `AccessResult`
- `CheckAccess`, `Allows`

## `pkg/queue`

- `Job`, `BaseJob`, `TypedJob[T]`, `GenericJob[T]`
- `RedisQueue`, `RedisDispatcher`, `RedisWorker`
- `NewRedisQueue`, `NewRedisDispatcher`, `NewRedisWorker`
- `FailedJobManager`, `RedisFailedJobsStore`
- `Scheduler`, `ScheduledJob`
- `WorkerMetrics`

Redis is a required dependency for these implementations.

## `pkg/cache`

Includes memory and Redis stores plus lock interfaces and implementations.
`pkg/cache/standalone.go` provides standalone construction helpers.

## `pkg/observability`

- `observability/trace`: `Init`, `GetTracer`, `StartSpan`
- `observability/metrics`: `Init`, `GetMeter`
- `observability/health`: `HealthChecker`, `HealthReport`,
  `DiskSpaceCheck`, `LatencyCheck`
- `observability/audit`: `Auditor`, `AuditEvent`
- `observability/fault_tolerance`: local and distributed circuit breakers

## Other public packages

- `pkg/storage`: local and S3 storage
- `pkg/mail`: mail interfaces and mailers
- `pkg/graphql`: GraphQL schema, handler, directives, dataloader helpers
- `pkg/i18n`: translation support
- `pkg/crypto`: AES-256-GCM encryption
- `pkg/dto`: DTO conversion helpers
- `pkg/errors`: application error types
- `pkg/ws`: WebSocket support
- `pkg/engine/json`: JSON encoder/decoder abstraction

For complete signatures, defaults, and deprecation details, read package
comments and generated Go documentation.
