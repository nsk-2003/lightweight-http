// Purpose: Recovery middleware that converts handler panics into 500 responses (ADR-007).
// It is the only site in the codebase that calls recover().

package middleware

import (
	"fmt"
	"log/slog"
	"net/http"
)

// responseTracker wraps an http.ResponseWriter and tracks whether the response
// header has been committed, so Recovery can detect a partial response.
type responseTracker struct {
	http.ResponseWriter
	wroteHeader bool
}

// WriteHeader records that headers were committed, then delegates to the underlying writer.
func (rt *responseTracker) WriteHeader(code int) {
	rt.wroteHeader = true
	rt.ResponseWriter.WriteHeader(code)
}

// Write records that the response started (headers are sent on first Write), then delegates.
func (rt *responseTracker) Write(b []byte) (int, error) {
	rt.wroteHeader = true
	return rt.ResponseWriter.Write(b)
}

// Recovery returns a Middleware that catches panics in the handler or downstream middleware
// and converts them into a 500 Internal Server Error response.
//
// Special cases:
//   - http.ErrAbortHandler is re-panicked so the server can clean up the connection (ADR-007).
//   - If the handler has already begun writing the response, Recovery cannot overwrite the
//     status code; it logs the incident and returns without writing a new body.
//
// If logger is nil, slog.Default() is used.
func Recovery(logger *slog.Logger) Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tracker := &responseTracker{ResponseWriter: w}
			defer func() {
				val := recover()
				if val == nil {
					return
				}
				// Re-panic on ErrAbortHandler so net/http can tear down the connection.
				if val == http.ErrAbortHandler {
					panic(val) //nolint:gocritic // intentional re-panic
				}
				logger.Error("handler panic recovered",
					"panic", fmt.Sprintf("%v", val),
					"method", r.Method,
					"path", r.URL.Path,
				)
				if tracker.wroteHeader {
					logger.Error("panic occurred after partial response; connection may be corrupted",
						"method", r.Method,
						"path", r.URL.Path,
					)
					return
				}
				http.Error(tracker, "Internal Server Error", http.StatusInternalServerError)
			}()
			next.ServeHTTP(tracker, r)
		})
	}
}
