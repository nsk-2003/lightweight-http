// Purpose: Tests for in-process metrics collection covering counts, patterns, status codes, and concurrency.

package observability_test

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// TestMetricsAccurateCount verifies that request counts are exact after a known sequence.
func TestMetricsAccurateCount(t *testing.T) {
	collector := observability.NewInProcessCollector()
	ro := router.New()
	ro.Use(observability.MetricsMiddleware(collector))
	if err := ro.GET("/hello", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 3; i++ {
		ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/hello", nil))
	}

	snap := collector.Snapshot()
	key := observability.MetricKey{Route: "/hello", Method: http.MethodGet}
	if snap[key].RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3", snap[key].RequestCount)
	}
	if snap[key].StatusCodes[200] != 3 {
		t.Errorf("StatusCodes[200] = %d, want 3", snap[key].StatusCodes[200])
	}
}

// TestMetricsPatternAggregation verifies that multiple concrete paths matching one route
// pattern are counted under the same metric key.
func TestMetricsPatternAggregation(t *testing.T) {
	collector := observability.NewInProcessCollector()
	ro := router.New()
	ro.Use(observability.MetricsMiddleware(collector))
	if err := ro.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	for _, path := range []string{"/users/1", "/users/2", "/users/3"} {
		ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, path, nil))
	}

	snap := collector.Snapshot()
	key := observability.MetricKey{Route: "/users/:id", Method: http.MethodGet}
	if snap[key].RequestCount != 3 {
		t.Errorf("RequestCount = %d, want 3 (all paths aggregated under route pattern)", snap[key].RequestCount)
	}
}

// TestMetricsStatusCodesSeparate verifies that 200 and 500 responses are tallied separately.
func TestMetricsStatusCodesSeparate(t *testing.T) {
	collector := observability.NewInProcessCollector()
	ro := router.New()
	ro.Use(observability.MetricsMiddleware(collector))
	if err := ro.GET("/ok", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}
	if err := ro.GET("/fail", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}); err != nil {
		t.Fatal(err)
	}

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))
	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/fail", nil))

	snap := collector.Snapshot()
	okKey := observability.MetricKey{Route: "/ok", Method: http.MethodGet}
	failKey := observability.MetricKey{Route: "/fail", Method: http.MethodGet}

	if snap[okKey].StatusCodes[200] != 1 {
		t.Errorf("ok 200 count = %d, want 1", snap[okKey].StatusCodes[200])
	}
	if snap[failKey].StatusCodes[500] != 1 {
		t.Errorf("fail 500 count = %d, want 1", snap[failKey].StatusCodes[500])
	}
}

// TestMetricsConcurrent verifies that counts are exact under concurrent access (race detector).
func TestMetricsConcurrent(t *testing.T) {
	collector := observability.NewInProcessCollector()
	chain := middleware.New(observability.MetricsMiddleware(collector))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	const goroutines = 50
	const requestsEach = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsEach; j++ {
				handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/concurrent", nil))
			}
		}()
	}
	wg.Wait()

	var total int64
	for _, v := range collector.Snapshot() {
		total += v.RequestCount
	}
	want := int64(goroutines * requestsEach)
	if total != want {
		t.Errorf("total request count = %d, want %d", total, want)
	}
}

// TestMetricsLatencyRecordedOnPanic verifies that latency and status are recorded even
// when the handler panics (recovery runs inside the metrics middleware).
func TestMetricsLatencyRecordedOnPanic(t *testing.T) {
	collector := observability.NewInProcessCollector()
	ro := router.New()
	// metrics wraps recovery wraps handler — so recovery's 500 is visible to metrics.
	ro.Use(observability.MetricsMiddleware(collector), middleware.Recovery(nil))
	if err := ro.GET("/panic", func(w http.ResponseWriter, r *http.Request) {
		panic("test panic")
	}); err != nil {
		t.Fatal(err)
	}

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/panic", nil))

	snap := collector.Snapshot()
	key := observability.MetricKey{Route: "/panic", Method: http.MethodGet}
	if snap[key].RequestCount != 1 {
		t.Errorf("RequestCount after panic = %d, want 1", snap[key].RequestCount)
	}
	if snap[key].StatusCodes[500] != 1 {
		t.Errorf("StatusCodes[500] after panic = %d, want 1", snap[key].StatusCodes[500])
	}
	if snap[key].TotalLatencyMs < 0 {
		t.Error("TotalLatencyMs should be non-negative")
	}
}

// TestMetricsSnapshotIsCopy verifies that mutating the snapshot does not affect the collector.
func TestMetricsSnapshotIsCopy(t *testing.T) {
	collector := observability.NewInProcessCollector()
	chain := middleware.New(observability.MetricsMiddleware(collector))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/x", nil))

	snap1 := collector.Snapshot()
	// Mutate snapshot
	for k, v := range snap1 {
		v.RequestCount = 9999
		snap1[k] = v
	}

	snap2 := collector.Snapshot()
	for _, v := range snap2 {
		if v.RequestCount == 9999 {
			t.Error("snapshot mutation leaked into the collector")
		}
	}
}
