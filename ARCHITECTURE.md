# Astra architecture

Astra is a set of ordinary Go packages, not an application container or a
convention-driven framework. The application owns its composition root and
decides which packages it uses.

## Composition

Construct dependencies with constructors and pass them to the code that needs
them. Google Wire is used only to generate constructor calls; it is not a
runtime registry and it does not hide dependencies. Keep the generated
`wire_gen.go` file in the application and keep `wire.go` focused on the
application graph.

The CLI scaffold starts with `main.go`, `wire.go`, `routes.go`, and optional
packages such as `handler/`, `model/`, and `middleware/`. These directories are
examples, not required framework paths. Move or remove them as the application
grows.

## Lifecycle

`engine.App` is a lifecycle manager. A managed component implements:

```go
type Component interface {
    Start(context.Context) error
    Stop(context.Context) error
}
```

Components are constructed explicitly and added in dependency order.
Startup runs in addition order; shutdown runs in reverse order. Components
must honor the context passed to both methods, tolerate a partially completed
startup, and release only resources they own. `App` does not resolve services
by name and has no service locator.

Use `OnStart` and `OnStop` sparingly for application-level coordination. Prefer
a small component when a resource has a meaningful lifecycle.

## Package boundaries

Packages under `pkg/` are independently usable libraries. HTTP, database,
queue, cache, identity, storage, and observability code should accept interfaces
or concrete dependencies through constructors rather than reading global state.
The kernel is optional: a package can be used directly without booting an
`engine.App`.

Authentication adapters are implementations of an OAuth interface, not a
framework registration mechanism. Similarly, database migrations and data sets
are explicit inputs to a runner; file names and directories are not runtime
registries.

## Error and concurrency policy

Public operations return errors rather than logging and continuing with a
success-shaped result. Context cancellation and deadlines are part of every
long-running operation. Shared state is protected internally, while callers
retain ownership of constructed dependencies and decide how to synchronize
their use.

This deliberately favors boring Go: explicit wiring, small interfaces,
standard library semantics, and composition over conventions.
