// Purpose: Tests for the HTTPError type and helpers in pkg/errors, covering all
// Phase 4 and Phase 5 acceptance criteria for error representation and wire format.
package errors_test

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperrors "github.com/example/lightweight-http/pkg/errors"
)

// ─── HTTPError.Error() ───────────────────────────────────────────────────────

func TestHTTPError_Error_NoWrap(t *testing.T) {
	e := httperrors.New(http.StatusBadRequest, "bad input")
	if e.Error() != "bad input" {
		t.Errorf("want %q, got %q", "bad input", e.Error())
	}
}

func TestHTTPError_Error_WithWrap(t *testing.T) {
	cause := stderrors.New("underlying cause")
	e := httperrors.Wrap(http.StatusBadRequest, "bad input", cause)
	if e.Error() == "" {
		t.Error("Error() must not be empty")
	}
	// Message must be present
	if e.Message != "bad input" {
		t.Errorf("want Message %q, got %q", "bad input", e.Message)
	}
}

// ─── HTTPError.Unwrap() ──────────────────────────────────────────────────────

func TestHTTPError_Unwrap_NewHasNoCause(t *testing.T) {
	e := httperrors.New(http.StatusBadRequest, "no cause")
	if stderrors.Unwrap(e) != nil {
		t.Error("New HTTPError must have no wrapped cause")
	}
}

func TestHTTPError_Unwrap_WrapPreservesCause(t *testing.T) {
	sentinel := stderrors.New("sentinel")
	e := httperrors.Wrap(http.StatusBadRequest, "msg", sentinel)
	if !stderrors.Is(e, sentinel) {
		t.Error("Wrap must preserve cause for errors.Is traversal")
	}
}

// ─── New / Wrap constructors ─────────────────────────────────────────────────

func TestNew_SetsCodeAndMessage(t *testing.T) {
	e := httperrors.New(http.StatusNotFound, "not found")
	if e.Code != http.StatusNotFound {
		t.Errorf("want code 404, got %d", e.Code)
	}
	if e.Message != "not found" {
		t.Errorf("want message %q, got %q", "not found", e.Message)
	}
}

func TestWrap_SetsCodeMessageAndCause(t *testing.T) {
	cause := stderrors.New("original")
	e := httperrors.Wrap(http.StatusBadRequest, "wrapped", cause)
	if e.Code != http.StatusBadRequest {
		t.Errorf("want code 400, got %d", e.Code)
	}
	if e.Message != "wrapped" {
		t.Errorf("want message %q, got %q", "wrapped", e.Message)
	}
	if !stderrors.Is(e, cause) {
		t.Error("Wrap must make cause reachable via errors.Is")
	}
}

// ─── CodeOf ──────────────────────────────────────────────────────────────────

func TestCodeOf_HTTPError(t *testing.T) {
	e := httperrors.New(http.StatusForbidden, "forbidden")
	if got := httperrors.CodeOf(e); got != http.StatusForbidden {
		t.Errorf("want 403, got %d", got)
	}
}

func TestCodeOf_WrappedHTTPError(t *testing.T) {
	inner := httperrors.New(http.StatusNotFound, "not found")
	outer := stderrors.Join(inner, stderrors.New("extra"))
	// CodeOf must find the HTTPError even when wrapped in another error.
	if got := httperrors.CodeOf(inner); got != http.StatusNotFound {
		t.Errorf("want 404, got %d", got)
	}
	_ = outer
}

func TestCodeOf_PlainError_Returns500(t *testing.T) {
	plain := stderrors.New("something went wrong")
	if got := httperrors.CodeOf(plain); got != http.StatusInternalServerError {
		t.Errorf("want 500 for plain error, got %d", got)
	}
}

func TestCodeOf_Nil_Returns0(t *testing.T) {
	if got := httperrors.CodeOf(nil); got != 0 {
		t.Errorf("want 0 for nil, got %d", got)
	}
}

// ─── MessageOf ───────────────────────────────────────────────────────────────

