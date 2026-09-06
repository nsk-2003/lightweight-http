// Purpose: Router and Group types — method registration, request dispatch,
// and route grouping. The Router implements http.Handler so it can be mounted
// on any net/http server (ADR-003).
package router

import (
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"

	httperrors "github.com/example/lightweight-http/pkg/errors"
)

// Router matches incoming requests to registered handlers. The zero value is
// not usable; use New to construct one.
type Router struct {
	mu   sync.RWMutex
	root *trieNode
}

// New returns a Router ready for route registration.
func New() *Router {
	return &Router{root: newNode()}
}

// GET registers h for GET requests matching pattern.
func (r *Router) GET(pattern string, h http.HandlerFunc) error {
	return r.register(http.MethodGet, pattern, h)
}

// POST registers h for POST requests matching pattern.
func (r *Router) POST(pattern string, h http.HandlerFunc) error {
	return r.register(http.MethodPost, pattern, h)
}

// PUT registers h for PUT requests matching pattern.
func (r *Router) PUT(pattern string, h http.HandlerFunc) error {
	return r.register(http.MethodPut, pattern, h)
}

// DELETE registers h for DELETE requests matching pattern.
func (r *Router) DELETE(pattern string, h http.HandlerFunc) error {
	return r.register(http.MethodDelete, pattern, h)
}

// PATCH registers h for PATCH requests matching pattern.
func (r *Router) PATCH(pattern string, h http.HandlerFunc) error {
	return r.register(http.MethodPatch, pattern, h)
}

// Group returns a Group whose routes all share the given path prefix. Groups
// can be nested: each call prepends its prefix to the parent's.
func (r *Router) Group(prefix string) *Group {
	return &Group{prefix: prefix, router: r}
}

// ServeHTTP implements http.Handler. It dispatches the request to the matching
// handler, returning 404 when no route matches the path and 405 (with an Allow
// header) when the path is known but not for the requested method.
func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	segs := splitPath(req.URL.Path)

	r.mu.RLock()
	result := r.root.match(segs, 0, nil)
	var handler http.HandlerFunc
	var allowed []string
	if result != nil {
		h, ok := result.node.handlers[req.Method]
		if ok {
			handler = h
		} else {
			allowed = sortedKeys(result.node.handlers)
		}
	} else {
		allowed = r.root.allowedMethods(segs, 0)
	}
	r.mu.RUnlock()

	switch {
	case handler != nil:
		ctx := req.Context()
		// Write the matched pattern into any RoutePatternHolder installed by
		// observability middleware so metrics and logging can key by pattern.
		setRoutePattern(ctx, result.pattern)
		if len(result.params) > 0 {
			ctx = withParams(ctx, result.params)
		}
		handler(w, req.WithContext(ctx))

	case len(allowed) > 0:
		// Path exists but method not registered — set Allow before writing body.
		w.Header().Set("Allow", strings.Join(allowed, ", "))
		httperrors.WriteError(w, req,
			httperrors.NewCoded(http.StatusMethodNotAllowed, "method_not_allowed", "Method Not Allowed"))

	default:
		httperrors.WriteError(w, req, httperrors.ErrNotFound)
	}
}

// register validates pattern, splits it into segments, and inserts the handler
// into the trie under a write lock.
func (r *Router) register(method, pattern string, h http.HandlerFunc) error {
	if pattern == "" || pattern[0] != '/' {
		return fmt.Errorf("router: pattern must begin with '/': %q", pattern)
	}
	segs := splitPath(pattern)
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.root.add(method, segs, h)
}

// splitPath breaks a URL path into its segments.
//
// Rules:
//   - The leading '/' is stripped.
//   - Root "/" returns nil (zero segments).
//   - A trailing slash produces an empty string as the final segment, making
//     "/users/" and "/users" distinct patterns.
func splitPath(p string) []string {
	if p == "/" || p == "" {
		return nil
	}
	if p[0] == '/' {
		p = p[1:]
	}
	return strings.Split(p, "/")
}

// sortedKeys returns the keys of handlers in sorted order, for deterministic
// Allow headers.
func sortedKeys(handlers map[string]http.HandlerFunc) []string {
	out := make([]string, 0, len(handlers))
	for k := range handlers {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ─── Group ────────────────────────────────────────────────────────────────────

// Group is a router sub-namespace that prepends a fixed path prefix to every
// pattern registered through it.
type Group struct {
	prefix string
	router *Router
}

// GET registers h for GET prefix+pattern requests.
func (g *Group) GET(pattern string, h http.HandlerFunc) error {
	return g.router.GET(g.prefix+pattern, h)
}

// POST registers h for POST prefix+pattern requests.
func (g *Group) POST(pattern string, h http.HandlerFunc) error {
	return g.router.POST(g.prefix+pattern, h)
}

// PUT registers h for PUT prefix+pattern requests.
func (g *Group) PUT(pattern string, h http.HandlerFunc) error {
	return g.router.PUT(g.prefix+pattern, h)
}

// DELETE registers h for DELETE prefix+pattern requests.
func (g *Group) DELETE(pattern string, h http.HandlerFunc) error {
	return g.router.DELETE(g.prefix+pattern, h)
}

// PATCH registers h for PATCH prefix+pattern requests.
func (g *Group) PATCH(pattern string, h http.HandlerFunc) error {
	return g.router.PATCH(g.prefix+pattern, h)
}

// Group returns a nested Group whose prefix is g.prefix+prefix.
func (g *Group) Group(prefix string) *Group {
	return &Group{prefix: g.prefix + prefix, router: g.router}
}
