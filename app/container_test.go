package app

import (
	"fmt"
	"testing"

	"github.com/shaurya/astra/contracts"
)

func TestContainerBind(t *testing.T) {
	c := NewContainer()
	c.Bind("test/service", func(container contracts.ContainerContract) (any, error) {
		return "hello", nil
	})

	val, err := c.Make("test/service")
	if err != nil {
		t.Fatalf("Make() returned error: %v", err)
	}
	if val != "hello" {
		t.Fatalf("expected 'hello', got '%v'", val)
	}
}

func TestContainerBindReturnsNewInstance(t *testing.T) {
	c := NewContainer()
	counter := 0
	c.Bind("test/counter", func(container contracts.ContainerContract) (any, error) {
		counter++
		return counter, nil
	})

	val1, _ := c.Make("test/counter")
	val2, _ := c.Make("test/counter")

	if val1.(int) == val2.(int) {
		t.Fatal("Bind should return new instances, got same value")
	}
}

func TestContainerSingleton(t *testing.T) {
	c := NewContainer()
	counter := 0
	c.Singleton("test/singleton", func(container contracts.ContainerContract) (any, error) {
		counter++
		return counter, nil
	})

	val1, _ := c.Make("test/singleton")
	val2, _ := c.Make("test/singleton")

	if val1.(int) != val2.(int) {
		t.Fatalf("Singleton should return same instance, got %v and %v", val1, val2)
	}
	if counter != 1 {
		t.Fatalf("Singleton factory should be called once, called %d times", counter)
	}
}

func TestContainerUse(t *testing.T) {
	c := NewContainer()
	c.Bind("test/use", func(container contracts.ContainerContract) (any, error) {
		return 42, nil
	})

	val := c.Use("test/use")
	if val.(int) != 42 {
		t.Fatalf("expected 42, got %v", val)
	}
}

func TestContainerUsePanicsOnMissing(t *testing.T) {
	c := NewContainer()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("Use() should panic on missing binding")
		}
	}()
	c.Use("nonexistent")
}

func TestContainerHasBinding(t *testing.T) {
	c := NewContainer()
	c.Bind("exists", func(container contracts.ContainerContract) (any, error) {
		return true, nil
	})

	if !c.HasBinding("exists") {
		t.Fatal("HasBinding should return true for registered binding")
	}
	if c.HasBinding("does-not-exist") {
		t.Fatal("HasBinding should return false for unregistered binding")
	}
}

func TestContainerAlias(t *testing.T) {
	c := NewContainer()
	c.Singleton("Astra/Core/Router", func(container contracts.ContainerContract) (any, error) {
		return "router-instance", nil
	})
	c.Alias("Route", "Astra/Core/Router")

	val := c.Use("Route")
	if val != "router-instance" {
		t.Fatalf("expected 'router-instance', got '%v'", val)
	}
}

func TestContainerFakeAndRestore(t *testing.T) {
	c := NewContainer()
	c.Singleton("service", func(container contracts.ContainerContract) (any, error) {
		return "real", nil
	})

	// Original
	val, _ := c.Make("service")
	if val != "real" {
		t.Fatalf("expected 'real', got '%v'", val)
	}

	// Fake it
	c.Fake("service", func(container contracts.ContainerContract) (any, error) {
		return "fake", nil
	})
	val, _ = c.Make("service")
	if val != "fake" {
		t.Fatalf("expected 'fake', got '%v'", val)
	}

	// Restore
	c.Restore("service")
	val, _ = c.Make("service")
	if val != "real" {
		t.Fatalf("expected 'real' after restore, got '%v'", val)
	}
}

func TestContainerMakeErrorOnMissing(t *testing.T) {
	c := NewContainer()
	_, err := c.Make("missing")
	if err == nil {
		t.Fatal("Make should return error for unregistered namespace")
	}
}

func TestContainerWithBindings(t *testing.T) {
	c := NewContainer()
	c.Bind("svc1", func(container contracts.ContainerContract) (any, error) {
		return "one", nil
	})
	c.Bind("svc2", func(container contracts.ContainerContract) (any, error) {
		return "two", nil
	})

	err := c.WithBindings([]string{"svc1", "svc2"}, func(bindings map[string]any) error {
		if bindings["svc1"] != "one" || bindings["svc2"] != "two" {
			t.Fatal("WithBindings returned wrong values")
		}
		return nil
	})
	if err != nil {
		t.Fatalf("WithBindings returned error: %v", err)
	}
}

type mockService interface {
	Do() string
}

type concreteService struct{}

func (s *concreteService) Do() string { return "done" }

func TestContainerCall(t *testing.T) {
	c := NewContainer()

	// Register a type for injection
	var serviceInterface *mockService
	c.RegisterType(serviceInterface, "service")
	c.Singleton("service", func(container contracts.ContainerContract) (any, error) {
		return &concreteService{}, nil
	})

	// 1. Test auto-injection of registered type
	results, err := c.Call(func(s mockService) string {
		return s.Do()
	})
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if results[0].(string) != "done" {
		t.Errorf("Expected 'done', got '%v'", results[0])
	}

	// 2. Test mixed auto-injection and explicit args
	results, err = c.Call(func(s mockService, name string, age int) string {
		return fmt.Sprintf("%s-%s-%d", s.Do(), name, age)
	}, "astra", 5)
	if err != nil {
		t.Fatalf("Call failed: %v", err)
	}
	if results[0].(string) != "done-astra-5" {
		t.Errorf("Expected 'done-astra-5', got '%v'", results[0])
	}
}

func TestContainerRegisterType(t *testing.T) {
	c := NewContainer()

	// Register by instance
	c.RegisterType((*mockService)(nil), "mock")

	// Inject and verify
	c.Singleton("mock", func(container contracts.ContainerContract) (any, error) {
		return &concreteService{}, nil
	})

	res, _ := c.Call(func(m mockService) string { return m.Do() })
	if res[0].(string) != "done" {
		t.Errorf("RegisterType failed")
	}
}

func TestContainerCallErrorOnUnresolvable(t *testing.T) {
	c := NewContainer()
	_, err := c.Call(func(s mockService) {})
	if err == nil {
		t.Fatal("Call should fail when dependency cannot be resolved")
	}
}