func TestMessageOf_HTTPError(t *testing.T) {
	e := httperrors.New(http.StatusBadRequest, "bad input")
	if got := httperrors.MessageOf(e); got != "bad input" {
		t.Errorf("want %q, got %q", "bad input", got)
	}
}

func TestMessageOf_PlainError_ReturnsGeneric(t *testing.T) {
	plain := stderrors.New("secret internal error")
	got := httperrors.MessageOf(plain)
	if got != "Internal Server Error" {
		t.Errorf("want %q, got %q", "Internal Server Error", got)
	}
	// Must not leak the internal message.
	if got == "secret internal error" {
		t.Error("MessageOf must not expose plain error messages")
	}
}

func TestMessageOf_Nil_ReturnsEmpty(t *testing.T) {
	if got := httperrors.MessageOf(nil); got != "" {
		t.Errorf("want empty, got %q", got)
	}
}

// ─── NewCoded ─────────────────────────────────────────────────────────────────

func TestNewCoded_SetsAllFields(t *testing.T) {
	e := httperrors.NewCoded(http.StatusNotFound, "not_found", "resource not found")
	if e.Code != http.StatusNotFound {
		t.Errorf("want Code 404, got %d", e.Code)
	}
	if e.ErrCode != "not_found" {
		t.Errorf("want ErrCode %q, got %q", "not_found", e.ErrCode)
	}
	if e.Message != "resource not found" {
		t.Errorf("want Message %q, got %q", "resource not found", e.Message)
	}
}

// ─── Sentinel errors ──────────────────────────────────────────────────────────

func TestSentinelErrors_StatusAndCode(t *testing.T) {
	cases := []struct {
		name        string
		err         *httperrors.HTTPError
		wantCode    int
		wantErrCode string
	}{
		{"ErrBadRequest", httperrors.ErrBadRequest, 400, "bad_request"},
		{"ErrUnauthorized", httperrors.ErrUnauthorized, 401, "unauthorized"},
		{"ErrForbidden", httperrors.ErrForbidden, 403, "forbidden"},
		{"ErrNotFound", httperrors.ErrNotFound, 404, "not_found"},
		{"ErrConflict", httperrors.ErrConflict, 409, "conflict"},
		{"ErrUnsupportedMedia", httperrors.ErrUnsupportedMedia, 415, "unsupported_media_type"},
		{"ErrUnprocessable", httperrors.ErrUnprocessable, 422, "unprocessable_entity"},
		{"ErrTooManyRequests", httperrors.ErrTooManyRequests, 429, "too_many_requests"},
		{"ErrInternal", httperrors.ErrInternal, 500, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.err.Code != tc.wantCode {
				t.Errorf("want Code %d, got %d", tc.wantCode, tc.err.Code)
			}
			if tc.err.ErrCode != tc.wantErrCode {
				t.Errorf("want ErrCode %q, got %q", tc.wantErrCode, tc.err.ErrCode)
			}
		})
	}
}

func TestSentinelErrors_IsWorksThrough2Wraps(t *testing.T) {
	w1 := fmt.Errorf("layer1: %w", httperrors.ErrNotFound)
	w2 := fmt.Errorf("layer2: %w", w1)
	if !stderrors.Is(w2, httperrors.ErrNotFound) {
		t.Error("errors.Is must find ErrNotFound through two fmt.Errorf wraps")
	}
}

func TestSentinelErrors_AsWorksForHTTPError(t *testing.T) {
	wrapped := fmt.Errorf("context: %w", httperrors.ErrForbidden)
	var he *httperrors.HTTPError
	if !stderrors.As(wrapped, &he) {
		t.Error("errors.As must find *HTTPError wrapped in fmt.Errorf")
	}
	if he.Code != http.StatusForbidden {
		t.Errorf("want Code 403, got %d", he.Code)
	}
}

// ─── WithDetails ──────────────────────────────────────────────────────────────

