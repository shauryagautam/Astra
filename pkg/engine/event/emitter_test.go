package event

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

type testEvent struct {
	BaseEvent
	Val string
}

func (e testEvent) Name() string { return "test.event" }
func (e testEvent) Data() any    { return e.Val }

type mockListener struct {
	mu     sync.Mutex
	called bool
	val    string
	err    error
}

func (l *mockListener) Handle(ctx context.Context, e Event) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.called = true
	l.val = e.Data().(string)
	return l.err
}

func TestEmitter_OnAndEmit(t *testing.T) {
	emitter := New()
	listener := &mockListener{}

	emitter.On("test.event", listener)

	emitter.Emit(context.Background(), testEvent{Val: "hello"})

	// Wait a bit for async execution
	time.Sleep(10 * time.Millisecond)

	assert.True(t, listener.called)
	assert.Equal(t, "hello", listener.val)
}

func TestEmitter_Wildcard(t *testing.T) {
	emitter := New()
	listener := &mockListener{}

	emitter.On("*", listener)

	emitter.Emit(context.Background(), testEvent{Val: "wildcard"})

	time.Sleep(10 * time.Millisecond)

	assert.True(t, listener.called)
	assert.Equal(t, "wildcard", listener.val)
}

func TestEmitter_PanicRecovery(t *testing.T) {
	emitter := New()

	// A listener that panics
	emitter.OnFunc("panic.event", func(ctx context.Context, e Event) error {
		panic("boom")
	})

	// This should not crash the program
	emitter.Emit(context.Background(), BaseEvent{EventName: "panic.event"})

	time.Sleep(10 * time.Millisecond)
	// If we reach here, we didn't crash
}

func TestEmitter_ListenerError(t *testing.T) {
	emitter := New()
	listener := &mockListener{err: errors.New("fail")}

	emitter.On("test.event", listener)

	emitter.Emit(context.Background(), testEvent{Val: "error"})

	time.Sleep(10 * time.Millisecond)
	assert.True(t, listener.called)
}
