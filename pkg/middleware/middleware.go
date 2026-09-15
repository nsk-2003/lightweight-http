// Purpose: Defines the Middleware function type and the Chain composition abstraction
// that applies middleware in registration order (ADR-009).

package middleware

import "net/http"

// Middleware wraps an http.Handler to produce a new http.Handler.
// A Middleware may observe, modify, or short-circuit the request and response.
// It must derive new requests via r.WithContext(ctx) rather than mutating the
// shared request in place (ADR-005).
type Middleware func(http.Handler) http.Handler

// Chain holds an ordered, immutable sequence of Middleware.
// The zero value is an empty chain and is safe to use.
type Chain struct {
	mws []Middleware
}

// New returns a Chain that applies mws in registration order.
// The first element is the outermost wrapper and sees the request first (ADR-009).
func New(mws ...Middleware) Chain {
	copied := make([]Middleware, len(mws))
	copy(copied, mws)
	return Chain{mws: copied}
}

// Append returns a new Chain with the given middleware appended after the receiver's.
// The receiver is not modified.
func (c Chain) Append(mws ...Middleware) Chain {
	all := make([]Middleware, len(c.mws)+len(mws))
	copy(all, c.mws)
	copy(all[len(c.mws):], mws)
	return Chain{mws: all}
}

// Then wraps final with the chain's middleware and returns the resulting http.Handler.
// Execution order is mws[0] → mws[1] → … → final.
// If the chain is empty, final is returned unchanged.
func (c Chain) Then(final http.Handler) http.Handler {
	if len(c.mws) == 0 {
		return final
	}
	h := final
	for i := len(c.mws) - 1; i >= 0; i-- {
		h = c.mws[i](h)
	}
	return h
}
