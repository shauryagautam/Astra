// Package engine provides the lifecycle manager and shared contracts for an
// Astra application.
//
// Components are constructed by the application (usually with Wire) and
// added explicitly with App.AddComponent. A managed component must
// implement Start(context.Context) error and Stop(context.Context) error.
// Start runs in addition order and must not return before the component is
// ready. Stop runs in reverse order, must honor its context, and must be safe
// after a partial start. The App owns no service registry and performs no
// implicit dependency resolution.
package engine
