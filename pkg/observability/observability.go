// Purpose: Request ID tracing, in-process request metrics, and structured
// request logging — each exposed as a composable middleware function compatible
// with the Phase 3 chain (ADR-011, ADR-012). Observability failure never fails
// a request.
package observability

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	httperrors "github.com/example/lightweight-http/pkg/errors"
	routerpkg "github.com/example/lightweight-http/pkg/router"
)

// ─── Request ID ───────────────────────────────────────────────────────────────

const (
	requestIDHeader = "X-Request-ID"
	maxRequestIDLen = 128
)

// RequestID returns a middleware that adopts a well-formed X-Request-ID from
// the incoming request or generates a cryptographically random one. The ID is
// stored in the request context via pkg/errors.WithRequestID and echoed on the
// X-Request-ID response header. An incoming ID is replaced if it exceeds
// maxRequestIDLen characters or contains any non-visible-ASCII character
// (outside 0x21–0x7E); values are never echoed unsanitized (ADR-011).
func RequestID() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestIDHeader)
			if !isValidRequestID(id) {
				id = generateRequestID()
			}
			ctx := httperrors.WithRequestID(r.Context(), id)
			w.Header().Set(requestIDHeader, id)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// isValidRequestID reports whether id may be adopted without sanitization:
// non-empty, at most maxRequestIDLen bytes, and every character is a visible
// ASCII non-space character (0x21 '!' through 0x7E '~').
func isValidRequestID(id string) bool {
	if len(id) == 0 || len(id) > maxRequestIDLen {
		return false
	}
	for _, c := range id {
		if c < 0x21 || c > 0x7E {
			return false
		}
	}
	return true
}

// generateRequestID returns a fresh 32-character lowercase hex string derived
// from 16 bytes of cryptographic randomness. Falls back to a timestamp-based
// string if the OS entropy source is unavailable.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("fb-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// ─── Logger ───────────────────────────────────────────────────────────────────

// Logger returns a middleware that emits one structured log line per request
// after the downstream chain returns. The log carries the standard Phase 7
// field set: request_id, method, route, path, status, duration_ms, bytes,
// remote_addr, and error (omitted when nil).
//
// Logger also installs a RoutePatternHolder in the request context so that
// the router can write the matched pattern; both Logger and any Metrics
// middleware below it read the pattern after dispatch (ADR-011).
//
// Secrets (Authorization, Cookie, and similar headers) are never logged.
func Logger(log *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rw := routerpkg.WrapResponseWriter(w, log)

			// Install route pattern holder so the router can populate it.
			ctx, holder := routerpkg.WithRoutePatternHolder(r.Context())
			r = r.WithContext(ctx)

			next.ServeHTTP(rw, r)

			elapsed := time.Since(start)
			log.LogAttrs(r.Context(), slog.LevelInfo, "request",
				slog.String("request_id", httperrors.RequestIDFromContext(r.Context())),
				slog.String("method", r.Method),
				slog.String("route", holder.Pattern),
				slog.String("path", r.URL.Path),
				slog.Int("status", rw.Status()),
				slog.Int64("duration_ms", elapsed.Milliseconds()),
				slog.Int64("bytes", rw.BytesWritten()),
				slog.String("remote_addr", r.RemoteAddr),
			)
		})
	}
}

// ─── Metrics ──────────────────────────────────────────────────────────────────

// MetricKey identifies a metric time series by HTTP method and route pattern.
// Pattern is the template registered with the router (e.g. "/users/:id"),
// never the concrete path (ADR-012).
type MetricKey struct {
	Method  string
	Pattern string
}

// RouteMetrics holds the aggregate counters for one MetricKey.
type RouteMetrics struct {
	// Requests is the total number of completed requests.
	Requests int64
	// StatusCodes maps each HTTP status code to its request count.
	StatusCodes map[int]int64
	// TotalMs is the cumulative request latency in milliseconds.
	TotalMs int64
}

// Snapshot is a point-in-time copy of all recorded metrics.
type Snapshot map[MetricKey]RouteMetrics

// Recorder collects in-process request metrics. Its zero value is ready to
// use. All methods are safe for concurrent use.
type Recorder struct {
	mu   sync.Mutex
	data map[MetricKey]*routeEntry
}

type routeEntry struct {
	requests    int64
	statusCodes map[int]int64
	totalMs     int64
}

// Middleware returns a middleware that records request count, latency, and
// status-code distribution keyed by (HTTP method, matched route pattern).
//
// Middleware ordering: place Metrics inside Logger (Logger → Metrics → …) so
// that Logger's RoutePatternHolder is already installed when Metrics runs. If
// used without Logger, Metrics installs its own holder so the router can still
// populate the pattern.
func (rec *Recorder) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Ensure a route pattern holder is in context (Logger may have set one).
			if routerpkg.RoutePatternHolderFromContext(r.Context()) == nil {
				ctx, _ := routerpkg.WithRoutePatternHolder(r.Context())
				r = r.WithContext(ctx)
			}

			start := time.Now()
			rw := &metricsRW{ResponseWriter: w}

			next.ServeHTTP(rw, r)

			elapsed := time.Since(start)
			key := MetricKey{
				Method:  r.Method,
				Pattern: routerpkg.RoutePattern(r.Context()),
			}
			rec.record(key, rw.statusCode(), elapsed.Milliseconds())
		})
	}
}

func (rec *Recorder) record(key MetricKey, status int, ms int64) {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	if rec.data == nil {
		rec.data = make(map[MetricKey]*routeEntry)
	}
	e := rec.data[key]
	if e == nil {
		e = &routeEntry{statusCodes: make(map[int]int64)}
		rec.data[key] = e
	}
	e.requests++
	e.statusCodes[status]++
	e.totalMs += ms
}

// Snapshot returns a point-in-time copy of all recorded metrics (ADR-012).
func (rec *Recorder) Snapshot() Snapshot {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	out := make(Snapshot, len(rec.data))
	for k, e := range rec.data {
		sc := make(map[int]int64, len(e.statusCodes))
		for code, n := range e.statusCodes {
			sc[code] = n
		}
		out[k] = RouteMetrics{
			Requests:    e.requests,
			StatusCodes: sc,
			TotalMs:     e.totalMs,
		}
	}
	return out
}

// ─── metricsRW ────────────────────────────────────────────────────────────────

// metricsRW is a thin http.ResponseWriter wrapper that captures the first
// status code written, for use by Recorder.Middleware.
type metricsRW struct {
	http.ResponseWriter
	status int
	wrote  bool
}

func (rw *metricsRW) WriteHeader(code int) {
	if !rw.wrote {
		rw.wrote = true
		rw.status = code
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *metricsRW) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.wrote = true
		rw.status = http.StatusOK
	}
	return rw.ResponseWriter.Write(b)
}

// statusCode returns the recorded status, defaulting to 200 if no header was
// written (the net/http server would have written 200 implicitly).
func (rw *metricsRW) statusCode() int {
	if !rw.wrote {
		return http.StatusOK
	}
	return rw.status
}