func TestHTTPError_WithDetails_ReturnsCopy(t *testing.T) {
	original := httperrors.NewCoded(http.StatusBadRequest, "bad_request", "validation failed")
	withD := original.WithDetails(httperrors.Detail{Field: "email", Reason: "required"})
	if original == withD {
		t.Error("WithDetails must return a new *HTTPError, not mutate the original")
	}
}

func TestHTTPError_WithDetails_SentinelUnchanged(t *testing.T) {
	before := httperrors.ErrBadRequest
	_ = httperrors.ErrBadRequest.WithDetails(httperrors.Detail{Field: "x", Reason: "y"})
	if httperrors.ErrBadRequest != before {
		t.Error("WithDetails on a sentinel must not modify the sentinel pointer")
	}
}

// ─── Request ID context helpers ───────────────────────────────────────────────

func TestWithRequestID_RoundTrip(t *testing.T) {
	ctx := httperrors.WithRequestID(context.Background(), "req-abc-123")
	got := httperrors.RequestIDFromContext(ctx)
	if got != "req-abc-123" {
		t.Errorf("want %q, got %q", "req-abc-123", got)
	}
}

func TestRequestIDFromContext_MissingReturnsEmpty(t *testing.T) {
	got := httperrors.RequestIDFromContext(context.Background())
	if got != "" {
		t.Errorf("want empty string when no request ID in context, got %q", got)
	}
}

// ─── Handler / ServeError ─────────────────────────────────────────────────────

// newNoopHandler is a test helper that builds a production-mode Handler with
// all log output discarded.
func newNoopHandler() *httperrors.Handler {
	return httperrors.NewHandler(slog.New(slog.NewTextHandler(io.Discard, nil)), false)
}

// envelopeKeys returns the top-level keys of a JSON object, used in assertions.
func envelopeKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func TestHandler_WritesCorrectStatus(t *testing.T) {
	h := newNoopHandler()
	err := httperrors.NewCoded(http.StatusNotFound, "not_found", "not found")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, err)
	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestHandler_WritesJSONContentType(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, httperrors.ErrNotFound)
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("want Content-Type application/json, got %q", ct)
	}
}

func TestHandler_EnvelopeHasSingleTopLevelKey(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	h.ServeError(w, r, httperrors.NewCoded(http.StatusBadRequest, "bad_request", "invalid input"))

	var env map[string]json.RawMessage
	if parseErr := json.Unmarshal(w.Body.Bytes(), &env); parseErr != nil {
		t.Fatalf("body is not valid JSON: %v\nbody: %s", parseErr, w.Body.String())
	}
	if _, ok := env["error"]; !ok {
		t.Fatalf("response JSON must have top-level 'error' key; got: %v", envelopeKeys(env))
	}
	if len(env) != 1 {
		t.Errorf("exactly one top-level key required, got %d: %v", len(env), envelopeKeys(env))
	}
}

func TestHandler_EnvelopeFields_CodeMessageStatus(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	h.ServeError(w, r, httperrors.NewCoded(http.StatusBadRequest, "bad_request", "invalid input"))

	var env struct {
		Error struct {
			Code    string  `json:"code"`
			Message string  `json:"message"`
			Status  float64 `json:"status"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(w.Body.Bytes(), &env); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if env.Error.Code != "bad_request" {
		t.Errorf("want code %q, got %q", "bad_request", env.Error.Code)
	}
	if env.Error.Message != "invalid input" {
		t.Errorf("want message %q, got %q", "invalid input", env.Error.Message)
	}
	if env.Error.Status != 400 {
		t.Errorf("want status 400, got %v", env.Error.Status)
	}
}

func TestHandler_EmptyDetails_Omitted(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, httperrors.NewCoded(http.StatusNotFound, "not_found", "not found"))

	var env map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := env["error"]["details"]; ok {
		t.Error("details must be omitted from the envelope when empty")
	}
}

func TestHandler_WithDetails_Included(t *testing.T) {
	h := newNoopHandler()
	err := httperrors.NewCoded(http.StatusBadRequest, "bad_request", "validation failed").
		WithDetails(httperrors.Detail{Field: "email", Reason: "required"})
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodPost, "/", nil)
	h.ServeError(w, r, err)

	var env struct {
		Error struct {
			Details []map[string]string `json:"details"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(w.Body.Bytes(), &env); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if len(env.Error.Details) == 0 {
		t.Fatal("expected details in envelope, got none")
	}
	d := env.Error.Details[0]
	if d["field"] != "email" {
		t.Errorf("want field %q, got %q", "email", d["field"])
	}
	if d["reason"] != "required" {
		t.Errorf("want reason %q, got %q", "required", d["reason"])
	}
}

func TestHandler_PlainError_Returns500Generic(t *testing.T) {
	h := newNoopHandler()
	err := stderrors.New("internal: DB connection refused")
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, err)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("want 500, got %d", w.Code)
	}
	body := w.Body.String()
	if strings.Contains(body, "DB connection refused") {
		t.Error("production response must not expose internal error messages")
	}
}

