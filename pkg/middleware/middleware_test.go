// Purpose: Black-box tests for the middleware package covering all Phase 3
// behavioral requirements: execution order, short-circuit, context propagation,
// header forwarding, panic recovery, ErrAbortHandler re-panic, and concurrency.
package middleware_test

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/example/lightweight-http/pkg/middleware"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// newTestLogger returns a slog.Logger that writes to w at DEBUG level.
func newTestLogger(w io.Writer) *slog.Logger {
	return slog.New(slog.NewTextHandler(w, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// makeRecorder returns a Middleware that appends name to *seq on every request.
func makeRecorder(name string, seq *[]string, mu *sync.Mutex) middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			*seq = append(*seq, name+"-before")
			mu.Unlock()
			next.ServeHTTP(w, r)
			mu.Lock()
			*seq = append(*seq, name+"-after")
			mu.Unlock()
		})
	}
}

// ─── Chain ────────────────────────────────────────────────────────────────────

// TestChain_ExecutionOrder asserts the request sequence A→B→C→handler and the
// response sequence handler→C→B→A by recording named events into a slice.
func TestChain_ExecutionOrder(t *testing.T) {
	var seq []string
	var mu sync.Mutex

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.Chain(
		makeRecorder("A", &seq, &mu),
		makeRecorder("B", &seq, &mu),
		makeRecorder("C", &seq, &mu),
	)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := []string{
		"A-before", "B-before", "C-before",
		"handler",
		"C-after", "B-after", "A-after",
	}
	if !reflect.DeepEqual(seq, want) {
		t.Fatalf("execution order\n got:  %v\n want: %v", seq, want)
	}
}

// TestChain_ZeroMiddleware_ReturnsOriginalHandler asserts that composing zero
// middleware returns a handler with identical behavior to the original.
func TestChain_ZeroMiddleware_ReturnsOriginalHandler(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})

	h := middleware.Chain()(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusTeapot {
		t.Fatalf("expected 418, got %d", rec.Code)
	}
}

// TestChain_ShortCircuit asserts that a middleware that does not call next
// prevents the handler from running and controls the response.
func TestChain_ShortCircuit(t *testing.T) {
	handlerCalled := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
		w.WriteHeader(http.StatusOK)
	})

	stopMW := middleware.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			// intentionally does not call next
		})
	})

	h := middleware.Chain(stopMW)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if handlerCalled {
		t.Fatal("handler must not run after short-circuit middleware")
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", rec.Code)
	}
}

// contextKey is a private type used only in these tests (ADR-005).
type contextKey struct{}

// TestChain_ContextPropagation asserts that a value written into the context by
// middleware A is visible to the handler downstream.
func TestChain_ContextPropagation(t *testing.T) {
	setMW := middleware.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), contextKey{}, "propagated")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	var got string
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ = r.Context().Value(contextKey{}).(string)
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.Chain(setMW)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got != "propagated" {
		t.Fatalf("expected context value %q, got %q", "propagated", got)
	}
}

// TestChain_MiddlewareHeaderForwarding asserts that a response header set by
// middleware before calling next is visible in the final response.
func TestChain_MiddlewareHeaderForwarding(t *testing.T) {
	addHeaderMW := middleware.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Added-By", "middleware")
			next.ServeHTTP(w, r)
		})
	})

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	h := middleware.Chain(addHeaderMW)(handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Added-By"); got != "middleware" {
		t.Fatalf("expected X-Added-By=middleware, got %q", got)
	}
}

// TestChain_GroupComposesWithGlobal asserts that a global chain followed by a
// group-level chain executes in parent-to-child order (A→B→C→handler).
func TestChain_GroupComposesWithGlobal(t *testing.T) {
	var seq []string
	record := func(name string) middleware.Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				seq = append(seq, name)
				next.ServeHTTP(w, r)
			})
		}
	}

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seq = append(seq, "handler")
		w.WriteHeader(http.StatusOK)
	})

	global := middleware.Chain(record("A"), record("B"))
	group := middleware.Chain(record("C"))
	// Simulates: global chain wraps the group-chain-wrapped handler.
	h := global(group(handler))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	want := []string{"A", "B", "C", "handler"}
	if !reflect.DeepEqual(seq, want) {
		t.Fatalf("group composition order\n got:  %v\n want: %v", seq, want)
	}
}

// TestChain_Concurrent_NoRace drives the same chain from many goroutines to
// verify there are no data races (run with go test -race).
func TestChain_Concurrent_NoRace(t *testing.T) {
	const goroutines = 20
	const requestsEach = 50

	passThroughMW := middleware.Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})

	h := middleware.Chain(passThroughMW, passThroughMW, passThroughMW)(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}),
	)

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			for j := 0; j < requestsEach; j++ {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				h.ServeHTTP(rec, req)
			}
		}()
	}
	wg.Wait()
}

// ─── Recovery ─────────────────────────────────────────────────────────────────

// TestRecovery_PanicConvertsTo500 asserts that a handler panic yields a 500
// response and a log entry containing "panic recovered".
func TestRecovery_PanicConvertsTo500(t *testing.T) {
	var buf strings.Builder
	log := newTestLogger(&buf)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	})

	h := middleware.Recovery(log)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Fatalf("expected 'panic recovered' in log output; got:\n%s", buf.String())
	}
}

// TestRecovery_ErrAbortHandler_Repanics asserts that Recovery re-panics on
// http.ErrAbortHandler so the server's abort handling is preserved (ADR-007).
func TestRecovery_ErrAbortHandler_Repanics(t *testing.T) {
	log := newTestLogger(io.Discard)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	})

	h := middleware.Recovery(log)(panicHandler)

	var caught interface{}
	func() {
		defer func() { caught = recover() }()
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
	}()

	if caught != http.ErrAbortHandler {
		t.Fatalf("expected ErrAbortHandler re-panic, got %v", caught)
	}
}

// TestRecovery_AlreadyWroteHeader_LogsAndDoesNotWrite500 asserts that Recovery
// logs the incident but does not attempt to write a 500 when the response header
// has already been committed by the handler.
func TestRecovery_AlreadyWroteHeader_LogsAndDoesNotWrite500(t *testing.T) {
	var buf strings.Builder
	log := newTestLogger(&buf)

	panicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // commits the header
		panic("panic after header written")
	})

	h := middleware.Recovery(log)(panicHandler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	// Status must be what the handler wrote, not 500.
	if rec.Code != http.StatusOK {
		t.Fatalf("expected original status 200, got %d", rec.Code)
	}
	if !strings.Contains(buf.String(), "panic recovered") {
		t.Fatalf("expected 'panic recovered' in log output; got:\n%s", buf.String())
	}
}
