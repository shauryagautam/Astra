# CLI reference

> Source files reviewed for this document: `cmd/astra/main.go`,
> `cmd/astra/main_test.go`, and `internal/scaffold/tpl/*.tmpl`.

Install:

```bash
go install github.com/shauryagautam/Astra/cmd/astra@latest
```

The CLI returns exit code `0` on success, `1` on command failure, and `2` for
usage or unknown-command errors.

## `astra new`

```bash
astra new example.com/myservice
astra new example.com/myapi --api-only
astra new example.com/shared --library
```

Creates the module, runs `go mod tidy`, writes a migration and application
templates, and initializes Git if necessary. `--api-only` changes generated
README/routes comments; it does not create a separate runtime mode.

## `astra dev`

```bash
astra dev
```

Requires `main.go` in the current directory. It builds `bin/dev_app`, runs the
binary, watches Go/HTML/template/environment files, ignores generated and
hidden directories, debounces changes for 150ms, and gracefully terminates
the child process before rebuilding.

## Generators

```bash
astra make:controller users
astra make:middleware request_id
astra make:model user_profile
astra make:migration add_index_to_users
```

Generators write to `handler/`, `middleware/`, `model/`, and
`database/migrations/` when those directories exist. Otherwise they write in
the current package. Existing generated files are never overwritten.

Available templates in the repository include controller, middleware, model,
migration, handler, job, listener, mailer, policy, rule, schema, and event
templates. The current command dispatcher exposes controller, middleware,
model, and migration generators.

## Migrations

```bash
astra migrate up
astra migrate down
astra migrate status
astra migrate create add_orders
```

The commands read `.env` and use `DB_DRIVER`/`DB_CONNECTION` plus
`DB_DSN`/`DATABASE_URL`. `create` writes a timestamped SQL file. `up`, `down`,
and `status` require a reachable configured database.

## Data sets

```bash
astra seed
```

Loads sorted `.sql` files from `database/data`. If the directory does not
exist, the command reports that no data sets were found and succeeds. Files
are executed as SQL; they are not registered Go functions.

## Keys

```bash
astra key:generate
astra key:rotate
```

Both commands write a random 32-byte base64 key to `.env` with mode `0600`.
`key:generate` appends `APP_KEY` if absent; `key:rotate` replaces it. Rotating
`APP_KEY` can invalidate sessions or encrypted data depending on application
usage; plan key migration explicitly.

## Doctor

```bash
astra doctor
```

Checks that `go` and `git` are available and that the current directory has a
`go.mod`. It does not connect to databases, Redis, or external telemetry.

## Help and errors

```bash
astra help
astra unknown-command
```

Errors are written through the injected structured logger in the command
implementation. Library callers cannot invoke `main`; use the command binary
for exit-code behavior.
