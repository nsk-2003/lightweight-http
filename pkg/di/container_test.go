// Purpose: Tests for the DI container covering every behavioral requirement row in the spec.
package di_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/example/lightweight-http/pkg/di"
)

// ---- helpers ----------------------------------------------------------------

type disposableSvc struct {
	name      string
	onDispose func()
}

func (d *disposableSvc) Dispose() error {
	d.onDispose()
	return nil
}

type failingDisposable struct{}

func (f *failingDisposable) Dispose() error { return errors.New("dispose failed") }

func noopCtor(_ *di.Resolver) (any, error) { return struct{}{}, nil }

// ---- behavioral table tests -------------------------------------------------

func TestResolveUnregistered(t *testing.T) {
	c := di.New()
	_, err := c.Resolve("missing")
	if err == nil {
		t.Fatal("expected error for unregistered key")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error should name the key, got: %v", err)
	}
}

func TestSingletonSameInstance(t *testing.T) {
	c := di.New()
	type svc struct{}
	err := c.Register("svc", func(_ *di.Resolver) (any, error) { return &svc{}, nil }, di.Singleton)
	if err != nil {
		t.Fatal(err)
	}
	v1, err1 := c.Resolve("svc")
	v2, err2 := c.Resolve("svc")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if v1 != v2 {
		t.Error("singleton must return the same instance on repeated resolution")
	}
}

func TestScopedSameInstanceWithinScope(t *testing.T) {
	c := di.New()
	type svc struct{}
	err := c.Register("svc", func(_ *di.Resolver) (any, error) { return &svc{}, nil }, di.Scoped)
	if err != nil {
		t.Fatal(err)
	}
	scope := c.NewScope()
	v1, err1 := scope.Resolve("svc")
	v2, err2 := scope.Resolve("svc")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if v1 != v2 {
		t.Error("scoped service must return the same instance within one scope")
	}
}

func TestScopedDifferentInstancesAcrossScopes(t *testing.T) {
	c := di.New()
	type svc struct{ id int }
	var seq int
	err := c.Register("svc", func(_ *di.Resolver) (any, error) {
		seq++
		return &svc{id: seq}, nil
	}, di.Scoped)
	if err != nil {
		t.Fatal(err)
	}
	s1 := c.NewScope()
	s2 := c.NewScope()
	v1, _ := s1.Resolve("svc")
	v2, _ := s2.Resolve("svc")
	if v1 == v2 {
		t.Error("scoped service must return different instances across different scopes")
	}
	if v1.(*svc).id == v2.(*svc).id {
		t.Error("scoped service must have different IDs across different scopes")
	}
}

func TestScopedFromRootIsError(t *testing.T) {
	c := di.New()
	err := c.Register("svc", noopCtor, di.Scoped)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Resolve("svc")
	if err == nil {
		t.Fatal("expected error when resolving scoped service from root container")
	}
}

func TestCircularDependencyError(t *testing.T) {
	c := di.New()
	// A -> B -> C -> A
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(c.Register("A", func(r *di.Resolver) (any, error) { return r.Resolve("B") }, di.Singleton))
	must(c.Register("B", func(r *di.Resolver) (any, error) { return r.Resolve("C") }, di.Singleton))
	must(c.Register("C", func(r *di.Resolver) (any, error) { return r.Resolve("A") }, di.Singleton))

	_, err := c.Resolve("A")
	if err == nil {
		t.Fatal("expected cycle error")
	}
	msg := err.Error()
	for _, name := range []string{"A", "B", "C"} {
		if !strings.Contains(msg, name) {
			t.Errorf("cycle error should name %q, got: %v", name, msg)
		}
	}
	if !strings.Contains(msg, "->") {
		t.Errorf("cycle error should show path with ->, got: %v", msg)
	}
}

func TestConstructorErrorPropagated(t *testing.T) {
	c := di.New()
	sentinel := errors.New("build failed")
	err := c.Register("svc", func(_ *di.Resolver) (any, error) { return nil, sentinel }, di.Singleton)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Resolve("svc")
	if err == nil {
		t.Fatal("expected error from failing constructor")
	}
	if !strings.Contains(err.Error(), "svc") {
		t.Errorf("error should name the key, got: %v", err)
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error should wrap the original cause via errors.Is, got: %v", err)
	}
}

func TestConcurrentSingletonConstructedOnce(t *testing.T) {
	c := di.New()
	var mu sync.Mutex
	count := 0
	err := c.Register("svc", func(_ *di.Resolver) (any, error) {
		mu.Lock()
		count++
		mu.Unlock()
		return struct{}{}, nil
	}, di.Singleton)
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 100
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			if _, err := c.Resolve("svc"); err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()

	if count != 1 {
		t.Errorf("constructor should run exactly once, ran %d times", count)
	}
}

