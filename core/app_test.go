package core

import (
	"context"
	"testing"
	"time"
)

func TestApp_Lifecycle(t *testing.T) {
	app, err := New()
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}

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

	ctx := context.Background()
	if err := app.Boot(ctx); err != nil {
		t.Fatalf("failed to boot app: %v", err)
	}

	if !started {
		t.Error("expected OnStart hook to have run")
	}

	if err := app.Shutdown(1 * time.Second); err != nil {
		t.Fatalf("failed to shutdown app: %v", err)
	}

	if !stopped {
		t.Error("expected OnStop hook to have run")
	}
}

func TestApp_Recover(t *testing.T) {
	app, _ := New()
	defer app.Recover()
	// This test just ensures Recover doesn't panic itself or fail
}
