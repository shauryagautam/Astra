package container

import (
	"testing"
)

type A struct{ B *B }
type B struct{ A *A }

func TestCircularDependency(t *testing.T) {
	c := New()

	// Register A depends on B
	c.Singleton(func(c *Container) (*A, error) {
		_, err := Resolve[*B](c)
		return &A{}, err
	})

	// Register B depends on A
	c.Singleton(func(c *Container) (*B, error) {
		_, err := Resolve[*A](c)
		return &B{}, err
	})

	_, err := Resolve[*A](c)
	if err == nil {
		t.Fatal("expected circular dependency error, got nil")
	}

	expected := "container: circular dependency detected for *container.A"
	if err.Error() != expected {
		t.Errorf("expected error %q, got %q", expected, err.Error())
	}
}

func TestSingleton(t *testing.T) {
	c := New()
	count := 0
	c.Singleton(func() *A {
		count++
		return &A{}
	})

	a1, _ := Resolve[*A](c)
	a2, _ := Resolve[*A](c)

	if a1 != a2 {
		t.Error("expected same instance for singleton")
	}
	if count != 1 {
		t.Errorf("expected resolver to be called once, got %d", count)
	}
}

func TestTransient(t *testing.T) {
	c := New()
	count := 0
	c.Transient(func() *A {
		count++
		return &A{}
	})

	a1, _ := Resolve[*A](c)
	a2, _ := Resolve[*A](c)

	if a1 == a2 {
		t.Error("expected different instances for transient")
	}
	if count != 2 {
		t.Errorf("expected resolver to be called twice, got %d", count)
	}
}

type Greeter interface {
	Greet() string
}

type EnglishGreeter struct{}

func (e *EnglishGreeter) Greet() string { return "Hello" }

func TestInterfaceBinding(t *testing.T) {
	c := New()
	c.Singleton(&EnglishGreeter{})

	g, err := Resolve[Greeter](c)
	if err != nil {
		t.Fatalf("failed to resolve interface: %v", err)
	}
	if g.Greet() != "Hello" {
		t.Errorf("expected 'Hello', got %q", g.Greet())
	}
}

func TestRecursiveResolution(t *testing.T) {
	c := New()

	c.Singleton(func() string {
		return "nested"
	})

	c.Singleton(func(s string) int {
		if s == "nested" {
			return 42
		}
		return 0
	})

	c.Singleton(func(i int) bool {
		return i == 42
	})

	res, err := Resolve[bool](c)
	if err != nil {
		t.Fatalf("failed to resolve recursively: %v", err)
	}
	if !res {
		t.Error("expected true from recursive resolution")
	}
}