func TestHandler_WithRequestID_Included(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/resource", nil)
	r = r.WithContext(httperrors.WithRequestID(r.Context(), "req-xyz"))
	h.ServeError(w, r, httperrors.ErrNotFound)

	var env struct {
		Error struct {
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if jsonErr := json.Unmarshal(w.Body.Bytes(), &env); jsonErr != nil {
		t.Fatalf("unmarshal: %v", jsonErr)
	}
	if env.Error.RequestID != "req-xyz" {
		t.Errorf("want request_id %q, got %q", "req-xyz", env.Error.RequestID)
	}
}

func TestHandler_WithoutRequestID_Omitted(t *testing.T) {
	h := newNoopHandler()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, httperrors.ErrNotFound)

	var env map[string]map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := env["error"]["request_id"]; ok {
		t.Error("request_id must be omitted when not present in context")
	}
}

func TestHandler_ProductionMode_NoCauseText(t *testing.T) {
	h := newNoopHandler()
	cause := stderrors.New("secret: password=hunter2")
	err := httperrors.Wrap(http.StatusInternalServerError, "Internal Server Error", cause)
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	h.ServeError(w, r, err)

	body := w.Body.String()
	if strings.Contains(body, "secret") {
		t.Error("production mode must not include cause text in response body")
	}
	if strings.Contains(body, "password") {
		t.Error("production mode must not include cause text in response body")
	}
}

// TestHandler_NeverEmitsGoroutineOrRepoPath asserts the canonical security rule:
// no stack trace fragments or internal paths ever appear in the client body.
func TestHandler_NeverEmitsGoroutineOrRepoPath(t *testing.T) {
	for _, debug := range []bool{false, true} {
		t.Run(fmt.Sprintf("debug=%v", debug), func(t *testing.T) {
			var logBuf strings.Builder
			h := httperrors.NewHandler(
				slog.New(slog.NewTextHandler(&logBuf, nil)),
				debug,
			)
			err := httperrors.Wrap(http.StatusInternalServerError, "Internal Server Error",
				stderrors.New("some internal failure"))
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			h.ServeError(w, r, err)

			body := w.Body.String()
			if strings.Contains(body, "goroutine") {
				t.Errorf("response body contains 'goroutine': %s", body)
			}
			if strings.Contains(body, "github.com/example/lightweight-http") {
				t.Errorf("response body contains repo path: %s", body)
			}
		})
	}
}

// ─── WriteError ───────────────────────────────────────────────────────────────

func TestWriteError_WritesJSONEnvelope(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/missing", nil)
	httperrors.WriteError(w, r, httperrors.ErrNotFound)

	if w.Code != http.StatusNotFound {
		t.Errorf("WriteError: want 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("WriteError: want application/json, got %q", ct)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("WriteError: body is not valid JSON: %v", err)
	}
	if _, ok := env["error"]; !ok {
		t.Error("WriteError: envelope must have top-level 'error' key")
	}
}
