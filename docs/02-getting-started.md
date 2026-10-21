# Getting started

> Source files reviewed for this document: `cmd/astra/main.go`,
> `cmd/astra/main_test.go`, `internal/scaffold/tpl/*.tmpl`,
> `examples/api_only/main.go`, `examples/api_only/routes/api.go`,
> `pkg/engine/config/*`, and `pkg/engine/http/*`.

## Install the CLI

```bash
go install github.com/shauryagautam/Astra/cmd/astra@latest
astra help
```

The module path is exactly `github.com/shauryagautam/Astra/cmd/astra`.

## Create a service

```bash
astra new example.com/myservice --api-only
cd example.com/myservice
```

`astra new` runs `go mod init`, `go mod tidy`, and initializes a Git repository
when one does not already exist. Generated files are created exclusively; the
generator refuses to overwrite an existing path.

Use `--library` for a minimal reusable library. Library mode writes
`library.go` and does not create an HTTP application.

## Generated layout

```text
myservice/
├── main.go
├── wire.go
├── wire_gen.go
├── routes.go
├── README.md
├── Makefile
├── .env
├── database/
│   ├── migrations/
│   └── data/
├── handler/
├── model/
└── middleware/
```

The directories are ordinary Go packages. You can reorganize them without
changing Astra behavior.

## First route

The generated `routes.go` is in package `main`:

```go
func RegisterRoutes(r *http.Router) {
    r.Get("/health", func(c *http.Context) error {
        return c.JSON(map[string]string{"status": "UP"}, http.StatusOK)
    })
}
```

Add a handler as a normal Go function or method. The router's handler type is
defined by `pkg/engine/http`; standard `net/http` handlers can also be mounted
through the router's `Handle` method.

## Configuration

Create `.env` or export environment variables. The generated file contains
random `APP_KEY` and `JWT_SECRET` values, but production secrets should be
provided by the deployment environment.

The configuration loader reads, in increasing precedence:

1. `config/*.toml`
2. `config/*.yaml` and `config/*.yml`
3. Explicit `.env` paths passed to `config.Load`
4. The process environment

Use typed configuration:

```go
raw, err := config.Load(".env")
if err != nil {
    return err
}
cfg := config.LoadFromEnv(raw)
if err := cfg.Validate(); err != nil {
    return err
}
```

`Validate` requires `APP_KEY`, `JWT_SECRET` (or derives it from `APP_KEY`),
and `DATABASE_URL`. `ValidateProduction` additionally rejects debug mode,
disabled database SSL, and an unset request body limit.

## Run in development

```bash
astra key:generate
astra migrate up
astra dev
```

`astra dev` builds the current module into `bin/dev_app`, starts it, watches
source/template/environment files with `fsnotify`, and debounces changes for
150ms before rebuilding. It ignores `.git`, hidden directories, `vendor`,
`bin`, `tmp`, `node_modules`, `dist`, and `coverage`.

The development command expects `main.go` in the current directory.

## Use the library packages directly

You do not need the CLI or kernel to use the database, cache, auth, or HTTP
packages. For example, `database.Open` returns a usable `*database.DB`, and
`http.NewRouter` returns a router that can be passed to `net/http.Server`.
