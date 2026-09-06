// Purpose: Black-box tests for the structured response writer and helpers —
// ResponseWriter, JSON, Text, NoContent, Respond — covering all Phase 4 content
// negotiation requirements from docs/specifications/phase4.md.
package router_test

import (
	"encoding/json"
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperrors "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/router"
)

// ─── ResponseWriter ───────────────────────────────────────────────────────────

func TestResponseWriter_StatusTrackedAfterWriteHeader(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	rw.WriteHeader(http.StatusCreated)

	if rw.Status() != http.StatusCreated {
		t.Errorf("want Status()=201, got %d", rw.Status())
	}
}

func TestResponseWriter_StatusImplicit200OnWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	_, _ = rw.Write([]byte("body"))

	if rw.Status() != http.StatusOK {
		t.Errorf("Write without WriteHeader must record 200, got %d", rw.Status())
	}
}

func TestResponseWriter_BytesWrittenTracked(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	payload := []byte("hello")
	_, _ = rw.Write(payload)

	if rw.BytesWritten() != int64(len(payload)) {
		t.Errorf("want BytesWritten()=%d, got %d", len(payload), rw.BytesWritten())
	}
}

func TestResponseWriter_DoubleWriteHeader_SecondIsNoOp(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	rw.WriteHeader(http.StatusOK)
	rw.WriteHeader(http.StatusInternalServerError) // must be silently ignored

	if rec.Code != http.StatusOK {
		t.Errorf("second WriteHeader must not override the first; got %d", rec.Code)
	}
	if rw.Status() != http.StatusOK {
		t.Errorf("Status() must remain 200 after ignored second WriteHeader, got %d", rw.Status())
	}
}

func TestResponseWriter_ZeroStatus_BeforeAnyWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	if rw.Status() != 0 {
		t.Errorf("Status() before any write must be 0, got %d", rw.Status())
	}
}

func TestResponseWriter_DelegatesUnderlyingWriter(t *testing.T) {
	rec := httptest.NewRecorder()
	rw := router.WrapResponseWriter(rec, nil)

	rw.WriteHeader(http.StatusAccepted)
	_, _ = rw.Write([]byte("payload"))

	if rec.Code != http.StatusAccepted {
		t.Errorf("underlying recorder code want 202, got %d", rec.Code)
	}
	if rec.Body.String() != "payload" {
		t.Errorf("underlying recorder body want %q, got %q", "payload", rec.Body.String())
	}
}

// ─── JSON helper ──────────────────────────────────────────────────────────────

func TestJSON_SetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	err := router.JSON(rec, http.StatusOK, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("want application/json Content-Type, got %q", ct)
	}
}

func TestJSON_WritesEncodedBody(t *testing.T) {
	rec := httptest.NewRecorder()
	payload := map[string]string{"hello": "world"}
	_ = router.JSON(rec, http.StatusOK, payload)

	var got map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&got); err != nil {
		t.Fatalf("body is not valid JSON: %v", err)
	}
	if got["hello"] != "world" {
		t.Errorf("want hello=world, got %v", got)
	}
}

func TestJSON_WritesStatus(t *testing.T) {
	rec := httptest.NewRecorder()
	_ = router.JSON(rec, http.StatusCreated, struct{}{})

	if rec.Code != http.StatusCreated {
		t.Errorf("want status 201, got %d", rec.Code)
	}
}

// ─── Text helper ──────────────────────────────────────────────────────────────

func TestText_SetsContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	_ = router.Text(rec, http.StatusOK, "hello")

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("want text/plain Content-Type, got %q", ct)
	}
}

func TestText_WritesBody(t *testing.T) {
	rec := httptest.NewRecorder()
	_ = router.Text(rec, http.StatusOK, "hello world")

	if got := rec.Body.String(); got != "hello world" {
		t.Errorf("want %q, got %q", "hello world", got)
	}
}

// ─── NoContent helper ─────────────────────────────────────────────────────────

func TestNoContent_Status204_NoBody_NoContentType(t *testing.T) {
	rec := httptest.NewRecorder()
	router.NoContent(rec)

	if rec.Code != http.StatusNoContent {
		t.Errorf("want 204, got %d", rec.Code)
	}
	if rec.Body.Len() != 0 {
		t.Errorf("want no body, got %q", rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); ct != "" {
		t.Errorf("want no Content-Type for 204, got %q", ct)
	}
}

// ─── Respond (content negotiation) ───────────────────────────────────────────

func TestRespond_AcceptApplicationJSON_WritesJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/json")

	err := router.Respond(rec, req, http.StatusOK, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("want application/json, got %q", ct)
	}
}

func TestRespond_AcceptTextPlain_WritesText(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "text/plain")

	err := router.Respond(rec, req, http.StatusOK, "plain text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "text/plain") {
		t.Errorf("want text/plain, got %q", ct)
	}
	if rec.Body.Len() == 0 {
		t.Error("expected non-empty body")
	}
}

func TestRespond_AcceptStar_DefaultsToJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "*/*")

	err := router.Respond(rec, req, http.StatusOK, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Accept */* must default to JSON, got Content-Type %q", ct)
	}
}

func TestRespond_NoAcceptHeader_DefaultsToJSON(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	// No Accept header set.

	err := router.Respond(rec, req, http.StatusOK, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("absent Accept must default to JSON, got Content-Type %q", ct)
	}
}

func TestRespond_UnsupportedAccept_Returns406_WritesNothing(t *testing.T) {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "text/html")

	err := router.Respond(rec, req, http.StatusOK, map[string]string{"k": "v"})
	if err == nil {
		t.Fatal("expected 406 error for unsupported Accept, got nil")
	}

	var he *httperrors.HTTPError
	if !stderrors.As(err, &he) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if he.Code != http.StatusNotAcceptable {
		t.Errorf("want code 406, got %d", he.Code)
	}
	// Must not have written anything to the response.
	if rec.Body.Len() != 0 {
		t.Errorf("Respond must write no body when returning a 406 error, got: %q", rec.Body.String())
	}
}

func TestRespond_AcceptMultiple_PicksFirstSupported(t *testing.T) {
	// "application/xml, application/json" — xml is unsupported, json is supported.
	rec := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Accept", "application/xml, application/json")

	err := router.Respond(rec, req, http.StatusOK, map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("should pick json from multi-value Accept, got %q", ct)
	}
}
