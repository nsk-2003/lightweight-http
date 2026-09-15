// Purpose: Structured request logging middleware with a standard field set and sensitive-header redaction (ADR-011).

package observability

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	httperr "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/router"
)

// logErrKey is the unexported context key for an error stored by SetLogError.
type logErrKey struct{}

// logErrHolder is a pointer-sized mutable cell that allows inner handlers to communicate
// an error back to the logging middleware after ServeHTTP returns.
type logErrHolder struct {
	err error
}

// SetLogError stores err in ctx so the enclosing LoggingMiddleware includes it as the
// "error" field in the request log. The call is a no-op when ctx does not carry a holder
// (e.g. if the logging middleware is not in the chain).
func SetLogError(ctx context.Context, err error) {
	if h, ok := ctx.Value(logErrKey{}).(*logErrHolder); ok {
		h.err = err
	}
}

// LoggingMiddleware returns a Middleware that emits one structured log line per request
// after the inner handler completes.
//
// Every log line carries exactly these fields in order:
// timestamp, level, msg, request_id, method, route, path, status, duration_ms,
// bytes, remote_addr, error (omitted when nil).
//
// Sensitive headers (Authorization, Cookie, Set-Cookie, X-Api-Key, X-Auth-Token) are
// never included in any log field. If logger is nil, slog.Default() is used.
func LoggingMiddleware(logger *slog.Logger) middleware.Middleware {
	if logger == nil {
		logger = slog.Default()
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			holder := &logErrHolder{}
			r = r.WithContext(context.WithValue(r.Context(), logErrKey{}, holder))

			rec := newStatusRecorder(w)
			start := time.Now()
			next.ServeHTTP(rec, r)
			duration := time.Since(start)

			requestID := httperr.RequestIDFromContext(r.Context())
			route := router.RoutePattern(r)
			if route == "" {
				route = r.URL.Path
			}

			attrs := []slog.Attr{
				slog.String("request_id", requestID),
				slog.String("method", r.Method),
				slog.String("route", route),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Float64("duration_ms", float64(duration)/float64(time.Millisecond)),
				slog.Int("bytes", rec.bytes),
				slog.String("remote_addr", r.RemoteAddr),
			}
			if holder.err != nil {
				attrs = append(attrs, slog.Any("error", holder.err))
			}

			logger.LogAttrs(r.Context(), slog.LevelInfo, "request", attrs...)
		})
	}
}
