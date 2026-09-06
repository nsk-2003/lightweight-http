// Purpose: Context key types and accessor functions for path parameters, query
// values, and the matched route pattern; keeps keys unexported so no external
// package can forge stored values (ADR-005).
package router

import (
	"context"
	"net/http"
	"net/url"
)

// paramsKey is the unexported context key for path parameters (ADR-005).
type paramsKey struct{}

// Params returns the path parameters stored in ctx by the router.
// Returns nil when no parameters are present (e.g. routes with no :name segments).
func Params(ctx context.Context) map[string]string {
	m, _ := ctx.Value(paramsKey{}).(map[string]string)
	return m
}

// QueryValues returns the parsed query string from r.
// Repeated keys produce multiple values; all values are accessible via the
// returned url.Values map.
func QueryValues(r *http.Request) url.Values {
	return r.URL.Query()
}

// withParams attaches params to ctx and returns the new context.
func withParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, paramsKey{}, params)
}

// ─── Route pattern context helpers ───────────────────────────────────────────

// routePatternKey is the unexported context key for the matched route pattern.
type routePatternKey struct{}

// RoutePatternHolder is a mutable value that observability middleware installs
// in the request context before dispatching. The router writes the matched
// route pattern into it; middleware reads it after ServeHTTP returns.
type RoutePatternHolder struct {
	// Pattern is the matched route template, e.g. "/users/:id".
	Pattern string
}

// WithRoutePatternHolder installs a fresh RoutePatternHolder into ctx and
// returns the updated context along with a pointer to the holder. Retain the
// pointer; read Pattern after the downstream handler returns.
func WithRoutePatternHolder(ctx context.Context) (context.Context, *RoutePatternHolder) {
	h := &RoutePatternHolder{}
	return context.WithValue(ctx, routePatternKey{}, h), h
}

// RoutePatternHolderFromContext returns the RoutePatternHolder installed in
// ctx, or nil if none has been set.
func RoutePatternHolderFromContext(ctx context.Context) *RoutePatternHolder {
	h, _ := ctx.Value(routePatternKey{}).(*RoutePatternHolder)
	return h
}

// RoutePattern returns the matched route pattern stored in ctx by the router,
// or "" if no RoutePatternHolder was installed.
func RoutePattern(ctx context.Context) string {
	if h := RoutePatternHolderFromContext(ctx); h != nil {
		return h.Pattern
	}
	return ""
}

// setRoutePattern writes pattern into the holder in ctx if one is present.
// Called by the router after a successful route match.
func setRoutePattern(ctx context.Context, pattern string) {
	if h := RoutePatternHolderFromContext(ctx); h != nil {
		h.Pattern = pattern
	}
}
