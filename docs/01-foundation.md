# 01. Foundation

This chapter covers the parts you need before you write a single route: install the CLI, scaffold the project, understand the directory boundaries, and learn how the Astra kernel boots and shuts down.

## Why this matters

The fastest way to build a maintainable Astra app is to start with the framework’s shape instead of fighting it. The CLI generates the right seams, the directory structure keeps application code explicit, and the lifecycle gives you one place to start and stop long-lived services.

> [!TIP]
> Treat the scaffold as the contract. Astra’s defaults are meant to be edited, not replaced with ad hoc structure.

## Install the CLI

Install the Astra command line tool with the Go toolchain:

```bash
go install github.com/shauryagautam/Astra/cmd/astra@latest
```

Make sure your Go bin directory is on `PATH` so the `astra` command is available in every shell session.

## Scaffold a new app

Create a project with the CLI:

```bash
astra new myapp
cd myapp
```

The generator creates a production-friendly layout with a clear boundary between application code, framework glue, frontend assets, and database concerns.

### Canonical project structure

```text
myapp/
├── app/
│   ├── handler/    # Request handlers
│   ├── schema/     # Data models & validation
│   └── jobs/       # Background tasks
├── database/
│   ├── migrations/ # Database schema versions
│   └── seeders/    # Test data
├── routes/
│   └── api.go/     # Route registration
├── shared/
│   └── astra-client/ # Generated TS/Zod client
├── main.go         # Composition root
├── wire.go         # DI configuration
└── wire_gen.go     # Generated DI code
```

Use `app/` for application-specific code, `routes/` for route registration, `database/` for migrations and seed data, and `shared/` for the bridge between backend and frontend.

> [!NOTE]
> `wire.go` is the source file you edit. `wire_gen.go` is generated code and should never be hand-edited.

## The application kernel

Astra’s `App` type is the kernel for the entire process. It owns the config, logger, container, health checks, and the lifecycle hooks that start and stop long-running services.

The important design choice is that the kernel is not a service locator. It is a composition root. Dependencies are registered up front, then passed into the parts of the app that need them.

### Boot lifecycle

When the kernel boots, the order matters:

1. `engine.New(...)` creates the kernel and its base context.
2. Providers are registered with `app.RegisterProvider(...)`.
3. `App.Boot()` calls each provider’s `Register` method.
4. The same providers then receive `Boot` so they can start connections, register routes, or install listeners.
5. `OnStart` hooks run after providers are ready.
6. `Run()` blocks until the process receives `SIGINT` or `SIGTERM`.
7. `OnStop` hooks run in reverse order so teardown follows the dependency chain.

> [!WARNING]
> Put cleanup work in `OnStop`, not in random handlers or deferred goroutines. Shutdown should be deterministic.

## `OnStart` and `OnStop`

Use `OnStart` for work that must happen only after the kernel is fully booted, such as starting background workers or logging startup metadata.

Use `OnStop` for resource cleanup, such as closing database pools, flushing telemetry, stopping queues, or draining background jobs.

Because `OnStop` runs in reverse registration order, you can register dependent resources in the same order you created them and still tear them down safely.

## Copy-Paste Example

```go
package main

import (
	"context"
	"log/slog"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
)

func main() {
	cfg := config.MustLoad()
	app := engine.New(&cfg.Astra, cfg, slog.Default())

	app.OnStart(func(ctx context.Context) error {
		app.Logger().Info("startup complete")
		return nil
	})

	app.OnStop(func(ctx context.Context) error {
		app.Logger().Info("shutdown complete")
		return nil
	})

	if err := app.Run(); err != nil {
		panic(err)
	}
}
```

---

**Next Chapter: [02. Architecture](./02-architecture.md)**
