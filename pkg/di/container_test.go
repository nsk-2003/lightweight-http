// Purpose: Black-box tests for the di package covering all Phase 6 behavioral
// requirements: registration, singleton/scoped lifecycles, cycle detection,
// constructor error propagation, disposal ordering, and context integration.
package di_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/example/lightweight-http/pkg/di"
)

// ─── Registration ──────────────────────────────────────────────────────────────

func TestRegister_Duplicate_ReturnsError(t *testing.T) {
	c := di.NewContainer()
	ctor := func(r di.Resolver) (any, error) { return "v", nil }
	if err := c.Register("svc", ctor, di.Singleton); err != nil {
		t.Fatalf("first Register: %v", err)
	}
	if err := c.Register("svc", ctor, di.Singleton); err == nil {
		t.Fatal("second Register with same key: expected error, got nil")
	}
}

// ─── Unregistered key ─────────────────────────────────────────────────────────

func TestResolve_Unregistered_ReturnsDescriptiveError(t *testing.T) {
	c := di.NewContainer()
	_, err := c.Resolve("missing")
	if err == nil {
		t.Fatal("expected error for unregistered key, got nil")
	}
	if !strings.Contains(err.Error(), "missing") {
		t.Errorf("error %q does not name the key", err.Error())
	}
}

// ─── Singleton ────────────────────────────────────────────────────────────────

func TestSingleton_ResolvedTwice_SameInstance(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return new(int), nil // new(int) produces a unique pointer on each call if not cached
	}, di.Singleton)

	v1, err1 := c.Resolve("svc")
	v2, err2 := c.Resolve("svc")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if v1 != v2 {
		t.Error("singleton: want same instance on both resolutions, got different")
	}
}

func TestSingleton_ConcurrentResolution_ConstructorRunsOnce(t *testing.T) {
	c := di.NewContainer()
	var count int64
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		atomic.AddInt64(&count, 1)
		return "shared", nil
	}, di.Singleton)

	const goroutines = 100
	var wg sync.WaitGroup
	results := make([]any, goroutines)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			v, err := c.Resolve("svc")
			if err != nil {
				t.Errorf("goroutine %d: unexpected error: %v", i, err)
				return
			}
			results[i] = v
		}(i)
	}
	wg.Wait()

	if n := atomic.LoadInt64(&count); n != 1 {
		t.Errorf("constructor called %d times, want exactly 1", n)
	}
	for i := range results {
		if results[i] != results[0] {
			t.Errorf("goroutine %d: got different instance", i)
		}
	}
}

// ─── Scoped ───────────────────────────────────────────────────────────────────

func TestScoped_SameScope_SameInstance(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return &struct{}{}, nil
	}, di.Scoped)

	scope := c.NewScope()
	v1, err1 := scope.Resolve("svc")
	v2, err2 := scope.Resolve("svc")
	if err1 != nil || err2 != nil {
		t.Fatalf("unexpected errors: %v, %v", err1, err2)
	}
	if v1 != v2 {
		t.Error("scoped: want same instance within one scope, got different")
	}
}

func TestScoped_TwoScopes_DifferentInstances(t *testing.T) {
	c := di.NewContainer()
	// Use *int so each constructor call produces a distinct pointer;
	// &struct{}{} is unsafe here because Go may return the same address
	// for all zero-size allocations.
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return new(int), nil
	}, di.Scoped)

	v1, _ := c.NewScope().Resolve("svc")
	v2, _ := c.NewScope().Resolve("svc")
	if v1 == v2 {
		t.Error("scoped: want different instances across two scopes, got same")
	}
}

func TestScoped_FromRoot_ReturnsError(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return "val", nil
	}, di.Scoped)

	_, err := c.Resolve("svc")
	if err == nil {
		t.Fatal("expected error resolving scoped service from root, got nil")
	}
	if !strings.Contains(err.Error(), "svc") {
		t.Errorf("error %q should name the key %q", err.Error(), "svc")
	}
}

// ─── Circular dependency ──────────────────────────────────────────────────────

func TestCircularDependency_TwoNode_NamesFullCycle(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("A", func(r di.Resolver) (any, error) {
		return r.Resolve("B")
	}, di.Singleton)
	_ = c.Register("B", func(r di.Resolver) (any, error) {
		return r.Resolve("A")
	}, di.Singleton)

	_, err := c.Resolve("A")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, "A") || !strings.Contains(msg, "B") {
		t.Errorf("cycle error %q should name A and B", msg)
	}
	if !strings.Contains(msg, "->") {
		t.Errorf("cycle error %q should use '->' notation", msg)
	}
}

