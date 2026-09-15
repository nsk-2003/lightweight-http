// Purpose: In-process request metrics collection and middleware; keyed by route pattern and method (ADR-012).

package observability

import (
	"net/http"
	"sync"
	"time"

	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/router"
)

// MetricKey identifies a distinct route-pattern + HTTP-method combination.
// Metrics are keyed by pattern (e.g. "/users/:id"), never by the concrete path,
// to prevent unbounded cardinality (ADR-012).
type MetricKey struct {
	Route  string
	Method string
}

// MetricSnapshot is a point-in-time, read-only view of the metrics for one MetricKey.
type MetricSnapshot struct {
	// RequestCount is the total number of requests handled under this key.
	RequestCount int64
	// TotalLatencyMs is the cumulative latency of all requests in milliseconds.
	TotalLatencyMs float64
	// StatusCodes maps each HTTP status code to the number of responses with that code.
	StatusCodes map[int]int64
}

// Collector exposes a snapshot of the collected in-process metrics.
type Collector interface {
	// Snapshot returns a point-in-time copy of all metrics. The returned map is safe
	// to read and mutate without affecting the collector.
	Snapshot() map[MetricKey]MetricSnapshot
}

// metricEntry is the mutable internal store for one MetricKey.
type metricEntry struct {
	requestCount   int64
	totalLatencyMs float64
	statusCodes    map[int]int64
}

// InProcessCollector holds request metrics in memory, protected by a mutex so it is
// safe for concurrent use (ADR-012).
type InProcessCollector struct {
	mu   sync.Mutex
	data map[MetricKey]*metricEntry
}

// NewInProcessCollector returns a ready-to-use in-process metrics collector.
func NewInProcessCollector() *InProcessCollector {
	return &InProcessCollector{
		data: make(map[MetricKey]*metricEntry),
	}
}

// record adds one request's measurements to the collector.
func (c *InProcessCollector) record(key MetricKey, latencyMs float64, status int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok {
		e = &metricEntry{statusCodes: make(map[int]int64)}
		c.data[key] = e
	}
	e.requestCount++
	e.totalLatencyMs += latencyMs
	e.statusCodes[status]++
}

// Snapshot returns a deep copy of all collected metrics.
// The returned map is independent of the collector's internal state.
func (c *InProcessCollector) Snapshot() map[MetricKey]MetricSnapshot {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make(map[MetricKey]MetricSnapshot, len(c.data))
	for k, e := range c.data {
		codes := make(map[int]int64, len(e.statusCodes))
		for code, count := range e.statusCodes {
			codes[code] = count
		}
		out[k] = MetricSnapshot{
			RequestCount:   e.requestCount,
			TotalLatencyMs: e.totalLatencyMs,
			StatusCodes:    codes,
		}
	}
	return out
}

// MetricsMiddleware returns a Middleware that records request count, latency, and response
// status for every request. Metrics are aggregated by route pattern and HTTP method.
//
// When no route pattern is available (the router did not dispatch the request), the raw
// request path is used as a fallback key.
//
// Place this middleware outside Recovery so that panicking handlers still produce a recorded
// status code (see package-level ordering guidance).
func MetricsMiddleware(c *InProcessCollector) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := newStatusRecorder(w)
			start := time.Now()
			next.ServeHTTP(rec, r)
			latencyMs := float64(time.Since(start)) / float64(time.Millisecond)

			route := router.RoutePattern(r)
			if route == "" {
				route = r.URL.Path
			}
			c.record(MetricKey{Route: route, Method: r.Method}, latencyMs, rec.status)
		})
	}
}