func TestDuplicateRegistrationError(t *testing.T) {
	c := di.New()
	if err := c.Register("svc", noopCtor, di.Singleton); err != nil {
		t.Fatal(err)
	}
	err := c.Register("svc", noopCtor, di.Singleton)
	if err == nil {
		t.Fatal("expected error on duplicate registration")
	}
}

func TestScopeDisposeReverseOrder(t *testing.T) {
	c := di.New()
	var order []string

	makeDisposable := func(name string) di.Constructor {
		return func(_ *di.Resolver) (any, error) {
			return &disposableSvc{name: name, onDispose: func() {
				order = append(order, name)
			}}, nil
		}
	}

	for _, name := range []string{"A", "B", "C"} {
		if err := c.Register(name, makeDisposable(name), di.Scoped); err != nil {
			t.Fatal(err)
		}
	}

	scope := c.NewScope()
	for _, name := range []string{"A", "B", "C"} {
		if _, err := scope.Resolve(name); err != nil {
			t.Fatal(err)
		}
	}

	if err := scope.Dispose(); err != nil {
		t.Fatalf("unexpected disposal error: %v", err)
	}

	want := []string{"C", "B", "A"}
	if fmt.Sprint(order) != fmt.Sprint(want) {
		t.Errorf("dispose order = %v, want %v", order, want)
	}
}

func TestDisposalFailureDoesNotPreventOthers(t *testing.T) {
	c := di.New()
	var aDisposed bool

	_ = c.Register("A", func(_ *di.Resolver) (any, error) {
		return &disposableSvc{name: "A", onDispose: func() { aDisposed = true }}, nil
	}, di.Scoped)
	_ = c.Register("B", func(_ *di.Resolver) (any, error) {
		return &failingDisposable{}, nil
	}, di.Scoped)

	scope := c.NewScope()
	if _, err := scope.Resolve("A"); err != nil {
		t.Fatal(err)
	}
	if _, err := scope.Resolve("B"); err != nil {
		t.Fatal(err)
	}

	err := scope.Dispose()
	if err == nil {
		t.Error("expected error from failing disposal")
	}
	if !aDisposed {
		t.Error("A should have been disposed despite B's disposal failure")
	}
}

// ---- context attachment -----------------------------------------------------

func TestContextScopeAttachAndRetrieve(t *testing.T) {
	c := di.New()
	scope := c.NewScope()
	ctx := di.WithScope(context.Background(), scope)

	retrieved, ok := di.ScopeFromContext(ctx)
	if !ok {
		t.Fatal("expected scope in context, got false")
	}
	if retrieved != scope {
		t.Error("retrieved scope must be the same pointer as the stored scope")
	}
}

func TestContextScopeAbsent(t *testing.T) {
	_, ok := di.ScopeFromContext(context.Background())
	if ok {
		t.Error("expected false when no scope is in context")
	}
}

// ---- interface key ----------------------------------------------------------

func TestInterfaceKeyRegistrationAndResolution(t *testing.T) {
	type Greeter interface{ Greet() string }
	type greetImpl struct{}

	key := di.InterfaceKey((*Greeter)(nil))
	if key == "" {
		t.Fatal("interface key must not be empty")
	}

	c := di.New()
	if err := c.Register(key, func(_ *di.Resolver) (any, error) {
		return &greetImpl{}, nil
	}, di.Singleton); err != nil {
		t.Fatal(err)
	}

	val, err := c.Resolve(key)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if val == nil {
		t.Error("expected non-nil service for interface key")
	}
}

// ---- dependency resolution through Resolver ---------------------------------

func TestResolverResolveDependency(t *testing.T) {
	c := di.New()
	type dep struct{ val int }
	type svc struct{ d *dep }

	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}

	must(c.Register("dep", func(_ *di.Resolver) (any, error) {
		return &dep{val: 42}, nil
	}, di.Singleton))

	must(c.Register("svc", func(r *di.Resolver) (any, error) {
		raw, err := r.Resolve("dep")
		if err != nil {
			return nil, err
		}
		return &svc{d: raw.(*dep)}, nil
	}, di.Singleton))

	raw, err := c.Resolve("svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := raw.(*svc)
	if got.d == nil || got.d.val != 42 {
		t.Errorf("expected dep.val=42, got %+v", got.d)
	}
}
