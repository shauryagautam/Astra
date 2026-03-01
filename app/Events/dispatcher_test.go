package events

import (
	"testing"

	"github.com/shaurya/astra/contracts"
)

func TestEventOnAndEmit(t *testing.T) {
	d := NewDispatcher()
	var emittedData any
	var callCount int

	listener := func(data any) error {
		emittedData = data
		callCount++
		return nil
	}

	d.On("test.event", listener)

	payload := map[string]string{"foo": "bar"}
	err := d.Emit("test.event", payload)
	if err != nil {
		t.Fatalf("Emit failed: %v", err)
	}

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
	if emittedData.(map[string]string)["foo"] != "bar" {
		t.Errorf("emitted data mismatch, got %v", emittedData)
	}
}

func TestEventOnce(t *testing.T) {
	d := NewDispatcher()
	var callCount int

	d.Once("once.event", func(data any) error {
		callCount++
		return nil
	})

	d.Emit("once.event", nil)
	d.Emit("once.event", nil)

	if callCount != 1 {
		t.Errorf("expected 1 call for Once event, got %d", callCount)
	}
}

func TestEventTrap(t *testing.T) {
	d := NewDispatcher()
	var trapCalled bool
	var originalCalled bool

	d.On("trap.event", func(data any) error {
		originalCalled = true
		return nil
	})

	d.Trap("trap.event", func(data any) error {
		trapCalled = true
		return nil
	})

	d.Emit("trap.event", nil)

	if !trapCalled {
		t.Error("trap was not called")
	}
	if originalCalled {
		t.Error("original listener should not be called when trap is active")
	}
}

func TestRemoveListener(t *testing.T) {
	d := NewDispatcher()
	var callCount int

	var listener contracts.ListenerFunc
	listener = func(data any) error {
		callCount++
		return nil
	}

	d.On("remove.event", listener)
	d.Emit("remove.event", nil)

	d.RemoveListener("remove.event", listener)
	d.Emit("remove.event", nil)

	if callCount != 1 {
		t.Errorf("expected 1 call, got %d", callCount)
	}
}

func TestClearListeners(t *testing.T) {
	d := NewDispatcher()
	var callCount int

	d.On("clear.event", func(data any) error {
		callCount++
		return nil
	})

	d.ClearListeners("clear.event")
	d.Emit("clear.event", nil)

	if callCount != 0 {
		t.Errorf("expected 0 calls after ClearListeners, got %d", callCount)
	}
}
