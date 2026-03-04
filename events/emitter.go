// Package events provides a lightweight, thread-safe, in-process event
// emitter for decoupling controllers from side-effects (emails, audit logs, etc.).
package events

import (
	"sync"
)

// Listener is a function that handles an event payload.
type Listener func(payload any)

// Emitter is a thread-safe in-process event bus.
type Emitter struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
}

// New creates a new Emitter.
func New() *Emitter {
	return &Emitter{
		listeners: make(map[string][]Listener),
	}
}

// DefaultEmitter is the package-level default emitter.
var DefaultEmitter = New()

// On registers a listener for the given event name.
// Multiple listeners can be registered for the same event.
func (e *Emitter) On(event string, listener Listener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[event] = append(e.listeners[event], listener)
}

// Once registers a listener that fires only one time for the given event.
func (e *Emitter) Once(event string, listener Listener) {
	var once sync.Once
	var wrapper Listener
	wrapper = func(payload any) {
		once.Do(func() { listener(payload) })
	}
	e.On(event, wrapper)
}

// Off removes all listeners for the given event.
func (e *Emitter) Off(event string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	delete(e.listeners, event)
}

// Emit fires all listeners for the given event synchronously.
func (e *Emitter) Emit(event string, payload any) {
	e.mu.RLock()
	ls := make([]Listener, len(e.listeners[event]))
	copy(ls, e.listeners[event])
	e.mu.RUnlock()

	for _, l := range ls {
		l(payload)
	}
}

// EmitAsync fires all listeners for the given event in separate goroutines.
func (e *Emitter) EmitAsync(event string, payload any) {
	e.mu.RLock()
	ls := make([]Listener, len(e.listeners[event]))
	copy(ls, e.listeners[event])
	e.mu.RUnlock()

	for _, l := range ls {
		go l(payload)
	}
}

// ListenerCount returns the number of listeners registered for an event.
func (e *Emitter) ListenerCount(event string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.listeners[event])
}

// Events returns all registered event names.
func (e *Emitter) Events() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	names := make([]string, 0, len(e.listeners))
	for k := range e.listeners {
		names = append(names, k)
	}
	return names
}

// ─── Generic Helpers ─────────────────────────────────────────────────────────

// On registers a type-safe listener for the given event on the provided emitter.
func On[T any](emitter *Emitter, event string, handler func(payload T)) {
	emitter.On(event, func(raw any) {
		if typed, ok := raw.(T); ok {
			handler(typed)
		}
	})
}

// Emit fires a type-safe event on the provided emitter.
func Emit[T any](emitter *Emitter, event string, payload T) {
	emitter.Emit(event, payload)
}
