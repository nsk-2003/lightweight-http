// Purpose: Context key type and accessor functions for path parameters and query
// values; keeps the key unexported so no external package can forge a params value.
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
