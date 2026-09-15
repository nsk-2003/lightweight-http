// Purpose: Tests for HTTPError creation, status codes, and the error string format.

package errors

import (
	"net/http"
	"strings"
	"testing"
)

// TestNew verifies that New sets Code and Message correctly.
func TestNew(t *testing.T) {
	e := New(http.StatusTeapot, "I'm a teapot")
	if e.Code != http.StatusTeapot {
		t.Errorf("Code = %d, want %d", e.Code, http.StatusTeapot)
	}
	if e.Message != "I'm a teapot" {
		t.Errorf("Message = %q, want %q", e.Message, "I'm a teapot")
	}
}

// TestError verifies that Error() contains the status code and message.
func TestError(t *testing.T) {
	e := New(400, "bad request")
	s := e.Error()
	if s == "" {
		t.Fatal("Error() must not be empty")
	}
	if !strings.Contains(s, "400") {
		t.Errorf("Error() = %q, want it to contain the status code", s)
	}
}

// TestStatusCode verifies that StatusCode() returns Code.
func TestStatusCode(t *testing.T) {
	e := New(422, "unprocessable")
	if e.StatusCode() != 422 {
		t.Errorf("StatusCode() = %d, want 422", e.StatusCode())
	}
}

// TestConstructors verifies that each convenience constructor produces the correct status.
func TestConstructors(t *testing.T) {
	cases := []struct {
		name string
		e    *HTTPError
		want int
	}{
		{"BadRequest", BadRequest("msg"), http.StatusBadRequest},
		{"RequestEntityTooLarge", RequestEntityTooLarge("msg"), http.StatusRequestEntityTooLarge},
		{"UnsupportedMediaType", UnsupportedMediaType("msg"), http.StatusUnsupportedMediaType},
		{"NotAcceptable", NotAcceptable("msg"), http.StatusNotAcceptable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.e.Code != tc.want {
				t.Errorf("Code = %d, want %d", tc.e.Code, tc.want)
			}
			if tc.e.Message != "msg" {
				t.Errorf("Message = %q, want %q", tc.e.Message, "msg")
			}
		})
	}
}

// TestHTTPErrorImplementsError confirms the type satisfies the error interface at compile time.
var _ error = (*HTTPError)(nil)
