package events

import (
	"fmt"
	"reflect"
	"sync"

	"github.com/shaurya/astra/contracts"
)

// Dispatcher implements the EventDispatcherContract.
type Dispatcher struct {
	mu        sync.RWMutex
	listeners map[string][]contracts.ListenerFunc
	onces     map[string][]contracts.ListenerFunc
	traps     map[string]contracts.ListenerFunc
}

// NewDispatcher creates a new event dispatcher.
func NewDispatcher() *Dispatcher {
	return &Dispatcher{
		listeners: make(map[string][]contracts.ListenerFunc),
		onces:     make(map[string][]contracts.ListenerFunc),
		traps:     make(map[string]contracts.ListenerFunc),
	}
}

// On registers a listener for an event.
func (d *Dispatcher) On(event string, listener contracts.ListenerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.listeners[event] = append(d.listeners[event], listener)
}

// Once registers a listener that runs only once.
func (d *Dispatcher) Once(event string, listener contracts.ListenerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onces[event] = append(d.onces[event], listener)
}

// Trap registers a temporary listener for testing.
// If a trap is registered, all other listeners for that event are ignored.
func (d *Dispatcher) Trap(event string, listener contracts.ListenerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.traps[event] = listener
}

// Emit dispatches an event to all listeners.
// Returns an error if any synchronous listener fails.
func (d *Dispatcher) Emit(event string, data any) error {
	d.mu.RLock()
	// Check for traps first (testing)
	if trap, ok := d.traps[event]; ok {
		d.mu.RUnlock()
		return trap(data)
	}

	// Copy listeners to avoid holding lock during execution
	listeners := make([]contracts.ListenerFunc, 0)
	if l, ok := d.listeners[event]; ok {
		listeners = append(listeners, l...)
	}

	onces := make([]contracts.ListenerFunc, 0)
	if o, ok := d.onces[event]; ok {
		onces = append(onces, o...)
	}
	d.mu.RUnlock()

	// Handle Once listeners (clear them after fetching)
	if len(onces) > 0 {
		d.mu.Lock()
		delete(d.onces, event)
		d.mu.Unlock()
	}

	// Combine all listeners
	allListeners := append(listeners, onces...)

	for _, listener := range allListeners {
		// By default, we run listeners synchronously to ensure sequential execution
		// if users want async, they can wrap their logic in a goroutine within the listener.
		if err := listener(data); err != nil {
			return fmt.Errorf("event listener error for '%s': %w", event, err)
		}
	}

	return nil
}

// RemoveListener removes a specific listener.
func (d *Dispatcher) RemoveListener(event string, listener contracts.ListenerFunc) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Helper function to remove from a slice
	remove := func(slice []contracts.ListenerFunc, target contracts.ListenerFunc) []contracts.ListenerFunc {
		targetPtr := reflect.ValueOf(target).Pointer()
		for i, l := range slice {
			if reflect.ValueOf(l).Pointer() == targetPtr {
				return append(slice[:i], slice[i+1:]...)
			}
		}
		return slice
	}

	if l, ok := d.listeners[event]; ok {
		d.listeners[event] = remove(l, listener)
	}
	if o, ok := d.onces[event]; ok {
		d.onces[event] = remove(o, listener)
	}
}

// ClearListeners removes all listeners for an event.
func (d *Dispatcher) ClearListeners(event string) {
	d.mu.Lock()
	defer d.mu.Unlock()
	delete(d.listeners, event)
	delete(d.onces, event)
	delete(d.traps, event)
}

// Ensure Dispatcher implements EventDispatcherContract.
var _ contracts.EventDispatcherContract = (*Dispatcher)(nil)
