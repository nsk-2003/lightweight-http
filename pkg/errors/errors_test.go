// Purpose: Tests for HTTPError construction, sentinel errors, Unwrap, errors.Is/As
// compatibility, and the request-ID context helpers.

package errors

import (
	"context"
	stderrors "errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// compile-time interface check
var _ error = (*HTTPError)(nil)

// TestNew verifies that New sets Status, Code, and Message correctly.
func TestNew(t *testing.T) {
	e := New(http.StatusTeapot, "teapot", "I'm a teapot")
	if e.Status != http.StatusTeapot {
		t.Errorf("Status = %d, want %d", e.Status, http.StatusTeapot)
	}
	if e.Code != "teapot" {
		t.Errorf("Code = %q, want %q", e.Code, "teapot")
	}
	if e.Message != "I'm a teapot" {
		t.Errorf("Message = %q, want %q", e.Message, "I'm a teapot")
	}
}

// TestError verifies that Error() contains the status code and message.
func TestError(t *testing.T) {
	e := New(400, "bad_request", "bad request")
	s := e.Error()
	if s == "" {
		t.Fatal("Error() must not be empty")
	}
	if !strings.Contains(s, "400") {
		t.Errorf("Error() = %q, want it to contain the status code", s)
	}
}

// TestErrorWithCause verifies that Error() includes the cause when present.
func TestErrorWithCause(t *testing.T) {
	cause := fmt.Errorf("underlying db error")
	e := Internal("server error", cause)
	s := e.Error()
	if !strings.Contains(s, "server error") {
		t.Errorf("Error() = %q, should contain message", s)
	}
	if !strings.Contains(s, "underlying db error") {
		t.Errorf("Error() = %q, should contain cause", s)
	}
}

// TestStatusCode verifies that StatusCode() returns the HTTP status.
func TestStatusCode(t *testing.T) {
	e := New(422, "unprocessable_entity", "unprocessable")
	if e.StatusCode() != 422 {
		t.Errorf("StatusCode() = %d, want 422", e.StatusCode())
	}
}

// TestConstructors verifies that each convenience constructor produces the correct status and code.
func TestConstructors(t *testing.T) {
	cases := []struct {
		name       string
		e          *HTTPError
		wantStatus int
		wantCode   string
	}{
		{"BadRequest", BadRequest("msg"), http.StatusBadRequest, "bad_request"},
		{"Unauthorized", Unauthorized("msg"), http.StatusUnauthorized, "unauthorized"},
		{"Forbidden", Forbidden("msg"), http.StatusForbidden, "forbidden"},
		{"NotFound", NotFound("msg"), http.StatusNotFound, "not_found"},
		{"Conflict", Conflict("msg"), http.StatusConflict, "conflict"},
		{"UnsupportedMediaType", UnsupportedMediaType("msg"), http.StatusUnsupportedMediaType, "unsupported_media_type"},
		{"TooManyRequests", TooManyRequests("msg"), http.StatusTooManyRequests, "too_many_requests"},
		{"RequestEntityTooLarge", RequestEntityTooLarge("msg"), http.StatusRequestEntityTooLarge, "request_too_large"},
		{"NotAcceptable", NotAcceptable("msg"), http.StatusNotAcceptable, "not_acceptable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.e.Status != tc.wantStatus {
				t.Errorf("Status = %d, want %d", tc.e.Status, tc.wantStatus)
			}
			if tc.e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", tc.e.Code, tc.wantCode)
			}
			if tc.e.Message != "msg" {
				t.Errorf("Message = %q, want %q", tc.e.Message, "msg")
			}
		})
	}
}

// TestInternalCause verifies that Internal wraps the cause and sets status 500.
func TestInternalCause(t *testing.T) {
	cause := fmt.Errorf("db timeout")
	e := Internal("server error", cause)
	if e.Status != http.StatusInternalServerError {
		t.Errorf("Status = %d, want 500", e.Status)
	}
	if e.Cause != cause {
		t.Error("Cause should be the passed error")
	}
}

