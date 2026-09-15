// Purpose: Tests for structured request logging middleware covering field set, redaction, and error propagation.

package observability_test

import (
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	httperr "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// captureHandler implements slog.Handler and records every log record for assertions.
type captureHandler struct {
	mu      sync.Mutex
	records []capturedRecord
}

type capturedRecord struct {
	msg   string
	level slog.Level
	attrs map[string]slog.Value
}

func (h *captureHandler) Enabled(_ context.Context, _ slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{
		msg:   r.Message,
		level: r.Level,
		attrs: make(map[string]slog.Value),
	}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, rec)
	return nil
}

func (h *captureHandler) WithAttrs(attrs []slog.Attr) slog.Handler { return h }
func (h *captureHandler) WithGroup(name string) slog.Handler       { return h }

func (h *captureHandler) last() (capturedRecord, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.records) == 0 {
		return capturedRecord{}, false
	}
	return h.records[len(h.records)-1], true
}

// TestLoggingStandardFields verifies that every required field appears in the log record.
func TestLoggingStandardFields(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	ro := router.New()
	ro.Use(observability.RequestID(), observability.LoggingMiddleware(logger))
	if err := ro.GET("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("pong"))
	}); err != nil {
		t.Fatal(err)
	}

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ping", nil))

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected at least one log record, got none")
	}
	if rec.msg != "request" {
		t.Errorf("log msg = %q, want %q", rec.msg, "request")
	}

	required := []string{"request_id", "method", "route", "path", "status", "duration_ms", "bytes", "remote_addr"}
	for _, field := range required {
		if _, present := rec.attrs[field]; !present {
			t.Errorf("log record missing required field %q", field)
		}
	}

	if rec.attrs["method"].String() != "GET" {
		t.Errorf("method = %q, want %q", rec.attrs["method"].String(), "GET")
	}
	if rec.attrs["path"].String() != "/ping" {
		t.Errorf("path = %q, want %q", rec.attrs["path"].String(), "/ping")
	}
	if rec.attrs["route"].String() != "/ping" {
		t.Errorf("route = %q, want %q", rec.attrs["route"].String(), "/ping")
	}
	if rec.attrs["status"].Int64() != 200 {
		t.Errorf("status = %d, want 200", rec.attrs["status"].Int64())
	}
}

// TestLoggingAuthorizationHeaderAbsent verifies that a request carrying an Authorization
// header produces no log line containing its value.
func TestLoggingAuthorizationHeaderAbsent(t *testing.T) {
	const secret = "Bearer supersecret-token-xyz"
	h := &captureHandler{}
	logger := slog.New(h)

	chain := middleware.New(observability.LoggingMiddleware(logger))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/secure", nil)
	r.Header.Set("Authorization", secret)
	handler.ServeHTTP(w, r)

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected a log record")
	}

	// No attribute value should contain the secret token.
	rec.attrs["_all"] = slog.StringValue("") // force iteration
	for key, val := range rec.attrs {
		s := val.String()
		if strings.Contains(s, secret) || strings.Contains(s, "supersecret") {
			t.Errorf("log field %q contains secret authorization value: %q", key, s)
		}
		if strings.EqualFold(key, "authorization") {
			t.Errorf("log record must not have an 'authorization' key, found %q", key)
		}
	}
}

// TestLoggingCookieHeaderAbsent verifies that Cookie header values are not logged.
func TestLoggingCookieHeaderAbsent(t *testing.T) {
	const cookieVal = "session=topsecretcookie"
	h := &captureHandler{}
	logger := slog.New(h)

	chain := middleware.New(observability.LoggingMiddleware(logger))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Cookie", cookieVal)
	handler.ServeHTTP(w, r)

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected a log record")
	}
	for key, val := range rec.attrs {
		if strings.Contains(val.String(), "topsecretcookie") {
			t.Errorf("log field %q contains cookie value", key)
		}
		if strings.EqualFold(key, "cookie") {
			t.Errorf("log record must not have a 'cookie' key, found %q", key)
		}
	}
}

// TestLoggingErrorFieldIncluded verifies that the error field appears when SetLogError is called.
func TestLoggingErrorFieldIncluded(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	chain := middleware.New(observability.LoggingMiddleware(logger))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		observability.SetLogError(r.Context(), httperr.NotFound("item not found"))
		w.WriteHeader(http.StatusNotFound)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected a log record")
	}
	if _, present := rec.attrs["error"]; !present {
		t.Error("log record missing 'error' field when SetLogError was called")
	}
}

// TestLoggingErrorFieldOmittedWhenNil verifies that the error field is absent for successful requests.
func TestLoggingErrorFieldOmittedWhenNil(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	chain := middleware.New(observability.LoggingMiddleware(logger))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/ok", nil))

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected a log record")
	}
	if _, present := rec.attrs["error"]; present {
		t.Error("log record must not include 'error' field when no error was set")
	}
}

// TestLoggingRoutePatternFromRouter verifies that the route field reflects the route pattern
// rather than the concrete path when the router is in use.
func TestLoggingRoutePatternFromRouter(t *testing.T) {
	h := &captureHandler{}
	logger := slog.New(h)

	ro := router.New()
	ro.Use(observability.LoggingMiddleware(logger))
	if err := ro.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/42", nil))

	rec, ok := h.last()
	if !ok {
		t.Fatal("expected a log record")
	}
	if rec.attrs["route"].String() != "/users/:id" {
		t.Errorf("route = %q, want %q", rec.attrs["route"].String(), "/users/:id")
	}
	if rec.attrs["path"].String() != "/users/42" {
		t.Errorf("path = %q, want %q", rec.attrs["path"].String(), "/users/42")
	}
}

// TestLoggingNilLoggerFallsBack verifies that passing nil logger does not panic.
func TestLoggingNilLoggerFallsBack(t *testing.T) {
	chain := middleware.New(observability.LoggingMiddleware(nil))
	handler := chain.Then(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	// Should not panic.
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
}
