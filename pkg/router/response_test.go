// Purpose: Tests for the ResponseWriter wrapper and the content-negotiated write helpers,
// covering every row of the Phase 4 response behavioral requirements table.

package router

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperr "github.com/example/lightweight-http/pkg/errors"
)

// discardLogger returns a slog.Logger that discards all output.
func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// ---- ResponseWriter tracking tests ------------------------------------------

// TestResponseWriterStatus verifies that Status() returns the code set by WriteHeader.
func TestResponseWriterStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := WrapResponseWriter(rec, discardLogger())
	rw.WriteHeader(http.StatusCreated)
	if rw.Status() != http.StatusCreated {
		t.Errorf("Status() = %d, want 201", rw.Status())
	}
}

// TestResponseWriterStatusDefault verifies that Status() returns 200 when WriteHeader is never called.
func TestResponseWriterStatusDefault(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := WrapResponseWriter(rec, discardLogger())
	if rw.Status() != http.StatusOK {
		t.Errorf("Status() before WriteHeader = %d, want 200", rw.Status())
	}
}

// TestResponseWriterBytesWritten verifies that BytesWritten accumulates correctly.
func TestResponseWriterBytesWritten(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := WrapResponseWriter(rec, discardLogger())
	rw.WriteHeader(http.StatusOK)
	_, _ = rw.Write([]byte("hello"))
	_, _ = rw.Write([]byte(" world"))
	if rw.BytesWritten() != 11 {
		t.Errorf("BytesWritten() = %d, want 11", rw.BytesWritten())
	}
}

// TestResponseWriterWriteHeader_OnlyOnce verifies that a second WriteHeader call is a no-op.
func TestResponseWriterWriteHeaderOnlyOnce(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := WrapResponseWriter(rec, discardLogger())
	rw.WriteHeader(http.StatusOK)
	rw.WriteHeader(http.StatusInternalServerError) // must be ignored
	if rw.Status() != http.StatusOK {
		t.Errorf("Status() = %d after duplicate WriteHeader, want 200", rw.Status())
	}
	if rec.Code != http.StatusOK {
		t.Errorf("underlying recorder code = %d, want 200", rec.Code)
	}
}

// TestResponseWriterWriteImpliesOK verifies that Write without a preceding WriteHeader
// causes Status() to return 200.
func TestResponseWriterWriteImpliesOK(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := WrapResponseWriter(rec, discardLogger())
	_, _ = rw.Write([]byte("body"))
	if rw.Status() != http.StatusOK {
		t.Errorf("Status() after implicit write = %d, want 200", rw.Status())
	}
}

// ---- WriteJSON tests --------------------------------------------------------

// TestWriteJSONAcceptsJSON verifies that Accept: application/json gets a JSON response.
func TestWriteJSONAcceptsJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")

	if err := WriteJSON(rec, r, http.StatusOK, map[string]string{"k": "v"}); err != nil {
		t.Fatalf("WriteJSON: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
	var got map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
}

// TestWriteJSONWildcardAccept verifies that Accept: */* gets a JSON response.
func TestWriteJSONWildcardAccept(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "*/*")

	if err := WriteJSON(rec, r, http.StatusOK, "value"); err != nil {
		t.Fatalf("WriteJSON with */*: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestWriteJSONAbsentAccept verifies that absent Accept gets a JSON response.
func TestWriteJSONAbsentAccept(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// no Accept header

	if err := WriteJSON(rec, r, http.StatusOK, 42); err != nil {
		t.Fatalf("WriteJSON without Accept: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// ---- WriteText tests --------------------------------------------------------

// TestWriteTextAcceptsPlain verifies that Accept: text/plain gets a text response.
func TestWriteTextAcceptsPlain(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "text/plain")

	if err := WriteText(rec, r, http.StatusOK, "hello"); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != "hello" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello")
	}
}

// ---- Respond tests (content negotiation) ------------------------------------

// TestRespondJSONAccept verifies that Respond with Accept: application/json returns JSON.
func TestRespondJSONAccept(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/json")

	if err := Respond(rec, r, http.StatusOK, map[string]int{"count": 1}); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	var got map[string]int
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("body not valid JSON: %v", err)
	}
}

// TestRespondTextAccept verifies that Respond with Accept: text/plain returns plain text.
func TestRespondTextAccept(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "text/plain")

	if err := Respond(rec, r, http.StatusOK, "hello text"); err != nil {
		t.Fatalf("Respond: %v", err)
	}
	if !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/plain") {
		t.Errorf("Content-Type = %q, want text/plain", rec.Header().Get("Content-Type"))
	}
	if rec.Body.String() != "hello text" {
		t.Errorf("body = %q, want %q", rec.Body.String(), "hello text")
	}
}

// TestRespondWildcardDefaultsToJSON verifies that Accept: */* returns JSON.
func TestRespondWildcardDefaultsToJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "*/*")

	if err := Respond(rec, r, http.StatusOK, "value"); err != nil {
		t.Fatalf("Respond with */*: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestRespondAbsentAcceptDefaultsToJSON verifies that absent Accept returns JSON.
func TestRespondAbsentAcceptDefaultsToJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	// no Accept header

	if err := Respond(rec, r, http.StatusOK, true); err != nil {
		t.Fatalf("Respond without Accept: %v", err)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestRespondUnsupportedAccept verifies that an unsupported Accept returns a 406 error.
func TestRespondUnsupportedAccept(t *testing.T) {
	rec := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Header.Set("Accept", "application/xml")

	err := Respond(rec, r, http.StatusOK, "data")
	if err == nil {
		t.Fatal("expected 406 error for unsupported Accept, got nil")
	}
	he, ok := err.(*httperr.HTTPError)
	if !ok {
		t.Fatalf("error type = %T, want *httperr.HTTPError", err)
	}
	if he.Status != http.StatusNotAcceptable {
		t.Errorf("status = %d, want 406", he.Status)
	}
	// Verify nothing was written to the response.
	if rec.Code != http.StatusOK { // httptest default
		t.Errorf("recorder code = %d, body should not have been written", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body should be empty on 406, got %q", rec.Body.String())
	}
}

// ---- NoContent test ---------------------------------------------------------

// TestNoContent verifies that NoContent writes a 204 with no body and no Content-Type.
func TestNoContent(t *testing.T) {
	rec := httptest.NewRecorder()
	NoContent(rec)
	if rec.Code != http.StatusNoContent {
		t.Errorf("status = %d, want 204", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("body = %q, want empty", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "" {
		t.Errorf("Content-Type = %q, want empty for 204", ct)
	}
}
