package app

import (
	"fmt"
	"sync"

	"github.com/shaurya/adonis/contracts"
)

// Container is the concrete implementation of the IoC Container.
// It replicates @adonisjs/fold's Container class.
//
// Go Idiom Note: AdonisJS uses TypeScript's type system to automatically
// resolve constructor dependencies. Go lacks this capability, so we use
// string-keyed registries with factory functions. Type assertions at the
// call site recover concrete types. This trades compile-time safety for
// the same runtime flexibility AdonisJS provides.
type Container struct {
	mu         sync.RWMutex
	bindings   map[string]contracts.BindingFactory
	singletons map[string]contracts.BindingFactory
	instances  map[string]any
	aliases    map[string]string
	fakes      map[string]contracts.BindingFactory
}

// NewContainer creates a new IoC container.
func NewContainer() *Container {
	return &Container{
		bindings:   make(map[string]contracts.BindingFactory),
		singletons: make(map[string]contracts.BindingFactory),
		instances:  make(map[string]any),
		aliases:    make(map[string]string),
		fakes:      make(map[string]contracts.BindingFactory),
	}
}

// Bind registers a factory function for a given namespace.
// Each call to Make/Use will invoke the factory, producing a new instance.
func (c *Container) Bind(namespace string, factory contracts.BindingFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bindings[namespace] = factory
}

// Singleton registers a factory that is invoked only once.
// Subsequent calls return the cached instance.
func (c *Container) Singleton(namespace string, factory contracts.BindingFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.singletons[namespace] = factory
}

// resolveNamespace resolves any alias chain to the real namespace.
func (c *Container) resolveNamespace(namespace string) string {
	if target, ok := c.aliases[namespace]; ok {
		return c.resolveNamespace(target)
	}
	return namespace
}

// Make resolves a binding by namespace, invoking its factory.
func (c *Container) Make(namespace string) (any, error) {
	c.mu.RLock()
	resolved := c.resolveNamespace(namespace)

	// Check for fakes first (testing)
	if factory, ok := c.fakes[resolved]; ok {
		c.mu.RUnlock()
		return factory(c)
	}

	// Check for cached singleton instances
	if instance, ok := c.instances[resolved]; ok {
		c.mu.RUnlock()
		return instance, nil
	}

	// Check for singleton factory
	if factory, ok := c.singletons[resolved]; ok {
		c.mu.RUnlock()

		// IMPORTANT: We release the lock BEFORE calling the factory
		// to avoid deadlocks if the factory calls Make/Use.
		// However, we must ensure we don't resolve the same singleton multiple times
		// needlessly in high-concurrency scenarios.
		instance, err := factory(c)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve singleton '%s': %w", namespace, err)
		}

		c.mu.Lock()
		defer c.mu.Unlock()

		// Double-check if someone else resolved it while we were busy
		if existing, ok := c.instances[resolved]; ok {
			return existing, nil
		}

		c.instances[resolved] = instance
		return instance, nil
	}

	// Check for regular binding
	if factory, ok := c.bindings[resolved]; ok {
		c.mu.RUnlock()
		// Releasing lock before calling factory for bindings too
		instance, err := factory(c)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve binding '%s': %w", namespace, err)
		}
		return instance, nil
	}

	c.mu.RUnlock()
	return nil, fmt.Errorf("no binding registered for namespace '%s'", namespace)
}

// MustMake resolves a binding, panicking on error.
func (c *Container) MustMake(namespace string) any {
	instance, err := c.Make(namespace)
	if err != nil {
		panic(err)
	}
	return instance
}

// Use is an alias for MustMake — resolves a binding or panics.
// Mirrors AdonisJS: const Route = use('Adonis/Src/Route')
func (c *Container) Use(namespace string) any {
	return c.MustMake(namespace)
}

// HasBinding checks if a namespace has been registered.
func (c *Container) HasBinding(namespace string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	resolved := c.resolveNamespace(namespace)
	_, inBindings := c.bindings[resolved]
	_, inSingletons := c.singletons[resolved]
	return inBindings || inSingletons
}

// Alias creates an alias for an existing namespace.
func (c *Container) Alias(alias string, namespace string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.aliases[alias] = namespace
}

// Fake replaces a binding with a fake/mock for testing.
func (c *Container) Fake(namespace string, factory contracts.BindingFactory) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resolved := c.resolveNamespace(namespace)
	c.fakes[resolved] = factory
}

// Restore removes a fake and restores the original binding.
func (c *Container) Restore(namespace string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	resolved := c.resolveNamespace(namespace)
	delete(c.fakes, resolved)
}

// Call resolves dependencies and calls the given function.
// For simplicity, this passes additional args directly.
func (c *Container) Call(fn any, args ...any) ([]any, error) {
	// In Go, we can't do full auto-injection like AdonisJS.
	// This is a simplified version that calls the function with provided args.
	// For full reflection-based injection, use the reflect package.
	return nil, fmt.Errorf("Call() is not yet implemented — use Make() for explicit resolution")
}

// WithBindings resolves multiple bindings and passes them to a callback.
func (c *Container) WithBindings(namespaces []string, callback func(bindings map[string]any) error) error {
	bindings := make(map[string]any, len(namespaces))
	for _, ns := range namespaces {
		instance, err := c.Make(ns)
		if err != nil {
			return fmt.Errorf("WithBindings failed to resolve '%s': %w", ns, err)
		}
		bindings[ns] = instance
	}
	return callback(bindings)
}

// Ensure Container implements ContainerContract at compile time.
var _ contracts.ContainerContract = (*Container)(nil)
