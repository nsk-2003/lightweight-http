// Purpose: Lightweight DI container supporting singleton and scoped lifecycles,
// circular-dependency detection, disposal in reverse construction order, and
// request-context integration (ADR-010, ADR-005).
package di

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
)

// ─── Lifecycle ────────────────────────────────────────────────────────────────

// Lifecycle controls how long a resolved service instance lives.
type Lifecycle int

const (
	// Singleton creates at most one instance per Container; thread-safe.
	Singleton Lifecycle = iota
	// Scoped creates at most one instance per Scope. Resolving a Scoped service
	// from the root Container is an error.
	Scoped
)

// ─── Public interfaces ────────────────────────────────────────────────────────

// Resolver resolves services by string key. It is passed to Constructor
// functions so they can declare dependencies without holding a Container.
type Resolver interface {
	// Resolve returns the service for key, constructing it if needed.
	Resolve(key string) (any, error)
}

// Disposable is implemented by services that need cleanup when a Scope ends.
type Disposable interface {
	// Dispose performs cleanup. Errors are logged but do not stop other disposals.
	Dispose() error
}

// Constructor builds a service instance. r may be used to resolve dependencies.
type Constructor func(r Resolver) (any, error)

// ─── internal types ───────────────────────────────────────────────────────────

type registration struct {
	ctor      Constructor
	lifecycle Lifecycle
}

// singletonEntry holds the once-constructed value or error for a Singleton key.
// sync.Once guarantees construction happens at most once even under concurrent
// calls; if construction fails, the error is returned to every waiting caller.
type singletonEntry struct {
	once  sync.Once
	value any
	err   error
}

// ─── Container ────────────────────────────────────────────────────────────────

// Container is the root dependency injection container. Register services at
// application startup; resolve or create request Scopes at the composition root.
// Container is safe for concurrent use after all registrations are complete.
type Container struct {
	mu            sync.RWMutex
	registrations map[string]*registration
	singletons    map[string]*singletonEntry
}

// NewContainer creates a new, empty root Container.
func NewContainer() *Container {
	return &Container{
		registrations: make(map[string]*registration),
		singletons:    make(map[string]*singletonEntry),
	}
}

// Register registers key with the given constructor and lifecycle. Returns an
// error if key is already registered (no silent last-wins overwrite).
func (c *Container) Register(key string, ctor Constructor, lifecycle Lifecycle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.registrations[key]; exists {
		return fmt.Errorf("di: key %q already registered", key)
	}
	c.registrations[key] = &registration{ctor: ctor, lifecycle: lifecycle}
	if lifecycle == Singleton {
		c.singletons[key] = &singletonEntry{}
	}
	return nil
}

// Resolve resolves the service for key from the root container. Scoped services
// must be resolved via NewScope().Resolve(); calling Resolve for a Scoped key
// on the root returns an error.
func (c *Container) Resolve(key string) (any, error) {
	return c.resolve(nil, key, nil)
}

// NewScope creates a request-scoped child of this Container.
func (c *Container) NewScope() *Scope {
	return &Scope{parent: c}
}

// resolve is the shared resolution core used by both Container and Scope.
// scope is nil when resolving from the root. path tracks the chain of keys
// currently being resolved to detect circular dependencies.
func (c *Container) resolve(scope *Scope, key string, path []string) (any, error) {
	// Cycle detection: key already on the resolution stack?
	for _, k := range path {
		if k == key {
			parts := make([]string, len(path)+1)
			copy(parts, path)
			parts[len(path)] = key
			return nil, fmt.Errorf("di: circular dependency detected: %s", strings.Join(parts, " -> "))
		}
	}

	c.mu.RLock()
	reg, ok := c.registrations[key]
	var entry *singletonEntry
	if ok && reg.lifecycle == Singleton {
		entry = c.singletons[key]
	}
	c.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("di: no registration for key %q", key)
	}

	switch reg.lifecycle {
	case Singleton:
		// Build a fresh path slice so the closure captures its own copy.
		newPath := make([]string, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = key

		entry.once.Do(func() {
			r := &internalResolver{container: c, scope: scope, path: newPath}
			v, err := reg.ctor(r)
			if err != nil {
				entry.err = fmt.Errorf("di: constructor for %q failed: %w", key, err)
				return
			}
			entry.value = v
		})
		return entry.value, entry.err

	case Scoped:
		if scope == nil {
			return nil, fmt.Errorf("di: scoped service %q cannot be resolved from the root container; use NewScope().Resolve()", key)
		}
		return scope.resolveScoped(key, path, reg.ctor)
	}

	return nil, fmt.Errorf("di: unknown lifecycle for key %q", key)
}

