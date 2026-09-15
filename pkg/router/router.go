// Purpose: Provides the Router type, route group support, and http.Handler implementation for method-and-path dispatch.

package router

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	httperr "github.com/example/lightweight-http/pkg/errors"
)

// Router matches incoming HTTP requests to registered handlers by method and path.
// Use New to create a Router. The zero value is not usable.
type Router struct {
	root  *trieNode
	mu    sync.RWMutex
	chain []func(http.Handler) http.Handler
}

// New returns a ready-to-use Router.
func New() *Router {
	return &Router{root: newTrieNode()}
}

// Use appends one or more middleware functions to the router's global chain.
// Middleware is applied to every request in registration order: the first Use call
// registers the outermost wrapper (ADR-009). Use is not safe to call concurrently
// with ServeHTTP; register all middleware before the server begins accepting requests.
func (ro *Router) Use(mws ...func(http.Handler) http.Handler) {
	ro.mu.Lock()
	defer ro.mu.Unlock()
	ro.chain = append(ro.chain, mws...)
}

// handle registers h for the given HTTP method and path pattern. It is the single
// entry point used by GET, POST, PUT, DELETE, PATCH, and the Group helpers.
//
// Patterns must begin with '/'. Path parameter segments start with ':', for example
// "/users/:id". Trailing slashes are significant: "/users" and "/users/" are distinct.
//
// handle returns an error when:
//   - the pattern is empty or does not begin with '/';
//   - the same method+pattern pair is registered more than once;
//   - a parameter name appears more than once in the same pattern;
//   - two patterns share a wildcard position but use different parameter names.
func (ro *Router) handle(method, pattern string, h http.HandlerFunc) error {
	if pattern == "" || pattern[0] != '/' {
		return fmt.Errorf("router: pattern %q must begin with '/'", pattern)
	}

	segments := splitPattern(pattern)
	paramsSeen := make(map[string]bool)

	ro.mu.Lock()
	defer ro.mu.Unlock()
	return ro.root.register(method, segments, paramsSeen, h)
}

// GET registers a handler for GET requests matching the given pattern.
func (ro *Router) GET(pattern string, h http.HandlerFunc) error {
	return ro.handle(http.MethodGet, pattern, h)
}

// POST registers a handler for POST requests matching the given pattern.
func (ro *Router) POST(pattern string, h http.HandlerFunc) error {
	return ro.handle(http.MethodPost, pattern, h)
}

// PUT registers a handler for PUT requests matching the given pattern.
func (ro *Router) PUT(pattern string, h http.HandlerFunc) error {
	return ro.handle(http.MethodPut, pattern, h)
}

// DELETE registers a handler for DELETE requests matching the given pattern.
func (ro *Router) DELETE(pattern string, h http.HandlerFunc) error {
	return ro.handle(http.MethodDelete, pattern, h)
}

// PATCH registers a handler for PATCH requests matching the given pattern.
func (ro *Router) PATCH(pattern string, h http.HandlerFunc) error {
	return ro.handle(http.MethodPatch, pattern, h)
}

// Group returns a Group that prefixes all registered patterns with prefix.
// prefix must begin with '/' and should not end with '/'.
// Groups can be nested; each nested call appends to the parent's prefix.
func (ro *Router) Group(prefix string) *Group {
	return &Group{prefix: prefix, router: ro}
}

