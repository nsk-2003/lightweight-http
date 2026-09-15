// Purpose: Tests for the middleware package covering chain composition, execution order,
// short-circuiting, context propagation, panic recovery, and concurrency safety.

package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
)

// record returns a Middleware that appends name+":before" and name+":after" to seq
// around the next handler call. mu serializes access to seq.
func record(name string, mu *sync.Mutex, seq *[]string) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			*seq = append(*seq, name+":before")
			mu.Unlock()
			next.ServeHTTP(w, r)
			mu.Lock()
			*seq = append(*seq, name+":after")
			mu.Unlock()
		})
	}
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// TestExecutionOrder asserts A→B→C→handler request order and handler→C→B→A response order
// by recording a sequence, satisfying the spec requirement that order is verified by observation.
func TestExecutionOrder(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	chain := New(
		record("A", &mu, &seq),
		record("B", &mu, &seq),
		record("C", &mu, &seq),
	)
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{
		"A:before", "B:before", "C:before",
		"handler",
		"C:after", "B:after", "A:after",
	}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("execution order = %v\nwant              %v", seq, want)
	}
}

// sentinelHandler is a minimal http.Handler used to verify handler identity.
// It uses a pointer type so that interface-value equality is well-defined.
type sentinelHandler struct{ called *bool }

func (s *sentinelHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	*s.called = true
	w.WriteHeader(http.StatusOK)
}

// TestChainEmpty verifies that composing zero middleware returns the original handler unchanged.
func TestChainEmpty(t *testing.T) {
	called := false
	original := &sentinelHandler{called: &called}

	got := New().Then(original)
	// Interface values are comparable when the dynamic type is a pointer.
	if got != http.Handler(original) {
		t.Error("empty chain must return the original handler unchanged (same interface value)")
	}

	w := httptest.NewRecorder()
	got.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Error("handler not called through empty chain")
	}
}

// TestChainShortCircuit verifies that middleware that does not call next prevents the handler
// from running and the response is whatever the middleware wrote.
func TestChainShortCircuit(t *testing.T) {
	handlerCalled := false

	shortCircuit := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "blocked", http.StatusForbidden)
			// deliberately does not call next.ServeHTTP
		})
	})

	chain := New(shortCircuit)
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerCalled = true
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if handlerCalled {
		t.Error("handler must not be called when middleware short-circuits")
	}
	if w.Code != http.StatusForbidden {
		t.Errorf("status = %d, want 403", w.Code)
	}
}

// TestContextPropagation verifies that a context value set in middleware A is visible
// in subsequent middleware B and in the handler, using an unexported key type (ADR-005).
func TestContextPropagation(t *testing.T) {
	type ctxKey struct{}

	setterMW := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), ctxKey{}, "propagated")
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	})

	var gotInB, gotInHandler string
	checkerMW := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotInB, _ = r.Context().Value(ctxKey{}).(string)
			next.ServeHTTP(w, r)
		})
	})

	chain := New(setterMW, checkerMW)
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotInHandler, _ = r.Context().Value(ctxKey{}).(string)
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	if gotInB != "propagated" {
		t.Errorf("ctx value in B = %q, want %q", gotInB, "propagated")
	}
	if gotInHandler != "propagated" {
		t.Errorf("ctx value in handler = %q, want %q", gotInHandler, "propagated")
	}
}

// TestRecoveryPanic verifies that a panic in the handler produces a 500 response and
// does not propagate to the caller.
func TestRecoveryPanic(t *testing.T) {
	mw := Recovery(discardLogger())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("something went wrong")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestRecoveryErrAbortHandler verifies that http.ErrAbortHandler is re-panicked rather
// than converted to a 500 response, so the server can clean up the connection (ADR-007).
func TestRecoveryErrAbortHandler(t *testing.T) {
	mw := Recovery(discardLogger())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic(http.ErrAbortHandler)
	}))

	defer func() {
		val := recover()
		if val != http.ErrAbortHandler {
			t.Errorf("expected re-panic with http.ErrAbortHandler, got %v", val)
		}
	}()

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}

// TestRecoveryAfterPartialResponse verifies that when a handler begins writing a response
// then panics, Recovery does not attempt to overwrite the already-committed header.
func TestRecoveryAfterPartialResponse(t *testing.T) {
	mw := Recovery(discardLogger())
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK) // header committed
		panic("panic after header")
	}))

	w := httptest.NewRecorder()
	// Should not panic from the test's perspective.
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	// Status 200 was written by the handler; recovery must not change it.
	if w.Code != http.StatusOK {
		t.Errorf("status = %d, want 200 (original header must not be overwritten)", w.Code)
	}
}

// TestRecoveryNilLogger verifies that Recovery works when logger is nil, using slog.Default().
func TestRecoveryNilLogger(t *testing.T) {
	mw := Recovery(nil)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("nil-logger test")
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
}

// TestChainAppend verifies that Append returns a new immutable Chain and does not modify
// the receiver.
func TestChainAppend(t *testing.T) {
	var seq []string
	mu := &sync.Mutex{}

	base := New(record("A", mu, &seq))
	extended := base.Append(record("B", mu, &seq))

	// base still has only one middleware
	base.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler-base")
		mu.Unlock()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	seq = seq[:0] // reset

	// extended has both
	extended.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler-ext")
		mu.Unlock()
	})).ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{"A:before", "B:before", "handler-ext", "B:after", "A:after"}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("extended chain order = %v, want %v", seq, want)
	}
}

// TestConcurrentRequests drives the same chain from many goroutines concurrently.
// The race detector must find no data races.
func TestConcurrentRequests(t *testing.T) {
	const goroutines = 20
	const reqsPerGoroutine = 50

	var counter int64
	mw := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt64(&counter, 1)
			next.ServeHTTP(w, r)
		})
	})

	handler := New(mw).Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < reqsPerGoroutine; j++ {
				handler.ServeHTTP(
					httptest.NewRecorder(),
					httptest.NewRequest(http.MethodGet, "/", nil),
				)
			}
		}()
	}
	wg.Wait()

	want := int64(goroutines * reqsPerGoroutine)
	if counter != want {
		t.Errorf("counter = %d, want %d", counter, want)
	}
}

// TestMiddlewareWritesHeaderThenCallsNext verifies that a header set by middleware
// before calling next is present in the final response.
func TestMiddlewareWritesHeaderThenCallsNext(t *testing.T) {
	addHeader := Middleware(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Middleware", "present")
			next.ServeHTTP(w, r)
		})
	})

	handler := New(addHeader).Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	handler.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Header().Get("X-Middleware"); got != "present" {
		t.Errorf("X-Middleware = %q, want %q", got, "present")
	}
}