// TestUnprocessableEntityDetails verifies that UnprocessableEntity populates Details.
func TestUnprocessableEntityDetails(t *testing.T) {
	d := Detail{Field: "email", Reason: "required"}
	e := UnprocessableEntity("validation failed", d)
	if e.Status != http.StatusUnprocessableEntity {
		t.Errorf("Status = %d, want 422", e.Status)
	}
	if len(e.Details) != 1 || e.Details[0] != d {
		t.Errorf("Details = %v, want %v", e.Details, []Detail{d})
	}
}

// TestUnwrap verifies that Unwrap returns the Cause for chain traversal.
func TestUnwrap(t *testing.T) {
	cause := fmt.Errorf("root cause")
	e := Internal("wrapped", cause)
	if e.Unwrap() != cause {
		t.Error("Unwrap() must return Cause")
	}

	e2 := NotFound("not found")
	if e2.Unwrap() != nil {
		t.Error("Unwrap() must return nil when Cause is nil")
	}
}

// TestSentinels verifies that each sentinel has the expected status and code.
func TestSentinels(t *testing.T) {
	cases := []struct {
		name       string
		e          *HTTPError
		wantStatus int
		wantCode   string
	}{
		{"ErrBadRequest", ErrBadRequest, 400, "bad_request"},
		{"ErrUnauthorized", ErrUnauthorized, 401, "unauthorized"},
		{"ErrForbidden", ErrForbidden, 403, "forbidden"},
		{"ErrNotFound", ErrNotFound, 404, "not_found"},
		{"ErrConflict", ErrConflict, 409, "conflict"},
		{"ErrUnsupportedMediaType", ErrUnsupportedMediaType, 415, "unsupported_media_type"},
		{"ErrUnprocessableEntity", ErrUnprocessableEntity, 422, "unprocessable_entity"},
		{"ErrTooManyRequests", ErrTooManyRequests, 429, "too_many_requests"},
		{"ErrInternal", ErrInternal, 500, "internal_error"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.e.Status != tc.wantStatus {
				t.Errorf("Status = %d, want %d", tc.e.Status, tc.wantStatus)
			}
			if tc.e.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", tc.e.Code, tc.wantCode)
			}
		})
	}
}

// TestErrorsIsThroughWraps verifies that errors.Is finds a sentinel through two Cause wraps.
func TestErrorsIsThroughWraps(t *testing.T) {
	inner := Internal("layer1", ErrNotFound) // Cause = ErrNotFound
	outer := Internal("layer2", inner)       // Cause = inner → ErrNotFound

	if !stderrors.Is(outer, ErrNotFound) {
		t.Error("errors.Is must find ErrNotFound through two wraps")
	}
}

// TestErrorsAsHTTPError verifies that errors.As finds *HTTPError through a fmt.Errorf wrap.
func TestErrorsAsHTTPError(t *testing.T) {
	he := NotFound("item not found")
	wrapped := fmt.Errorf("lookup: %w", he)

	var got *HTTPError
	if !stderrors.As(wrapped, &got) {
		t.Fatal("errors.As must find *HTTPError through fmt.Errorf wrap")
	}
	if got.Status != http.StatusNotFound {
		t.Errorf("Status = %d, want 404", got.Status)
	}
}

// TestRequestIDContextRoundTrip verifies WithRequestID and RequestIDFromContext.
func TestRequestIDContextRoundTrip(t *testing.T) {
	ctx := context.Background()
	id := "req-123"
	ctx = WithRequestID(ctx, id)

	got := RequestIDFromContext(ctx)
	if got != id {
		t.Errorf("RequestIDFromContext = %q, want %q", got, id)
	}
}

// TestRequestIDContextEmpty verifies that RequestIDFromContext returns "" when no ID is set.
func TestRequestIDContextEmpty(t *testing.T) {
	ctx := context.Background()
	if got := RequestIDFromContext(ctx); got != "" {
		t.Errorf("RequestIDFromContext = %q, want empty string", got)
	}
}