// ServeHTTP dispatches the incoming request to the best-matching handler.
//
//   - If a handler is found for the path and method, it is invoked with path parameters
//     attached to the request context.
//   - If the path matches but no handler is registered for the method, ServeHTTP writes
//     a 405 response with an Allow header listing the accepted methods.
//   - If no path matches, ServeHTTP writes a 404 response.
func (ro *Router) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	segments := splitPath(path)

	ro.mu.RLock()
	result := ro.root.match(segments, nil, nil)
	chain := make([]func(http.Handler) http.Handler, len(ro.chain))
	copy(chain, ro.chain)
	ro.mu.RUnlock()

	if result == nil {
		httperr.Handle(w, r, httperr.NotFound("not found"), nil, false)
		return
	}

	h, ok := result.node.handlers[r.Method]
	if !ok {
		allowed := result.node.allowedMethods()
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		httperr.Handle(w, r, httperr.New(http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed"), nil, false)
		return
	}

	if len(result.captured) > 0 {
		params := make(map[string]string, len(result.captured))
		for i, name := range result.paramNames {
			params[name] = result.captured[i]
		}
		r = r.WithContext(withParams(r.Context(), params))
	}

	if len(chain) == 0 {
		h(w, r)
		return
	}
	var handler http.Handler = h
	for i := len(chain) - 1; i >= 0; i-- {
		handler = chain[i](handler)
	}
	handler.ServeHTTP(w, r)
}

// splitPattern splits a pattern string into its path segments, stripping the leading slash.
// "/" returns an empty slice. "/a/b" returns ["a", "b"].
func splitPattern(pattern string) []string {
	trimmed := strings.TrimPrefix(pattern, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

// splitPath splits a request URL path into its segments, stripping the leading slash.
// The result mirrors splitPattern so that route registration and dispatch use
// identical segment sequences.
func splitPath(path string) []string {
	trimmed := strings.TrimPrefix(path, "/")
	if trimmed == "" {
		return []string{}
	}
	return strings.Split(trimmed, "/")
}

// Group is a set of routes that share a common URL prefix and an optional middleware chain.
// Create one with Router.Group or Group.Group.
type Group struct {
	prefix string
	router *Router
	chain  []func(http.Handler) http.Handler
}

// Use appends middleware to this group's chain. Middleware registered here wraps only
// handlers registered through this group and its descendants (ADR-009).
// Middleware must be registered before route handlers; not safe to call after serving starts.
func (g *Group) Use(mws ...func(http.Handler) http.Handler) {
	g.chain = append(g.chain, mws...)
}

// handle registers a handler on the parent Router with the group's prefix prepended.
// The handler is wrapped with the group's middleware chain before registration so that
// group-level middleware executes after the router's global chain (ADR-009).
func (g *Group) handle(method, pattern string, h http.HandlerFunc) error {
	if len(g.chain) == 0 {
		return g.router.handle(method, g.prefix+pattern, h)
	}
	var wrapped http.Handler = h
	for i := len(g.chain) - 1; i >= 0; i-- {
		wrapped = g.chain[i](wrapped)
	}
	return g.router.handle(method, g.prefix+pattern, wrapped.ServeHTTP)
}

// GET registers a handler for GET requests under the group's prefix.
func (g *Group) GET(pattern string, h http.HandlerFunc) error {
	return g.handle(http.MethodGet, pattern, h)
}

// POST registers a handler for POST requests under the group's prefix.
func (g *Group) POST(pattern string, h http.HandlerFunc) error {
	return g.handle(http.MethodPost, pattern, h)
}

// PUT registers a handler for PUT requests under the group's prefix.
func (g *Group) PUT(pattern string, h http.HandlerFunc) error {
	return g.handle(http.MethodPut, pattern, h)
}

// DELETE registers a handler for DELETE requests under the group's prefix.
func (g *Group) DELETE(pattern string, h http.HandlerFunc) error {
	return g.handle(http.MethodDelete, pattern, h)
}

// PATCH registers a handler for PATCH requests under the group's prefix.
func (g *Group) PATCH(pattern string, h http.HandlerFunc) error {
	return g.handle(http.MethodPatch, pattern, h)
}

// Group returns a new Group whose prefix is this group's prefix concatenated with prefix.
// The new group inherits a copy of this group's middleware chain; additional middleware
// added to the child via Use does not affect the parent.
func (g *Group) Group(prefix string) *Group {
	inherited := make([]func(http.Handler) http.Handler, len(g.chain))
	copy(inherited, g.chain)
	return &Group{prefix: g.prefix + prefix, router: g.router, chain: inherited}
}
