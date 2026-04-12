package engine_test

import (
	"context"
	"testing"

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
