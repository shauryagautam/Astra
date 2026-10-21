# Database and ORM

> Source files reviewed for this document: `pkg/database/engine.go`,
> `pkg/database/model.go`, `pkg/database/query_builder.go`,
> `pkg/database/transaction.go`, `pkg/database/relations.go`,
> `pkg/database/migration/runner.go`, `pkg/database/schema/*`,
> `pkg/database/standalone.go`, and database tests.

## Supported drivers

`database.Open` supports PostgreSQL (`pgx` stdlib), MySQL, SQLite, and the
Neon dialect. The driver is selected from `database.Config.Driver`; the CLI
uses `DB_DRIVER` or `DB_CONNECTION`.

```go
db, err := database.Open(database.Config{
    Driver:  "postgres",
    DSN:     os.Getenv("DATABASE_URL"),
    MaxOpen: 25,
    MaxIdle: 5,
})
if err != nil {
    return err
}
defer db.Close()
```

`Open` pings the database before returning. It closes the pool if that ping
fails. `DB.Pool`, `Stats`, `PingContext`, `SetMaxOpenConns`, and
`SetMaxIdleConns` expose pool operations when the underlying connection is a
`*sql.DB`.

## Models and metadata

Embedding `database.Model` is optional:

```go
type User struct {
    database.Model
    Email string `db:"email" orm:"guarded"`
    Name  string `db:"name"`
}
```

The `orm` tags currently include:

| Tag | Meaning |
| --- | --- |
| `primary_key` | Identifies the primary key |
| `auto_increment` | Omits the value during insert |
| `soft_delete` | Uses `DeletedAt` for delete behavior |
| `guarded` | Excludes the field from create/map update operations |
| `column:name` | Overrides the column mapping |

The model resolver derives table and column metadata from Go types and tags.
For explicit table selection, use `Table`.

## Queries

Use the generic builder with a context:

```go
users, err := database.QueryContext[User](ctx, db).
    Where("active", "=", true).
    OrderBy("created_at", "DESC").
    Limit(50).
    AllSlice()
```

Common builder operations include `Where`, `OrWhere`, `WhereIn`,
`WhereNull`, `WhereNotNull`, `WhereRaw`, `OrderBy`, `Limit`, `Offset`,
`With`, `WithTrashed`, `Scope`, `Paginate`, and `Chunk`.

Terminal operations include `First`, `Find`, `All`, `Count`, `Exists`,
`Create`, `Update`, `Delete`, `ForceDelete`, `Restore`, `Paginate`, and
`Chunk`. Confirm exact return types in the package documentation because
generic result shapes differ between operations.

Columns, table names, operators, and sort directions are validated. Values
are bound as SQL parameters. `WhereRaw` is intentionally an escape hatch:
the SQL fragment is trusted, while its arguments are still passed separately.

## Soft deletes and mass assignment

Models with a `DeletedAt *time.Time` field tagged `orm:"soft_delete"` are
filtered from normal queries. `WithTrashed` includes deleted rows. Delete
updates the timestamp; `ForceDelete` permanently removes the row, and
`Restore` clears the timestamp.

Fields tagged `guarded` are excluded from create and map-based update paths.
This is not a replacement for request validation; validate input at the HTTP
boundary and use typed input structures.

## Relations and eager loading

The package defines `HasOne`, `HasMany`, `BelongsTo`, `ManyToMany`,
`MorphTo`, and `MorphMany` wrappers. Relation metadata can be supplied with
ORM tags; otherwise the resolver uses documented snake-case conventions.

Eager loading is requested with `With`:

```go
orders, err := database.QueryContext[Order](ctx, db).
    With("Customer", "Lines").
    AllSlice()
```

Many-to-many loading uses a pivot table. Explicit pivot and key metadata is
preferred in production because convention-derived names are only defaults.

## Transactions

`DB.Transaction` begins a transaction, injects a transaction-bound `*DB` into
the callback context, rolls back on returned errors or panics, and commits on
success:

```go
err := db.Transaction(ctx, func(txCtx context.Context) error {
    if _, err := database.QueryContext[User](txCtx, db).Create(&user); err != nil {
        return err
    }
    return nil
})
```

Nested transactions use SQL savepoints. Queries created with `QueryContext`
follow the transaction database stored in the context.

## Migrations

`database/migration.Runner` reads `.sql` files from a directory in lexical
order. Files contain `-- +migrate Up` and `-- +migrate Down` sections. The
runner stores versions, batches, timestamps, and SHA-256 checksums in
`schema_migrations`.

```go
runner := migration.NewRunner(
    db.Pool(),
    db.Dialect(),
    "database/migrations",
    nil,
)
if err := runner.Run(ctx); err != nil {
    return err
}
```

PostgreSQL and MySQL advisory locks prevent concurrent migration runs. SQLite
does not use advisory locking. A changed applied migration fails checksum
validation. `Rollback` and `Status` are also available.

## Standalone use

The database package does not require `engine.App`, Wire, or the CLI. Use
`database.Open`, `database.Query`, schema builders, transactions, and the
migration runner directly from a normal Go program.

## Pool and query instrumentation

`Config` supports query logging, slow-query thresholds, tracing, query hooks,
and an auditor. `DB.SetQueryHook` can feed an application-specific dashboard.
Avoid logging SQL arguments that contain secrets or personal data.

## Limitations

The ORM is reflection-based for model metadata and scanning. It is not a
replacement for sqlc when compile-time SQL generation is the primary goal.
Database integration tests use SQLite by default and Testcontainers for
selected external services; production deployments must still test their
chosen driver and schema.
