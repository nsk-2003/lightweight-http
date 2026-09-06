// Purpose: Black-box tests for the observability package covering all Phase 7
// behavioral requirements: request ID generation/adoption/validation, structured
// logging field set, metrics counting/latency/status, route-pattern aggregation,
// panic resilience, and race-detector cleanliness.
package observability_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	httperrors "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// ─── In-memory slog handler ───────────────────────────────────────────────────

type memRecord struct {
	Level slog.Level
	Msg   string
	Attrs map[string]slog.Value
}

type memHandler struct {
	mu      sync.Mutex
	records []memRecord
}

func (h *memHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *memHandler) Handle(_ context.Context, r slog.Record) error {
	rec := memRecord{
		Level: r.Level,
		Msg:   r.Message,
		Attrs: make(map[string]slog.Value),
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.Attrs[a.Key] = a.Value
		return true
	})
	h.mu.Lock()
	h.records = append(h.records, rec)
	h.mu.Unlock()
	return nil
}

func (h *memHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *memHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *memHandler) Records() []memRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	out := make([]memRecord, len(h.records))
	copy(out, h.records)
	return out
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func newTestRouter(pattern string, status int) *router.Router {
	r := router.New()
	if err := r.GET(pattern, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}); err != nil {
		panic(err)
	}
	return r
}

func discardLog() *slog.Logger { return slog.New(&memHandler{}) }

// ─── Request ID: generation ───────────────────────────────────────────────────

func TestRequestID_GeneratesWhenAbsent(t *testing.T) {
	var gotIDInCtx string
	h := observability.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIDInCtx = httperrors.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeHTTP(rec, req)

	if gotIDInCtx == "" {
		t.Error("expected a request ID in context, got empty string")
	}
	got := rec.Header().Get("X-Request-ID")
	if got == "" {
		t.Error("expected X-Request-ID response header, got empty")
	}
	if got != gotIDInCtx {
		t.Errorf("response header %q != context ID %q", got, gotIDInCtx)
	}
}

// ─── Request ID: adoption ─────────────────────────────────────────────────────

func TestRequestID_AdoptsValidIncoming(t *testing.T) {
	const incoming = "my-request-id-123"
	var gotIDInCtx string
	h := observability.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotIDInCtx = httperrors.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", incoming)
	h.ServeHTTP(rec, req)

	if gotIDInCtx != incoming {
		t.Errorf("expected context ID %q, got %q", incoming, gotIDInCtx)
	}
	if got := rec.Header().Get("X-Request-ID"); got != incoming {
		t.Errorf("expected echoed header %q, got %q", incoming, got)
	}
}

// ─── Request ID: validation (over-long) ──────────────────────────────────────

func TestRequestID_ReplacesOverLongID(t *testing.T) {
	overlong := strings.Repeat("x", 129)
	var capturedID string
	h := observability.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = httperrors.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", overlong)
	h.ServeHTTP(rec, req)

	if capturedID == overlong {
		t.Error("over-long request ID should have been replaced with a generated one")
	}
	if capturedID == "" {
		t.Error("expected a generated request ID, got empty string")
	}
	if rec.Header().Get("X-Request-ID") != capturedID {
		t.Error("response header should match the generated context ID")
	}
}

// ─── Request ID: validation (non-printable) ───────────────────────────────────

func TestRequestID_ReplacesNonPrintableID(t *testing.T) {
	invalid := "id-with\x00null"
	var capturedID string
	h := observability.RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedID = httperrors.RequestIDFromContext(r.Context())
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", invalid)
	h.ServeHTTP(rec, req)

	if capturedID == invalid {
		t.Error("non-printable request ID should have been replaced")
	}
	if capturedID == "" {
		t.Error("expected a generated request ID, got empty string")
	}
}

// ─── Logger: standard fields ──────────────────────────────────────────────────

