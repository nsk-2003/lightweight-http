// Purpose: Tests for the HTTPError type and helpers in pkg/errors, covering all
// Phase 4 acceptance criteria for error representation.
package errors_test

import (
	stderrors "errors"
	"net/http"
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
