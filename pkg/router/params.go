// Purpose: Defines the unexported context key types and exported accessor functions for path parameters, query parameters, and the matched route pattern (ADR-005).

package router

import (
	"context"
	"net/http"
)

// paramsKey is an unexported context key type used to store path parameters (ADR-005).
type paramsKey struct{}

// withParams returns a new context carrying the given path parameter map.
func withParams(ctx context.Context, params map[string]string) context.Context {
	return context.WithValue(ctx, paramsKey{}, params)
}

// Params returns all path parameters extracted from the request context.
// Returns nil if no path parameters are present.
func Params(ctx context.Context) map[string]string {
	v, _ := ctx.Value(paramsKey{}).(map[string]string)
	return v
}

// PathParam returns the value of the named path parameter from the request.
// Returns an empty string if the parameter does not exist.
func PathParam(r *http.Request, name string) string {
	if m := Params(r.Context()); m != nil {
		return m[name]
	}
	return ""
}

// QueryParam returns the first value of the named query parameter.
// Returns an empty string if the parameter is absent.
func QueryParam(r *http.Request, name string) string {
	return r.URL.Query().Get(name)
}

// routePatternKey is an unexported context key type used to store the matched route pattern (ADR-005).
type routePatternKey struct{}

// withRoutePattern returns a new context carrying the matched route pattern string.
func withRoutePattern(ctx context.Context, pattern string) context.Context {
	return context.WithValue(ctx, routePatternKey{}, pattern)
}

// RoutePattern returns the matched route pattern (e.g. "/users/:id") from the request context.
// Returns an empty string if the request was not dispatched through the router or the pattern
// was not set.
func RoutePattern(r *http.Request) string {
	v, _ := r.Context().Value(routePatternKey{}).(string)
	return v
}