func TestCircularDependency_ThreeNode_NamesFullPath(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("A", func(r di.Resolver) (any, error) {
		return r.Resolve("B")
	}, di.Singleton)
	_ = c.Register("B", func(r di.Resolver) (any, error) {
		return r.Resolve("C")
	}, di.Singleton)
	_ = c.Register("C", func(r di.Resolver) (any, error) {
		return r.Resolve("A")
	}, di.Singleton)

	_, err := c.Resolve("A")
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
	msg := err.Error()
	for _, name := range []string{"A", "B", "C"} {
		if !strings.Contains(msg, name) {
			t.Errorf("cycle error %q should name %q", msg, name)
		}
	}
	// Verify arrow notation is present
	if !strings.Contains(msg, "->") {
		t.Errorf("cycle error %q should use '->' notation", msg)
	}
}

// ─── Constructor error propagation ───────────────────────────────────────────

func TestConstructor_Singleton_ErrorPropagatedWithKey(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return nil, fmt.Errorf("build failed")
	}, di.Singleton)

	_, err := c.Resolve("svc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "svc") {
		t.Errorf("error %q should name the key %q", err.Error(), "svc")
	}
}

func TestConstructor_Scoped_ErrorPropagatedWithKey(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		return nil, fmt.Errorf("build failed")
	}, di.Scoped)

	_, err := c.NewScope().Resolve("svc")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "svc") {
		t.Errorf("error %q should name the key %q", err.Error(), "svc")
	}
}

// ─── Disposal ─────────────────────────────────────────────────────────────────

type trackedDisposable struct {
	name  string
	order *[]string
	err   error
}

func (d *trackedDisposable) Dispose() error {
	*d.order = append(*d.order, d.name)
	return d.err
}

func TestScope_Dispose_ReverseConstructionOrder(t *testing.T) {
	c := di.NewContainer()
	var order []string

	for _, name := range []string{"first", "second", "third"} {
		name := name // capture
		_ = c.Register(name, func(r di.Resolver) (any, error) {
			return &trackedDisposable{name: name, order: &order}, nil
		}, di.Scoped)
	}

	scope := c.NewScope()
	for _, name := range []string{"first", "second", "third"} {
		if _, err := scope.Resolve(name); err != nil {
			t.Fatalf("Resolve(%q): %v", name, err)
		}
	}
	scope.Dispose()

	want := []string{"third", "second", "first"}
	if len(order) != len(want) {
		t.Fatalf("disposal count: got %d, want %d", len(order), len(want))
	}
	for i, got := range order {
		if got != want[i] {
			t.Errorf("disposal order[%d]: got %q, want %q", i, got, want[i])
		}
	}
}

func TestScope_Dispose_FailureDoesNotPreventRemainingDisposals(t *testing.T) {
	c := di.NewContainer()
	var order []string

	_ = c.Register("a", func(r di.Resolver) (any, error) {
		return &trackedDisposable{name: "a", order: &order, err: fmt.Errorf("dispose failed")}, nil
	}, di.Scoped)
	_ = c.Register("b", func(r di.Resolver) (any, error) {
		return &trackedDisposable{name: "b", order: &order}, nil
	}, di.Scoped)

	scope := c.NewScope()
	scope.Resolve("a")
	scope.Resolve("b")
	scope.Dispose() // b disposes first (reverse), then a — a returns error but b already ran

	if len(order) != 2 {
		t.Errorf("expected 2 disposals despite error, got %d: %v", len(order), order)
	}
}

// ─── Context integration ──────────────────────────────────────────────────────

func TestContext_ScopeRoundTrip(t *testing.T) {
	c := di.NewContainer()
	scope := c.NewScope()
	ctx := di.ContextWithScope(context.Background(), scope)

	got, ok := di.ScopeFromContext(ctx)
	if !ok {
		t.Fatal("ScopeFromContext: expected true, got false")
	}
	if got != scope {
		t.Error("ScopeFromContext: returned different scope than stored")
	}
}

func TestContext_NoScope_ReturnsFalse(t *testing.T) {
	_, ok := di.ScopeFromContext(context.Background())
	if ok {
		t.Error("ScopeFromContext on empty context: expected false, got true")
	}
}

// ─── Cross-lifecycle resolution ───────────────────────────────────────────────

func TestScope_CanResolve_Singleton_FromScope(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("singleton", func(r di.Resolver) (any, error) {
		return "shared-value", nil
	}, di.Singleton)

	scope := c.NewScope()
	v, err := scope.Resolve("singleton")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "shared-value" {
		t.Errorf("got %v, want %q", v, "shared-value")
	}
}

func TestScoped_DependsOnSingleton(t *testing.T) {
	c := di.NewContainer()
	_ = c.Register("dep", func(r di.Resolver) (any, error) {
		return "dep-val", nil
	}, di.Singleton)
	_ = c.Register("svc", func(r di.Resolver) (any, error) {
		dep, err := r.Resolve("dep")
		if err != nil {
			return nil, err
		}
		return "svc:" + dep.(string), nil
	}, di.Scoped)

	scope := c.NewScope()
	v, err := scope.Resolve("svc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "svc:dep-val" {
		t.Errorf("got %v, want %q", v, "svc:dep-val")
	}
}
