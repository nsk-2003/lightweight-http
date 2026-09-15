// Purpose: Tests for Handle: envelope shape, production-mode safety, debug-mode logging,
// request-ID inclusion, unmapped error fallback, and goroutine/path leak prevention.

package errors

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// testEnvelope mirrors the JSON envelope for test assertions.
type testEnvelope struct {
	Error testErrorBody `json:"error"`
}

type testErrorBody struct {
	Code      string       `json:"code"`
	Message   string       `json:"message"`
	Status    int          `json:"status"`
	RequestID string       `json:"request_id"`
	Details   []testDetail `json:"details"`
}

type testDetail struct {
	Field  string `json:"field"`
	Reason string `json:"reason"`
}

func discardLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func decodeEnvelope(t *testing.T, body string) testEnvelope {
	t.Helper()
	var env testEnvelope
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("failed to decode response body %q: %v", body, err)
	}
	return env
}

// TestHandleNotFound verifies that ErrNotFound produces a 404 JSON envelope.
func TestHandleNotFound(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/items/1", nil)
	w := httptest.NewRecorder()

	Handle(w, r, ErrNotFound, discardLogger(), false)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
	env := decodeEnvelope(t, w.Body.String())
	if env.Error.Code != "not_found" {
		t.Errorf("code = %q, want %q", env.Error.Code, "not_found")
	}
	if env.Error.Status != 404 {
		t.Errorf("envelope status = %d, want 404", env.Error.Status)
	}
	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

// TestHandleEnvelopeShape verifies the top-level JSON has exactly one key, "error".
func TestHandleEnvelopeShape(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, ErrBadRequest, discardLogger(), false)

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &raw); err != nil {
		t.Fatalf("response is not valid JSON: %v", err)
	}
	if len(raw) != 1 {
		keys := make([]string, 0, len(raw))
		for k := range raw {
			keys = append(keys, k)
		}
		t.Errorf("top-level keys = %v, want exactly [error]", keys)
	}
	if _, ok := raw["error"]; !ok {
		t.Error("top-level key 'error' is missing")
	}
}

// TestHandleProductionNoCause verifies that the client body never contains the cause message.
func TestHandleProductionNoCause(t *testing.T) {
	cause := fmt.Errorf("SELECT * FROM users: connection refused")
	err := Internal("internal server error", cause)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, err, discardLogger(), false)

	body := w.Body.String()
	if strings.Contains(body, "SELECT") {
		t.Error("body must not contain cause text in production mode")
	}
	if strings.Contains(body, "connection refused") {
		t.Error("body must not contain cause details in production mode")
	}
}

// TestHandleDebugBodyUnchanged verifies that in debug mode the body still excludes cause
// and stack trace; they appear only in the log.
func TestHandleDebugBodyUnchanged(t *testing.T) {
	cause := fmt.Errorf("db timeout")
	err := Internal("server error", cause)

	var logBuf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&logBuf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, err, logger, true)

	body := w.Body.String()
	if strings.Contains(body, "db timeout") {
		t.Error("body must not contain cause text even in debug mode")
	}
	if strings.Contains(body, "goroutine") {
		t.Error("body must not contain goroutine text even in debug mode")
	}
	if !strings.Contains(logBuf.String(), "db timeout") {
		t.Error("cause should appear in the log in debug mode")
	}
}

// TestHandleRequestID verifies that the request ID from context appears in the envelope.
func TestHandleRequestID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r = r.WithContext(WithRequestID(r.Context(), "req-abc123"))
	w := httptest.NewRecorder()

	Handle(w, r, ErrNotFound, discardLogger(), false)

	env := decodeEnvelope(t, w.Body.String())
	if env.Error.RequestID != "req-abc123" {
		t.Errorf("request_id = %q, want %q", env.Error.RequestID, "req-abc123")
	}
}

// TestHandleNoRequestID verifies that request_id is omitted from JSON when not in context.
func TestHandleNoRequestID(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, ErrNotFound, discardLogger(), false)

	body := w.Body.String()
	if strings.Contains(body, "request_id") {
		t.Error("request_id must be omitted from JSON when absent")
	}
}