// ─── internalResolver ─────────────────────────────────────────────────────────

// internalResolver implements Resolver and carries cycle-detection state.
type internalResolver struct {
	container *Container
	scope     *Scope
	path      []string
}

// Resolve implements Resolver by delegating to the container's resolve core.
func (r *internalResolver) Resolve(key string) (any, error) {
	return r.container.resolve(r.scope, key, r.path)
}

// ─── Scope ────────────────────────────────────────────────────────────────────

// scopedEntry is the per-key cache cell for a Scope, analogous to singletonEntry.
type scopedEntry struct {
	once  sync.Once
	value any
	err   error
}

type disposableEntry struct {
	key  string
	disp Disposable
}

// Scope is a request-scoped child container. Scoped services are constructed
// at most once per Scope and disposed when Dispose is called.
// Scope is safe for concurrent use.
type Scope struct {
	parent      *Container
	entries     sync.Map // map[string]*scopedEntry; key → *scopedEntry
	dispMu      sync.Mutex
	disposables []disposableEntry // appended in construction order
}

// Resolve resolves the service for key within this Scope. Both Singleton and
// Scoped services may be resolved; Singletons are served from the parent Container.
func (s *Scope) Resolve(key string) (any, error) {
	return s.parent.resolve(s, key, nil)
}

// resolveScoped handles Scoped construction within this scope.
func (s *Scope) resolveScoped(key string, path []string, ctor Constructor) (any, error) {
	actual, _ := s.entries.LoadOrStore(key, &scopedEntry{})
	entry := actual.(*scopedEntry)

	// Build a fresh path slice for this key before capturing in the closure.
	newPath := make([]string, len(path)+1)
	copy(newPath, path)
	newPath[len(path)] = key

	entry.once.Do(func() {
		r := &internalResolver{container: s.parent, scope: s, path: newPath}
		v, err := ctor(r)
		if err != nil {
			entry.err = fmt.Errorf("di: constructor for %q failed: %w", key, err)
			return
		}
		entry.value = v
		if d, ok := v.(Disposable); ok {
			s.dispMu.Lock()
			s.disposables = append(s.disposables, disposableEntry{key: key, disp: d})
			s.dispMu.Unlock()
		}
	})
	return entry.value, entry.err
}

// Dispose calls Dispose on every Disposable instance in reverse construction
// order. A Dispose error is logged but does not prevent remaining disposals.
func (s *Scope) Dispose() {
	s.dispMu.Lock()
	snapshot := make([]disposableEntry, len(s.disposables))
	copy(snapshot, s.disposables)
	s.dispMu.Unlock()

	for i := len(snapshot) - 1; i >= 0; i-- {
		if err := snapshot[i].disp.Dispose(); err != nil {
			slog.Default().Error("di: disposal failed",
				slog.String("key", snapshot[i].key),
				slog.Any("error", err),
			)
		}
	}
}

// ─── Context helpers ──────────────────────────────────────────────────────────

// scopeKey is the unexported context key for the active Scope (ADR-005).
type scopeKey struct{}

// ContextWithScope returns a new context carrying scope.
func ContextWithScope(ctx context.Context, scope *Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, scope)
}

// ScopeFromContext retrieves the Scope stored by ContextWithScope. Returns
// false if the context carries no Scope.
func ScopeFromContext(ctx context.Context) (*Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(*Scope)
	return s, ok
}
