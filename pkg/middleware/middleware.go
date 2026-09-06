// Purpose: Middleware type, Chain composer, and Recovery middleware — the
// composable handler-wrapping layer with registration-order execution,
// context propagation, and panic recovery (ADR-007, ADR-009).
package middleware

import (
	"log/slog"
	"net/http"
)

// Middleware wraps an http.Handler and returns a new http.Handler. Middleware
// may modify the request before calling next, observe or modify the response
// after, or short-circuit by not calling next at all.
type Middleware func(http.Handler) http.Handler

// Chain composes zero or more Middleware into a single Middleware function.
// The first element is the outermost wrapper: it receives the request first
// and the response last (ADR-009). Composing zero middleware returns a
// Middleware that returns the handler unchanged.
//
// Usage:
//
//	h := middleware.Chain(A, B, C)(handler)
//	// request:  A → B → C → handler
//	// response: handler → C → B → A
func Chain(mw ...Middleware) Middleware {
	return func(h http.Handler) http.Handler {
		for i := len(mw) - 1; i >= 0; i-- {
			h = mw[i](h)
		}
		return h
	}
}

// Recovery returns a Middleware that catches any panic in the downstream
// handler chain and converts it into a 500 Internal Server Error response.
//
// Special cases:
//   - http.ErrAbortHandler is re-panicked so the server's own connection-abort
//     handling is preserved (ADR-007).
//   - If the response header has already been committed when the panic occurs,
//     Recovery logs the incident and returns without writing a second header.
func Recovery(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{ResponseWriter: w}
			defer func() {
				v := recover()
				if v == nil {
					return
				}
				if v == http.ErrAbortHandler {
					panic(v) // preserve server abort handling
				}
				log.ErrorContext(r.Context(), "panic recovered",
					slog.Any("panic", v),
					slog.String("method", r.Method),
					slog.String("path", r.URL.Path),
				)
				if rw.wrote {
					// Header already committed; cannot write a second status.
					return
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(rw, r)
		})
	}
}

// responseWriter wraps http.ResponseWriter to track whether WriteHeader or
// Write has been called, allowing Recovery to detect a committed response.
type responseWriter struct {
	http.ResponseWriter
	wrote bool
}

// WriteHeader records that the header has been sent and delegates to the
// underlying ResponseWriter.
func (rw *responseWriter) WriteHeader(code int) {
	rw.wrote = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write records that a response body has been started (which implicitly
// commits the header) and delegates to the underlying ResponseWriter.
func (rw *responseWriter) Write(b []byte) (int, error) {
	rw.wrote = true
	return rw.ResponseWriter.Write(b)
}
