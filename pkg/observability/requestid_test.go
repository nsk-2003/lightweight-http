// Purpose: Tests for request ID middleware covering generation, adoption, validation, and echo.

package observability_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperr "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
)

const requestIDHeader = "X-Request-ID"

func makeRequestIDChain(next http.HandlerFunc) http.Handler {
	return middleware.New(observability.RequestID()).Then(next)
}

// TestRequestIDGenerated verifies that a request without X-Request-ID gets a generated ID
// present in both the response header and the request context.
func TestRequestIDGenerated(t *testing.T) {
	var gotID string
	handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
		gotID = httperr.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	handler.ServeHTTP(w, r)

	if gotID == "" {
		t.Error("expected non-empty request ID in context, got empty string")
	}
	if respID := w.Header().Get(requestIDHeader); respID != gotID {
		t.Errorf("response X-Request-ID=%q, want %q (context ID)", respID, gotID)
	}
}

// TestRequestIDAdopted verifies that a well-formed incoming X-Request-ID is adopted unchanged.
func TestRequestIDAdopted(t *testing.T) {
	const incoming = "valid-request-id-123"
	var gotID string
	handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
		gotID = httperr.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(requestIDHeader, incoming)
	handler.ServeHTTP(w, r)

	if gotID != incoming {
		t.Errorf("request ID in context = %q, want %q", gotID, incoming)
	}
	if respID := w.Header().Get(requestIDHeader); respID != incoming {
		t.Errorf("response X-Request-ID=%q, want %q", respID, incoming)
	}
}

// TestRequestIDOverLong verifies that an over-long incoming X-Request-ID is replaced.
func TestRequestIDOverLong(t *testing.T) {
	longID := strings.Repeat("a", 200)
	var gotID string
	handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
		gotID = httperr.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(requestIDHeader, longID)
	handler.ServeHTTP(w, r)

	if gotID == longID {
		t.Error("over-long request ID should be replaced, not adopted")
	}
	if gotID == "" {
		t.Error("expected a generated replacement request ID, got empty string")
	}
	if respID := w.Header().Get(requestIDHeader); respID != gotID {
		t.Errorf("response X-Request-ID=%q does not match generated ID %q", respID, gotID)
	}
}

// TestRequestIDInvalidChars verifies that a request ID with forbidden characters is replaced.
func TestRequestIDInvalidChars(t *testing.T) {
	tests := []struct {
		name string
		id   string
	}{
		{"space", "bad value"},
		{"bang", "id!"},
		{"slash", "a/b"},
		{"null byte", "ab\x00cd"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var gotID string
			handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
				gotID = httperr.RequestIDFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set(requestIDHeader, tc.id)
			handler.ServeHTTP(w, r)

			if gotID == tc.id {
				t.Errorf("invalid request ID %q should be replaced, not adopted", tc.id)
			}
			if gotID == "" {
				t.Error("expected a generated replacement request ID, got empty string")
			}
		})
	}
}

// TestRequestIDEmpty verifies that an empty X-Request-ID header causes a new ID to be generated.
func TestRequestIDEmpty(t *testing.T) {
	var gotID string
	handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
		gotID = httperr.RequestIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set(requestIDHeader, "")
	handler.ServeHTTP(w, r)

	if gotID == "" {
		t.Error("expected generated request ID for empty header, got empty string")
	}
}

// TestRequestIDUnique verifies that successive requests without an incoming ID get distinct IDs.
func TestRequestIDUnique(t *testing.T) {
	ids := make(map[string]bool)
	handler := makeRequestIDChain(func(w http.ResponseWriter, r *http.Request) {
		id := httperr.RequestIDFromContext(r.Context())
		ids[id] = true
		w.WriteHeader(http.StatusOK)
	})

	for i := 0; i < 10; i++ {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodGet, "/", nil)
		handler.ServeHTTP(w, r)
	}

	if len(ids) != 10 {
		t.Errorf("got %d unique IDs for 10 requests, want 10", len(ids))
	}
}
