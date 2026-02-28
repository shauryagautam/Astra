package contracts

// EventContract represents an event.
type EventContract interface {
	// Name returns the event name.
	Name() string
}

// ListenerFunc is a function that processes an event.
type ListenerFunc func(data any) error

// EventDispatcherContract defines the event manager.
// Mirrors AdonisJS's Event provider.
type EventDispatcherContract interface {
	// On registers a listener for an event.
	On(event string, listener ListenerFunc)

	// Once registers a listener that runs only once.
	Once(event string, listener ListenerFunc)

	// Emit dispatches an event to all listeners.
	Emit(event string, data any) error

	// Trap registers a temporary listener for testing.
	Trap(event string, listener ListenerFunc)

	// RemoveListener removes a specific listener.
	RemoveListener(event string, listener ListenerFunc)

	// ClearListeners removes all listeners for an event.
	ClearListeners(event string)
}
