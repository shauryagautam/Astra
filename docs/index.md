# Astra

Astra is a production-grade full-stack framework for Go built around one opinionated idea: high productivity comes from compile-time guarantees, not runtime surprises.

## Philosophy

Astra exists for teams that want a framework-shaped developer experience without giving up the properties that make Go useful in production. The framework is deliberately explicit. Dependencies are wired ahead of time, routes are declared with standard library primitives, and the application lifecycle is visible rather than hidden behind reflection or container magic.

The result is a codebase that is easier to reason about, easier to test, and easier to operate when the system is under load.

### Core Pillars

1. Zero Magic with Wire. Astra uses static dependency injection so wiring mistakes surface at build time.
2. Go 1.23+ idioms. Astra leans into generics, `iter.Seq2`, `net/http`, `slog`, and other standard library-first patterns.
3. Production-ready by default. Observability, resilience, graceful shutdown, database migrations, and frontend delivery are part of the baseline, not optional extras.

> [!NOTE]
> Astra does not ask you to trust hidden runtime behavior. If something important is misconfigured, the failure should happen during compilation or boot, not after traffic arrives.

> [!TIP]
> Read the chapters in order. Astra is designed as a stack: lifecycle first, architecture second, then security, persistence, frontend delivery, observability, testing, and finally deployment.

## Roadmap

### [01. Foundation](./01-foundation.md)
Install the CLI, scaffold an app, understand the project layout, and learn how the kernel boots and shuts down.

### [02. Architecture](./02-architecture.md)
Learn how static dependency injection, service providers, and the router fit together without runtime service lookup.

### [03. Security](./03-security.md)
Work with guards, password hashing, RBAC middleware, and policy-based authorization.

### [04. Persistence](./04-persistence.md)
Use the generics-based ORM, stream rows with `iter.Seq2`, run nested transactions, and manage migrations and multiple databases.

### [05. Frontend](./05-frontend.md)
Integrate Vite for development and production, then add SSR, SSE, and WebSockets for interactive full-stack apps.

### [06. Observability](./06-observability.md)
Wire in structured logging, OpenTelemetry, distributed circuit breakers, and rate limiting that behaves the same in every environment.

### [07. Testing](./07-testing.md)
Write expressive HTTP tests, swap dependencies through Wire, and run real integration suites with testcontainers-go.

### [08. Deployment](./08-deployment.md)
Build a production image, bundle frontend assets, and shut the system down gracefully when the platform asks.

## References

- **[Release Management](../RELEASE.md)** - Versioning and stability guarantees.
- **[Generation Contract](../cmd/astra/gen/CONTRACT.md)** - How Go maps to TypeScript/Zod.
- **[Security Policy](../SECURITY.md)** - Reporting vulnerabilities.

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
		return nil
	})

	app.OnStop(func(ctx context.Context) error {
		return nil
	})

	if err := app.Run(); err != nil {
		panic(err)
	}
}
```
