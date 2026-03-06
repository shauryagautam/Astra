package core

import "fmt"

// Resolve retrieves a named service from the app and asserts it to type T.
// Returns an error if the service is missing or of the wrong type.
func Resolve[T any](app *App, name string) (T, error) {
	svc := app.Get(name)
	if svc == nil {
		var zero T
		return zero, fmt.Errorf("service '%s' not registered", name)
	}
	
	typed, ok := svc.(T)
	if !ok {
		var zero T
		return zero, fmt.Errorf("service '%s' is not of expected type %T", name, zero)
	}
	
	return typed, nil
}

// MustResolve retrieves a named service and asserts it to type T.
// Panics if the service is missing or of the wrong type.
func MustResolve[T any](app *App, name string) T {
	val, err := Resolve[T](app, name)
	if err != nil {
		panic(err)
	}
	return val
}
// Get is an alias for Resolve[T].
func Get[T any](app *App, name string) (T, error) {
	return Resolve[T](app, name)
}

// MustGet is an alias for MustResolve[T].
func MustGet[T any](app *App, name string) T {
	return MustResolve[T](app, name)
}
