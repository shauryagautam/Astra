# Contributing

> Source files reviewed for this document: `go.mod`, `.golangci.yml`,
> `CONTRIBUTING.md`, `cmd/astra/main_test.go`, package tests, integration tests,
> and benchmark files.

## Development setup

Install Go 1.26 or newer:

```bash
go version
go mod download
```

Docker is required for tests that start external services with Testcontainers.
Not every package test requires Docker; begin with the focused package test.

## Verification

Before opening a change:

```bash
gofmt -w path/to/changed.go
go test ./...
go test -race ./pkg/engine ./cmd/astra
go vet ./...
git diff --check
```

Run benchmarks when changing hot paths:

```bash
go test -bench=. ./benchmark/...
```

Use focused tests during iteration, then run the full suite. Add tests beside
the package they cover. Integration tests belong under `test/integration` or
the relevant package when they require package internals.

## Go standards

- Follow `gofmt`, `go vet`, and the repository linter configuration.
- Prefer constructors and explicit dependency injection.
- Keep interfaces small and define them near their consumers.
- Accept `context.Context` for cancellable I/O and long-running work.
- Return errors with operation context; do not silently ignore failures.
- Use `log/slog` for structured logs.
- Avoid package-global mutable state in new code.
- Preserve `net/http`, `database/sql`, and standard Go contracts.
- Document exported declarations with Go-style comments.

## API changes

When changing a public API:

1. Update package documentation and the relevant guide.
2. Add unit tests for success, failure, cancellation, and concurrency where
   applicable.
3. Update examples or CLI templates if they use the API.
4. Check generated scaffolds by building a temporary project.
5. Call out compatibility or migration impact in the changelog.

Do not reintroduce runtime service lookup, filename-based registration, or
framework-required directory layouts.

## Pull requests

Keep changes focused. Explain the problem, the design, compatibility effects,
and validation performed. Include reproduction steps for bug fixes. Security
issues must follow [`SECURITY.md`](../SECURITY.md) rather than being posted
publicly.

## Documentation

Documentation must describe the implementation that exists in the repository.
Mark incomplete or experimental integrations clearly. Prefer executable
examples with real package names and signatures over conceptual pseudocode.
