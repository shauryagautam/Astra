package engine_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/shauryagautam/Astra/pkg/engine"
	"github.com/shauryagautam/Astra/pkg/engine/config"
	"github.com/shauryagautam/Astra/pkg/test_util"
)

func TestApp_Lifecycle(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	app := ta.App

	started := false
	app.OnStart(func(ctx context.Context) error {
		started = true
		return nil
	})

	stopped := false
	app.OnStop(func(ctx context.Context) error {
		stopped = true
		return nil
	})

	if err := app.Boot(); err != nil {
		t.Fatalf("failed to boot app: %v", err)
	}

	if !started {
		t.Error("expected OnStart hook to have run")
	}

	if err := app.Shutdown(); err != nil {
		t.Fatalf("failed to shutdown app: %v", err)
	}

	if !stopped {
		t.Error("expected OnStop hook to have run")
	}
}

func TestApp_Recover(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	app := ta.App

	defer app.Recover()
	// This test just ensures Recover doesn't panic itself or fail
}

type testComponent struct {
	name  string
	order *[]string
	mu    *sync.Mutex
	stop  func(context.Context) error
}

func (c *testComponent) Start(context.Context) error {
	c.mu.Lock()
	*c.order = append(*c.order, "start:"+c.name)
	c.mu.Unlock()
	return nil
}

func (c *testComponent) Stop(ctx context.Context) error {
	c.mu.Lock()
	*c.order = append(*c.order, "stop:"+c.name)
	c.mu.Unlock()
	if c.stop != nil {
		return c.stop(ctx)
	}
	return nil
}

func TestApp_ComponentOrderAndJoinedShutdownErrors(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	var order []string
	var mu sync.Mutex
	firstErr := errors.New("first")
	secondErr := errors.New("second")

	if err := ta.App.RegisterComponent(&testComponent{name: "one", order: &order, mu: &mu, stop: func(context.Context) error {
		return firstErr
	}}); err != nil {
		t.Fatal(err)
	}
	if err := ta.App.RegisterComponent(&testComponent{name: "two", order: &order, mu: &mu, stop: func(context.Context) error {
		return secondErr
	}}); err != nil {
		t.Fatal(err)
	}
	if err := ta.App.Boot(); err != nil {
		t.Fatal(err)
	}
	if err := ta.App.Shutdown(); !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("expected joined shutdown errors, got %v", err)
	}

	want := []string{"start:one", "start:two", "stop:two", "stop:one"}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Fatalf("unexpected lifecycle order: got %v want %v", order, want)
	}
}

func TestApp_ConcurrentRegistrationAndHooks(t *testing.T) {
	ta := test_util.NewTestApp(t, nil)
	const count = 100
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := ta.App.RegisterComponent(&testComponent{order: new([]string), mu: &sync.Mutex{}}); err != nil {
				t.Errorf("register component: %v", err)
			}
			ta.App.OnStart(func(context.Context) error { return nil })
			ta.App.OnStop(func(context.Context) error { return nil })
		}()
	}
	wg.Wait()
	if err := ta.App.Boot(); err != nil {
		t.Fatal(err)
	}
	if err := ta.App.Shutdown(); err != nil {
		t.Fatal(err)
	}
}

func TestApp_ShutdownPropagatesParentCancellationUnderLoad(t *testing.T) {
	parent, cancelParent := context.WithCancel(context.Background())
	app := engine.NewWithContext(parent, &config.AstraConfig{}, &config.Config{}, nil)
	var workers sync.WaitGroup
	var stopCtx context.Context
	app.RegisterComponent(&testComponent{order: new([]string), mu: &sync.Mutex{}, stop: func(ctx context.Context) error {
		stopCtx = ctx
		workers.Wait()
		return nil
	}})
	if err := app.Boot(); err != nil {
		t.Fatal(err)
	}

	workers.Add(1)
	go func() {
		defer workers.Done()
		<-parent.Done()
	}()
	cancelParent()
	done := make(chan error, 1)
	go func() { done <- app.Shutdown() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("shutdown did not respect parent cancellation")
	}
	if stopCtx == nil {
		t.Fatal("component was not stopped")
	}
	if !errors.Is(stopCtx.Err(), context.Canceled) {
		t.Fatalf("expected canceled stop context, got %v", stopCtx.Err())
	}
}
