// Purpose: DI container with singleton/scoped lifecycles, circular-dependency detection, and
// request-scope context attachment (ADR-005, ADR-010).
package di

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
)

// Lifecycle controls how many times a service instance is constructed.
type Lifecycle int

const (
	// Singleton builds the service at most once per Container.
	Singleton Lifecycle = iota
	// Scoped builds the service once per Scope and disposes it when the scope ends.
	Scoped
)

// Disposable may be implemented by services that require cleanup when a Scope ends.
type Disposable interface {
	Dispose() error
}

// Constructor is a factory called by the container to build a service.
// r allows the constructor to resolve other registered services.
type Constructor func(r *Resolver) (any, error)

// registration is one service entry in the container.
type registration struct {
	ctor      Constructor
	lifecycle Lifecycle
}

// singletonEntry caches a singleton constructor's result.
// once ensures the constructor runs exactly once, even under concurrent resolution.
type singletonEntry struct {
	once sync.Once
	val  any
	err  error
}

// Container holds all service registrations and singleton instances.
// Constructed once at the composition root (cmd/, examples/); never imported by
// pkg/router, pkg/middleware, or pkg/errors (ARCHITECTURE layering rule).
type Container struct {
	mu            sync.RWMutex
	registrations map[string]registration
	singletons    map[string]*singletonEntry
}

// New returns an empty Container ready for service registrations.
func New() *Container {
	return &Container{
		registrations: make(map[string]registration),
		singletons:    make(map[string]*singletonEntry),
	}
}

// InterfaceKey returns the string key for an interface type using minimal reflection (ADR-010).
// Pass a nil pointer to the interface type: di.InterfaceKey((*MyInterface)(nil)).
func InterfaceKey(iface any) string {
	t := reflect.TypeOf(iface)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.String()
}

// Register adds a service to the container under key with the given lifecycle.
// Registering the same key twice is an error.
func (c *Container) Register(key string, ctor Constructor, lifecycle Lifecycle) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.registrations[key]; exists {
		return fmt.Errorf("di: key %q is already registered", key)
	}
	c.registrations[key] = registration{ctor: ctor, lifecycle: lifecycle}
	if lifecycle == Singleton {
		c.singletons[key] = &singletonEntry{}
	}
	return nil
}

// Resolve returns the singleton service registered under key.
// Resolving a Scoped service from the root container is an error.
func (c *Container) Resolve(key string) (any, error) {
	return c.resolve(key, nil, nil)
}

// NewScope creates a child Scope for resolving scoped services within a request.
func (c *Container) NewScope() *Scope {
	return &Scope{
		parent:    c,
		instances: make(map[string]any),
	}
}

// resolve is the shared recursive resolver used by both Container and Scope.
// scope is nil when resolving from the root (singleton-only context).
// path is the chain of keys being resolved; it detects cycles before they deadlock.
func (c *Container) resolve(key string, scope *Scope, path []string) (any, error) {
	for _, p := range path {
		if p == key {
			return nil, fmt.Errorf("di: circular dependency detected: %s",
				strings.Join(append(path, key), " -> "))
		}
	}

	c.mu.RLock()
	reg, ok := c.registrations[key]
	c.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("di: no service registered for key %q", key)
	}

	switch reg.lifecycle {
	case Singleton:
		return c.resolveSingleton(key, reg, scope, path)
	case Scoped:
		if scope == nil {
			return nil, fmt.Errorf("di: scoped service %q cannot be resolved from the root container", key)
		}
		return scope.resolveScoped(key, reg, path)
	default:
		return nil, fmt.Errorf("di: unknown lifecycle %d for key %q", reg.lifecycle, key)
	}
}

// resolveSingleton ensures the constructor runs exactly once per container.
// A failed constructor stores its error so every subsequent caller receives it,
// and never caches a partial instance.
func (c *Container) resolveSingleton(key string, reg registration, scope *Scope, path []string) (any, error) {
	c.mu.RLock()
	entry := c.singletons[key]
	c.mu.RUnlock()

	entry.once.Do(func() {
		newPath := make([]string, len(path)+1)
		copy(newPath, path)
		newPath[len(path)] = key
		r := &Resolver{container: c, scope: scope, path: newPath}
		entry.val, entry.err = reg.ctor(r)
		if entry.err != nil {
			entry.err = fmt.Errorf("di: constructing %q: %w", key, entry.err)
			entry.val = nil
		}
	})
	return entry.val, entry.err
}

// scopeKey is the unexported context key for the per-request Scope (ADR-005).
type scopeKey struct{}

// WithScope returns a new context carrying s as the request's DI scope.
func WithScope(ctx context.Context, s *Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// ScopeFromContext returns the Scope stored by WithScope, and whether one was present.
func ScopeFromContext(ctx context.Context) (*Scope, bool) {
	s, ok := ctx.Value(scopeKey{}).(*Scope)
	return s, ok
}

// Scope is a request-scoped child container.
// Scoped services are created once per Scope and disposed in reverse construction
// order when Dispose is called.
type Scope struct {
	parent      *Container
	mu          sync.Mutex
	instances   map[string]any
	disposables []Disposable
}

// Resolve returns the service registered under key within this scope.
// Singleton services are resolved through the parent container.
func (s *Scope) Resolve(key string) (any, error) {
	return s.parent.resolve(key, s, nil)
}

// resolveScoped returns the existing scoped instance or constructs a new one.
func (s *Scope) resolveScoped(key string, reg registration, path []string) (any, error) {
	s.mu.Lock()
	if val, ok := s.instances[key]; ok {
		s.mu.Unlock()
		return val, nil
	}
	s.mu.Unlock()

	newPath := make([]string, len(path)+1)
	copy(newPath, path)
	newPath[len(path)] = key
	r := &Resolver{container: s.parent, scope: s, path: newPath}
	val, err := reg.ctor(r)
	if err != nil {
		return nil, fmt.Errorf("di: constructing %q: %w", key, err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	// Double-check: another goroutine may have constructed the same service concurrently.
	if existing, ok := s.instances[key]; ok {
		if d, ok := val.(Disposable); ok {
			_ = d.Dispose()
		}
		return existing, nil
	}
	s.instances[key] = val
	if d, ok := val.(Disposable); ok {
		s.disposables = append(s.disposables, d)
	}
	return val, nil
}

// Dispose runs Dispose on all Disposable services in reverse construction order.
// A disposal failure is collected but does not prevent the remaining disposals.
func (s *Scope) Dispose() error {
	s.mu.Lock()
	disposables := s.disposables
	s.disposables = nil
	s.mu.Unlock()

	var errs []error
	for i := len(disposables) - 1; i >= 0; i-- {
		if err := disposables[i].Dispose(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// Resolver is passed to Constructor functions so they can resolve their own dependencies.
type Resolver struct {
	container *Container
	scope     *Scope
	path      []string
}

// Resolve returns the service registered under key.
func (r *Resolver) Resolve(key string) (any, error) {
	return r.container.resolve(key, r.scope, r.path)
}

// ResolveType resolves a service by its interface reflect.Type.
// t must be an interface type obtained via reflect.TypeOf((*MyInterface)(nil)).Elem().
func (r *Resolver) ResolveType(t reflect.Type) (any, error) {
	return r.container.resolve(t.String(), r.scope, r.path)
}
