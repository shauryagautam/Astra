// Package contracts defines all interfaces (contracts) for the Adonis framework.
// Following AdonisJS convention, contracts are defined first and implementations
// are provided by concrete types in the app/ package.
//
// Go Idiom Note: AdonisJS uses TypeScript's type system and decorators for IoC.
// In Go, we use string-keyed registries with `any` type and factory functions
// to achieve equivalent dynamic binding/resolution. Type assertions are used
// at the call site to recover concrete types.
package contracts

// BindingFactory is a factory function that receives the container
// and returns a new instance. This replicates AdonisJS's container
// callback pattern: container.bind('key', (app) => new Service(app))
type BindingFactory func(container ContainerContract) (any, error)

// ContainerContract defines the IoC (Inversion of Control) container interface.
// This is the Go equivalent of @adonisjs/fold's Container class.
//
// The container is the backbone of the framework — it manages all service
// bindings, singletons, and dependency resolution.
//
// Usage mirrors AdonisJS:
//
//	container.Bind("Adonis/Src/Route", func(c ContainerContract) (any, error) {
//	    return NewRouter(), nil
//	})
//
//	router := container.Use("Adonis/Src/Route").(RouterContract)
type ContainerContract interface {
	// Bind registers a factory function for a given namespace.
	// Each call to Make/Use will invoke the factory, producing a new instance.
	// Mirrors: container.bind('namespace', callback)
	Bind(namespace string, factory BindingFactory)

	// Singleton registers a factory that is invoked only once.
	// Subsequent calls to Make/Use return the cached instance.
	// Mirrors: container.singleton('namespace', callback)
	Singleton(namespace string, factory BindingFactory)

	// Make resolves a binding by namespace, invoking its factory.
	// Returns an error if the namespace is not registered.
	// Mirrors: container.make('namespace')
	Make(namespace string) (any, error)

	// MustMake resolves a binding, panicking on error.
	// Convenient for boot-time resolution where failure is fatal.
	MustMake(namespace string) any

	// Use is an alias for MustMake — resolves a binding or panics.
	// This matches AdonisJS's use() import pattern:
	//   const Route = use('Adonis/Src/Route')
	Use(namespace string) any

	// HasBinding checks if a namespace has been registered.
	HasBinding(namespace string) bool

	// Alias creates an alias for an existing namespace.
	// Mirrors: container.alias('shortName', 'Full/Namespace/Path')
	Alias(alias string, namespace string)

	// Fake replaces a binding with a fake/mock for testing.
	// Mirrors: container.fake('namespace', callback)
	Fake(namespace string, factory BindingFactory)

	// Restore removes a fake and restores the original binding.
	Restore(namespace string)

	// Call resolves all dependencies and calls the given function.
	// The function signature determines which bindings to inject.
	// This is a simplified version of AdonisJS's auto-injection.
	Call(fn any, args ...any) ([]any, error)

	// RegisterType maps a reflect.Type to a namespace for auto-injection.
	// Mirrors: container.registerType(Router, 'Adonis/Core/Route')
	RegisterType(kind any, namespace string)

	// WithBindings executes a callback with specific bindings resolved.
	// Useful for scoped resolution within request lifecycles.
	WithBindings(namespaces []string, callback func(bindings map[string]any) error) error
}