func TestLogger_StandardFields(t *testing.T) {
	mh := &memHandler{}
	log := slog.New(mh)

	r := newTestRouter("/hello", http.StatusOK)
	chain := middleware.Chain(
		observability.RequestID(),
		observability.Logger(log),
	)(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello", nil)
	req.RemoteAddr = "127.0.0.1:9000"
	chain.ServeHTTP(rec, req)

	records := mh.Records()
	if len(records) != 1 {
		t.Fatalf("expected 1 log record, got %d", len(records))
	}
	attrs := records[0].Attrs

	required := []string{"request_id", "method", "route", "path", "status", "duration_ms", "bytes", "remote_addr"}
	for _, field := range required {
		if _, ok := attrs[field]; !ok {
			t.Errorf("missing required log field %q", field)
		}
	}
	if attrs["method"].String() != http.MethodGet {
		t.Errorf("method: want %q, got %q", http.MethodGet, attrs["method"].String())
	}
	if attrs["path"].String() != "/hello" {
		t.Errorf("path: want %q, got %q", "/hello", attrs["path"].String())
	}
	if attrs["route"].String() != "/hello" {
		t.Errorf("route: want %q, got %q", "/hello", attrs["route"].String())
	}
	if attrs["status"].Int64() != http.StatusOK {
		t.Errorf("status: want 200, got %d", attrs["status"].Int64())
	}
	if attrs["remote_addr"].String() != "127.0.0.1:9000" {
		t.Errorf("remote_addr: want %q, got %q", "127.0.0.1:9000", attrs["remote_addr"].String())
	}
}

// ─── Logger: Authorization header never logged ────────────────────────────────

func TestLogger_NoAuthorizationHeader(t *testing.T) {
	mh := &memHandler{}
	log := slog.New(mh)

	r := newTestRouter("/secure", http.StatusOK)
	chain := middleware.Chain(
		observability.RequestID(),
		observability.Logger(log),
	)(r)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/secure", nil)
	req.Header.Set("Authorization", "Bearer super-secret-token-12345")
	chain.ServeHTTP(rec, req)

	const secret = "super-secret-token-12345"
	for _, record := range mh.Records() {
		if strings.Contains(record.Msg, secret) {
			t.Error("authorization token found in log message")
		}
		for key, val := range record.Attrs {
			if strings.Contains(val.String(), secret) {
				t.Errorf("authorization token found in log attr %q", key)
			}
		}
	}
}

// ─── Metrics: request count ───────────────────────────────────────────────────

func TestMetrics_CountsRequests(t *testing.T) {
	rec := &observability.Recorder{}
	r := newTestRouter("/ping", http.StatusOK)
	chain := middleware.Chain(
		observability.Logger(discardLog()),
		rec.Middleware(),
	)(r)

	const N = 5
	for i := 0; i < N; i++ {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/ping", nil)
		chain.ServeHTTP(w, req)
	}

	snap := rec.Snapshot()
	key := observability.MetricKey{Method: http.MethodGet, Pattern: "/ping"}
	m, ok := snap[key]
	if !ok {
		t.Fatalf("no metrics for key %+v; snapshot: %+v", key, snap)
	}
	if m.Requests != N {
		t.Errorf("want %d requests, got %d", N, m.Requests)
	}
}

// ─── Metrics: status code distribution ───────────────────────────────────────

func TestMetrics_StatusCodes(t *testing.T) {
	rec := &observability.Recorder{}

	r := router.New()
	_ = r.GET("/ok", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	_ = r.GET("/err", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })

	chain := middleware.Chain(
		observability.Logger(discardLog()),
		rec.Middleware(),
	)(r)

	for _, path := range []string{"/ok", "/ok", "/err"} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		chain.ServeHTTP(w, req)
	}

	snap := rec.Snapshot()
	var total200, total500 int64
	for _, m := range snap {
		total200 += m.StatusCodes[http.StatusOK]
		total500 += m.StatusCodes[http.StatusInternalServerError]
	}
	if total200 != 2 {
		t.Errorf("want 2×200, got %d", total200)
	}
	if total500 != 1 {
		t.Errorf("want 1×500, got %d", total500)
	}
}

// ─── Metrics: route-pattern aggregation ──────────────────────────────────────

func TestMetrics_AggregatesRoutePattern(t *testing.T) {
	rec := &observability.Recorder{}
	r := router.New()
	if err := r.GET("/users/:id", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}
	chain := middleware.Chain(
		observability.Logger(discardLog()),
		rec.Middleware(),
	)(r)

	paths := []string{"/users/1", "/users/2", "/users/abc"}
	for _, p := range paths {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, p, nil)
		chain.ServeHTTP(w, req)
	}

	snap := rec.Snapshot()
	key := observability.MetricKey{Method: http.MethodGet, Pattern: "/users/:id"}
	m, ok := snap[key]
	if !ok {
		t.Fatalf("no metrics for route pattern key %+v; full snapshot: %+v", key, snap)
	}
	if m.Requests != int64(len(paths)) {
		t.Errorf("want %d requests aggregated under pattern, got %d", len(paths), m.Requests)
	}
}

// ─── Metrics: latency non-negative ───────────────────────────────────────────

func TestMetrics_RecordsLatency(t *testing.T) {
	rec := &observability.Recorder{}
	r := newTestRouter("/fast", http.StatusNoContent)
	chain := middleware.Chain(rec.Middleware())(r)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/fast", nil)
	chain.ServeHTTP(w, req)

	snap := rec.Snapshot()
	if len(snap) == 0 {
		t.Fatal("expected at least one metric entry")
	}
	for _, m := range snap {
		if m.TotalMs < 0 {
			t.Errorf("TotalMs should be non-negative, got %d", m.TotalMs)
		}
	}
}

// ─── Metrics: panic — latency and status still recorded ──────────────────────

func TestMetrics_PanicLatencyAndStatusRecorded(t *testing.T) {
	rec := &observability.Recorder{}
	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	})

	// Metrics wraps Recovery: Metrics sees the 500 written by Recovery.
	chain := middleware.Chain(
		rec.Middleware(),
		middleware.Recovery(discardLog(), false),
	)(panicHandler)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/boom", nil)
	chain.ServeHTTP(w, req)

	snap := rec.Snapshot()
	if len(snap) == 0 {
		t.Fatal("expected metrics recorded even after a handler panic")
	}
	for _, m := range snap {
		if m.TotalMs < 0 {
			t.Errorf("TotalMs should be non-negative after panic, got %d", m.TotalMs)
		}
		if m.StatusCodes[http.StatusInternalServerError] != 1 {
			t.Errorf("expected one 500 status recorded, got %+v", m.StatusCodes)
		}
	}
}

// ─── Metrics: concurrent safety (race detector) ───────────────────────────────

func TestMetrics_ConcurrentRequests(t *testing.T) {
	rec := &observability.Recorder{}
	r := newTestRouter("/concurrent", http.StatusOK)
	chain := middleware.Chain(
		observability.Logger(discardLog()),
		rec.Middleware(),
	)(r)

	const goroutines = 50
	const perGoroutine = 10

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < perGoroutine; j++ {
				w := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodGet, "/concurrent", nil)
				chain.ServeHTTP(w, req)
			}
		}()
	}
	wg.Wait()

	snap := rec.Snapshot()
	key := observability.MetricKey{Method: http.MethodGet, Pattern: "/concurrent"}
	m, ok := snap[key]
	if !ok {
		t.Fatal("expected metrics for /concurrent route")
	}
	expected := int64(goroutines * perGoroutine)
	if m.Requests != expected {
		t.Errorf("want %d total requests (race-free), got %d", expected, m.Requests)
	}
}
