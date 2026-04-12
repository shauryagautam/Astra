// Package events provides a lightweight, thread-safe, in-process event
// emitter for decoupling controllers from side-effects (emails, audit logs, etc.).
package event

import (
	"context"
	"log/slog"
	"reflect"
	"runtime/debug"
	"sync"
)

// Event is the interface that all events must implement.
type Event interface {
	Name() string
	Data() any
}

// Listener is the interface for event consumers.
type Listener interface {
	Handle(ctx context.Context, event Event) error
}

// ListenerFunc is a helper to use functions as listeners.
type ListenerFunc func(ctx context.Context, event Event) error

// Handle implements the Listener interface.
func (f ListenerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}

// BaseEvent is a simple implementation of Event.
type BaseEvent struct {
	EventName string
	EventData any
}

func (e BaseEvent) Name() string { return e.EventName }
func (e BaseEvent) Data() any    { return e.EventData }

// Emitter is a thread-safe in-process event bus.
type Emitter struct {
	mu        sync.RWMutex
	listeners map[string][]Listener
	pool      chan struct{} // worker pool for async emissions
}

// New creates a new Emitter with a default worker pool of 100.
func New() *Emitter {
	return &Emitter{
		listeners: make(map[string][]Listener),
		pool:      make(chan struct{}, 100),
	}
}

// DefaultEmitter is the package-level default emitter.
var DefaultEmitter = New()

// On registers a listener for an event name.
func (e *Emitter) On(eventName string, listener Listener) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.listeners[eventName] = append(e.listeners[eventName], listener)
}

// OnFunc is a helper to register a function as a listener.
func (e *Emitter) OnFunc(eventName string, fn func(ctx context.Context, event Event) error) {
	e.On(eventName, ListenerFunc(fn))
}

// Once registers a listener that fires only once.
func (e *Emitter) Once(eventName string, listener Listener) {
	var once sync.Once
	var wrapper ListenerFunc
	wrapper = func(ctx context.Context, event Event) error {
		once.Do(func() {
			e.Off(event.Name(), wrapper)
			_ = listener.Handle(ctx, event)
		})
		return nil
	}
	e.On(eventName, wrapper)
}

// Off removes exactly the specified handler.
func (e *Emitter) Off(eventName string, listener Listener) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ls, ok := e.listeners[eventName]
	if !ok {
		return
	}

	for i, l := range ls {
		// Pointer comparison for precision
		if reflect.ValueOf(l).Pointer() == reflect.ValueOf(listener).Pointer() {
			e.listeners[eventName] = append(ls[:i], ls[i+1:]...)
			break
		}
	}
}

// Emit fires all listeners for the given event synchronously.
func (e *Emitter) Emit(ctx context.Context, event Event) {
	ls := e.getListeners(event.Name())
	for _, l := range ls {
		e.safeHandle(ctx, l, event)
	}
}

// EmitAsync fires all listeners for the given event using a fixed worker pool.
func (e *Emitter) EmitAsync(ctx context.Context, event Event) {
	ls := e.getListeners(event.Name())
	for _, l := range ls {
		e.pool <- struct{}{} // acquire worker
		go func(li Listener) {
			defer func() { <-e.pool }() // release worker
			e.safeHandle(ctx, li, event)
		}(l)
	}
}

func (e *Emitter) getListeners(eventName string) []Listener {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Get specific listeners
	specific := e.listeners[eventName]

	// 2. Get wildcard listeners
	wildcards := e.listeners["*"]

	ls := make([]Listener, 0, len(specific)+len(wildcards))
	ls = append(ls, specific...)
	if eventName != "*" {
		ls = append(ls, wildcards...)
	}

	return ls
}

func (e *Emitter) safeHandle(ctx context.Context, l Listener, event Event) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("event: listener panicked",
				"event", event.Name(),
				"panic", r,
				"stack", string(debug.Stack()))
		}
	}()

	if err := l.Handle(ctx, event); err != nil {
		slog.Error("event: listener failed",
			"event", event.Name(),
			"error", err)
	}
}

// ─── Legacy Helpers ────────────────────────────────────────────────────────

// OnPayload registers a listener that receives the raw data.
func (e *Emitter) OnPayload(eventName string, fn func(data any)) {
	e.OnFunc(eventName, func(ctx context.Context, event Event) error {
		fn(event.Data())
		return nil
	})
}

// EmitPayload fires an event with a simple string name and any data.
func (e *Emitter) EmitPayload(ctx context.Context, eventName string, data any) {
	e.Emit(ctx, BaseEvent{EventName: eventName, EventData: data})
}