// TestHandleUnmappedError verifies that a plain (non-HTTPError) error becomes a 500
// with a generic client message; the original text must not leak.
func TestHandleUnmappedError(t *testing.T) {
	plainErr := fmt.Errorf("some unexpected internal error")

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, plainErr, discardLogger(), false)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	env := decodeEnvelope(t, w.Body.String())
	if env.Error.Status != 500 {
		t.Errorf("envelope status = %d, want 500", env.Error.Status)
	}
	if strings.Contains(env.Error.Message, "some unexpected internal error") {
		t.Error("body must not contain the original plain error message for unmapped errors")
	}
}

// TestHandleWithDetails verifies that Details are included in the envelope when present.
func TestHandleWithDetails(t *testing.T) {
	detail := Detail{Field: "email", Reason: "required"}
	err := UnprocessableEntity("validation failed", detail)

	r := httptest.NewRequest(http.MethodPost, "/users", nil)
	w := httptest.NewRecorder()
	Handle(w, r, err, discardLogger(), false)

	if w.Code != http.StatusUnprocessableEntity {
		t.Errorf("status = %d, want 422", w.Code)
	}
	env := decodeEnvelope(t, w.Body.String())
	if len(env.Error.Details) != 1 {
		t.Fatalf("details count = %d, want 1", len(env.Error.Details))
	}
	if env.Error.Details[0].Field != "email" || env.Error.Details[0].Reason != "required" {
		t.Errorf("detail = %+v, want {email required}", env.Error.Details[0])
	}
}

// TestHandleDetailsOmittedWhenEmpty verifies that "details" is absent from JSON when empty.
func TestHandleDetailsOmittedWhenEmpty(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, ErrNotFound, discardLogger(), false)

	if strings.Contains(w.Body.String(), "details") {
		t.Error("'details' key must be omitted from JSON when there are no details")
	}
}

// TestHandleBodyNeverContainsStackOrPath verifies the client body never contains "goroutine"
// or the repository module path, even when the cause carries such text (ADR-008).
func TestHandleBodyNeverContainsStackOrPath(t *testing.T) {
	cause := fmt.Errorf("goroutine 1 [running]:\ngithub.com/example/lightweight-http/pkg/db.Query()")
	err := Internal("internal server error", cause)

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, err, discardLogger(), false)

	body := w.Body.String()
	if strings.Contains(body, "goroutine") {
		t.Errorf("body must not contain %q; got: %s", "goroutine", body)
	}
	if strings.Contains(body, "github.com/example") {
		t.Errorf("body must not contain repository path; got: %s", body)
	}
}

// TestHandleBodyNeverContainsStackDebugMode verifies body safety even in debug mode.
func TestHandleBodyNeverContainsStackDebugMode(t *testing.T) {
	err := Internal("server error", fmt.Errorf("db error"))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, err, discardLogger(), true) // debug=true

	body := w.Body.String()
	if strings.Contains(body, "goroutine") {
		t.Errorf("body must not contain 'goroutine' even in debug mode; got: %s", body)
	}
}

// TestHandleRecoveryIndistinguishable verifies that a panic-derived 500 envelope is
// identical in shape to any other 500 envelope, as seen by the client.
func TestHandleRecoveryIndistinguishable(t *testing.T) {
	panicErr := Internal("an internal error occurred", fmt.Errorf("panic: something went wrong"))
	normalErr := Internal("an internal error occurred", fmt.Errorf("db error"))

	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w1 := httptest.NewRecorder()
	Handle(w1, r, panicErr, discardLogger(), false)

	w2 := httptest.NewRecorder()
	Handle(w2, r, normalErr, discardLogger(), false)

	env1 := decodeEnvelope(t, w1.Body.String())
	env2 := decodeEnvelope(t, w2.Body.String())

	if env1.Error.Code != env2.Error.Code {
		t.Errorf("panic code %q != normal code %q", env1.Error.Code, env2.Error.Code)
	}
	if env1.Error.Status != env2.Error.Status {
		t.Errorf("panic envelope status %d != normal envelope status %d", env1.Error.Status, env2.Error.Status)
	}
	if w1.Code != w2.Code {
		t.Errorf("panic HTTP status %d != normal HTTP status %d", w1.Code, w2.Code)
	}
}

// TestHandleNilLogger verifies that Handle works when logger is nil (falls back to slog.Default).
func TestHandleNilLogger(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	Handle(w, r, ErrNotFound, nil, false)

	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}
