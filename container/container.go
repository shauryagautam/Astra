// Package container provides a type-safe inversion of control (IoC) container.
// It supports singleton and transient bindings, interface-to-implementation
// mapping, and circular dependency detection.
package container

import (
	"fmt"
	"reflect"
	"sync"
)

// BindingType defines how a service is instantiated.
type BindingType int

const (
	// TypeSingleton ensures only one instance is created and shared.
	TypeSingleton BindingType = iota
	// TypeTransient creates a new instance on every resolution.
	TypeTransient
)

// Binding holds the definition of a registered service.
type Binding struct {
	Resolver any
	Type     BindingType
	Instance any
	// AdonisJS-inspired features
	Tags     []string
	Priority int // For dependency injection order
}

// Container is a thread-safe registry for application services.
type Container struct {
	mu       sync.RWMutex
	bindings map[reflect.Type]*Binding
	stack    []reflect.Type // For circular dependency detection
	// AdonisJS-inspired optimizations
	tagIndex map[string][]reflect.Type // Fast tag-based lookup
	cache    map[reflect.Type]any      // Resolution cache for ultra-fast performance
}

// New creates a new, empty Container with AdonisJS-inspired optimizations.
func New() *Container {
	return &Container{
		bindings: make(map[reflect.Type]*Binding),
		stack:    make([]reflect.Type, 0),
		tagIndex: make(map[string][]reflect.Type),
		cache:    make(map[reflect.Type]any),
	}
}

// Singleton registers a service as a singleton. The resolver can be an instance
// or a function that returns an instance and optionally an error.
func (c *Container) Singleton(resolver any) {
	c.bind(resolver, TypeSingleton)
}

// Transient registers a service as transient. The resolver must be a function
// that returns a new instance on each call.
func (c *Container) Transient(resolver any) {
	c.bind(resolver, TypeTransient)
}

func (c *Container) bind(resolver any, bType BindingType) {
	c.bindWithOptions(resolver, bType, BindingOptions{})
}

// BindingOptions for AdonisJS-inspired service configuration
type BindingOptions struct {
	Tags     []string
	Priority int
}

// SingletonWithOptions registers a service as singleton with options
func (c *Container) SingletonWithOptions(resolver any, opts BindingOptions) {
	c.bindWithOptions(resolver, TypeSingleton, opts)
}

// TransientWithOptions registers a service as transient with options
func (c *Container) TransientWithOptions(resolver any, opts BindingOptions) {
	c.bindWithOptions(resolver, TypeTransient, opts)
}

func (c *Container) bindWithOptions(resolver any, bType BindingType, opts BindingOptions) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var t reflect.Type
	if reflect.TypeOf(resolver).Kind() == reflect.Func {
		t = reflect.TypeOf(resolver).Out(0)
	} else {
		t = reflect.TypeOf(resolver)
	}

	binding := &Binding{
		Resolver: resolver,
		Type:     bType,
		Tags:     opts.Tags,
		Priority: opts.Priority,
	}

	c.bindings[t] = binding

	// Update tag index for ultra-fast tag-based lookup
	for _, tag := range opts.Tags {
		c.tagIndex[tag] = append(c.tagIndex[tag], t)
	}
}

// Resolve retrieves a service of type T from the container with ultra-fast caching and recursive resolution.
func Resolve[T any](c *Container) (T, error) {
	var zero T
	t := reflect.TypeOf((*T)(nil)).Elem()

	val, err := c.resolveByType(t)
	if err != nil {
		return zero, err
	}

	return val.(T), nil
}

// resolveByType helper for dependency injection with recursive support
func (c *Container) resolveByType(t reflect.Type) (any, error) {
	// Fast path: check cache first for singletons
	c.mu.RLock()
	if cached, exists := c.cache[t]; exists {
		c.mu.RUnlock()
		return cached, nil
	}
	c.mu.RUnlock()

	c.mu.Lock()
	// Circular dependency check
	for _, seen := range c.stack {
		if seen == t {
			c.mu.Unlock()
			return nil, fmt.Errorf("container: circular dependency detected for %v", t)
		}
	}
	c.stack = append(c.stack, t)
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		c.stack = c.stack[:len(c.stack)-1]
		c.mu.Unlock()
	}()

	c.mu.RLock()
	binding, ok := c.bindings[t]
	c.mu.RUnlock()

	if !ok {
		// Try interface binding if T is an interface
		if t.Kind() == reflect.Interface {
			c.mu.RLock()
			for bt, b := range c.bindings {
				if bt.Implements(t) {
					binding = b
					ok = true
					break
				}
			}
			c.mu.RUnlock()
		}
	}

	if !ok {
		return nil, fmt.Errorf("container: no binding found for %v", t)
	}

	if binding.Type == TypeSingleton && binding.Instance != nil {
		// Cache the singleton for ultra-fast future access
		c.mu.Lock()
		c.cache[t] = binding.Instance
		c.mu.Unlock()
		return binding.Instance, nil
	}

	// Resolve the instance
	var instance any
	if reflect.TypeOf(binding.Resolver).Kind() == reflect.Func {
		fn := reflect.ValueOf(binding.Resolver)
		// Support dependency injection via function parameters
		var args []reflect.Value
		if fn.Type().NumIn() > 0 {
			// Auto-resolve dependencies recursively
			for i := 0; i < fn.Type().NumIn(); i++ {
				argType := fn.Type().In(i)
				if argType == reflect.TypeOf(c) {
					args = append(args, reflect.ValueOf(c))
				} else {
					// Recursively resolve the dependency
					dep, err := c.resolveByType(argType)
					if err != nil {
						return nil, fmt.Errorf("failed to resolve dependency %v for %v: %w", argType, t, err)
					}
					args = append(args, reflect.ValueOf(dep))
				}
			}
		}

		results := fn.Call(args)
		instance = results[0].Interface()
		if len(results) > 1 && !results[1].IsNil() {
			return nil, results[1].Interface().(error)
		}
	} else {
		instance = binding.Resolver
	}

	if binding.Type == TypeSingleton {
		c.mu.Lock()
		binding.Instance = instance
		c.cache[t] = instance // Cache for ultra-fast access
		c.mu.Unlock()
	}

	return instance, nil
}

// MustResolve is like Resolve but panics on error.
func MustResolve[T any](c *Container) T {
	res, err := Resolve[T](c)
	if err != nil {
		panic(err)
	}
	return res
}

// Tagged retrieves all services with the given tag for ultra-fast lookup
func Tagged(c *Container, tag string) []any {
	c.mu.RLock()
	defer c.mu.RUnlock()

	types, exists := c.tagIndex[tag]
	if !exists {
		return nil
	}

	services := make([]any, len(types))
	for i, t := range types {
		binding := c.bindings[t]
		if binding.Type == TypeSingleton && binding.Instance != nil {
			services[i] = binding.Instance
		} else {
			// For simplicity, return the resolver
			services[i] = binding.Resolver
		}
	}
	return services
}

// TaggedTyped retrieves all services with the given tag as specific type
func TaggedTyped[T any](c *Container, tag string) []T {
	tagged := Tagged(c, tag)
	var result []T

	for _, service := range tagged {
		if t, ok := service.(T); ok {
			result = append(result, t)
		}
	}
	return result
}

// ClearCache clears the resolution cache (useful for testing)
func (c *Container) ClearCache() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache = make(map[reflect.Type]any)
}
