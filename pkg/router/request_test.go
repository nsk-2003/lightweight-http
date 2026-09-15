// Purpose: Tests for typed request parsing covering JSON body, form data, and query
// parameters, verifying every row of the Phase 4 behavioral requirements table.

package router

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	httperr "github.com/example/lightweight-http/pkg/errors"
)

// person is a small struct used by JSON parsing tests.
type person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// statusOf returns the HTTPError status code from err, or 0 if err is not an *HTTPError.
func statusOf(err error) int {
	if e, ok := err.(*httperr.HTTPError); ok {
		return e.Status
	}
	return 0
}

// ---- ParseJSON tests --------------------------------------------------------

// TestParseJSONSuccess verifies a valid JSON body is decoded into the target struct.
func TestParseJSONSuccess(t *testing.T) {
	body, _ := os.ReadFile("../../test/testdata/valid.json")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var p person
	if err := ParseJSON(r, &p); err != nil {
		t.Fatalf("ParseJSON: %v", err)
	}
	if p.Name != "Alice" || p.Age != 30 {
		t.Errorf("parsed = %+v, want {Name:Alice Age:30}", p)
	}
}

// TestParseJSONMalformed verifies a malformed JSON body returns a 400 without echoing body.
func TestParseJSONMalformed(t *testing.T) {
	body, _ := os.ReadFile("../../test/testdata/malformed.json")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var p person
	err := ParseJSON(r, &p)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if statusOf(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
	if strings.Contains(err.Error(), string(body)) {
		t.Errorf("error message must not echo the raw body")
	}
}

// TestParseJSONUnknownFieldStrict verifies that an unknown field returns 400 in strict mode.
func TestParseJSONUnknownFieldStrict(t *testing.T) {
	body, _ := os.ReadFile("../../test/testdata/unknown_field.json")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var p person
	err := ParseJSON(r, &p) // strict is the default
	if err == nil {
		t.Fatal("expected error for unknown field, got nil")
	}
	if statusOf(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Errorf("error message %q should mention the unknown field", err.Error())
	}
}

// TestParseJSONUnknownFieldPermissive verifies that DisallowUnknownFields=false allows extra fields.
func TestParseJSONUnknownFieldPermissive(t *testing.T) {
	body, _ := os.ReadFile("../../test/testdata/unknown_field.json")
	r := httptest.NewRequest(http.MethodPost, "/", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")

	var p person
	err := ParseJSON(r, &p, ParseOptions{DisallowUnknownFields: false})
	if err != nil {
		t.Fatalf("expected no error in permissive mode, got %v", err)
	}
	if p.Name != "Alice" {
		t.Errorf("Name = %q, want %q", p.Name, "Alice")
	}
}

// TestParseJSONBodyTooLarge verifies that a body exceeding the limit returns 413.
func TestParseJSONBodyTooLarge(t *testing.T) {
	smallLimit := int64(10) // 10 bytes — far less than any real JSON body in the test
	bigBody := strings.Repeat("x", 100)
	// Wrap in valid JSON to ensure the limit is the issue, not the format.
	payload := `{"name":"` + bigBody + `"}`

	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(payload))
	r.Header.Set("Content-Type", "application/json")

	var p person
	err := ParseJSON(r, &p, ParseOptions{MaxBodyBytes: smallLimit, DisallowUnknownFields: false})
	if err == nil {
		t.Fatal("expected error for oversized body, got nil")
	}
	if statusOf(err) != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", statusOf(err))
	}
}

// TestParseJSONWrongContentType verifies that a non-JSON Content-Type returns 415.
func TestParseJSONWrongContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("<root/>"))
	r.Header.Set("Content-Type", "application/xml")

	var p person
	err := ParseJSON(r, &p)
	if err == nil {
		t.Fatal("expected error for wrong Content-Type, got nil")
	}
	if statusOf(err) != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", statusOf(err))
	}
}

// TestParseJSONContentTypeWithCharset verifies that "application/json; charset=utf-8" is accepted.
func TestParseJSONContentTypeWithCharset(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"Bob","age":25}`))
	r.Header.Set("Content-Type", "application/json; charset=utf-8")

	var p person
	if err := ParseJSON(r, &p); err != nil {
		t.Fatalf("ParseJSON with charset: %v", err)
	}
}

// ---- ParseForm tests --------------------------------------------------------

// TestParseFormURLEncoded verifies form values are parsed from application/x-www-form-urlencoded.
func TestParseFormURLEncoded(t *testing.T) {
	body := strings.NewReader("name=Alice&age=30")
	r := httptest.NewRequest(http.MethodPost, "/", body)
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := ParseForm(r); err != nil {
		t.Fatalf("ParseForm: %v", err)
	}
	if got := r.FormValue("name"); got != "Alice" {
		t.Errorf("name = %q, want %q", got, "Alice")
	}
}

// TestParseFormWrongContentType verifies that a non-form Content-Type returns 415.
func TestParseFormWrongContentType(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"x":1}`))
	r.Header.Set("Content-Type", "application/json")

	err := ParseForm(r)
	if err == nil {
		t.Fatal("expected error for wrong Content-Type, got nil")
	}
	if statusOf(err) != http.StatusUnsupportedMediaType {
		t.Errorf("status = %d, want 415", statusOf(err))
	}
}

// ---- RequireQuery tests -----------------------------------------------------

// TestRequireQueryPresent verifies that a present query parameter is returned.
func TestRequireQueryPresent(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?q=hello", nil)
	v, err := RequireQuery(r, "q")
	if err != nil {
		t.Fatalf("RequireQuery: %v", err)
	}
	if v != "hello" {
		t.Errorf("v = %q, want %q", v, "hello")
	}
}

// TestRequireQueryMissing verifies that an absent parameter returns a 400.
func TestRequireQueryMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := RequireQuery(r, "q")
	if err == nil {
		t.Fatal("expected error for missing param, got nil")
	}
	if statusOf(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
	if !strings.Contains(err.Error(), "q") {
		t.Errorf("error %q should name the missing parameter", err.Error())
	}
}

// TestRequireQueryEmptyValue verifies that a present-but-empty value is returned without error.
func TestRequireQueryEmptyValue(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?key=", nil)
	v, err := RequireQuery(r, "key")
	if err != nil {
		t.Fatalf("RequireQuery for empty value: %v", err)
	}
	if v != "" {
		t.Errorf("v = %q, want empty string", v)
	}
}

// ---- QueryInt tests ---------------------------------------------------------

// TestQueryIntSuccess verifies a valid integer query parameter is returned.
func TestQueryIntSuccess(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?count=42", nil)
	n, err := QueryInt(r, "count")
	if err != nil {
		t.Fatalf("QueryInt: %v", err)
	}
	if n != 42 {
		t.Errorf("n = %d, want 42", n)
	}
}

// TestQueryIntNonNumeric verifies that a non-integer value returns a 400 naming the parameter.
func TestQueryIntNonNumeric(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/?count=abc", nil)
	_, err := QueryInt(r, "count")
	if err == nil {
		t.Fatal("expected error for non-numeric value, got nil")
	}
	if statusOf(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
	if !strings.Contains(err.Error(), "count") {
		t.Errorf("error %q should name the parameter", err.Error())
	}
}

// TestQueryIntMissing verifies that a missing integer parameter returns a 400.
func TestQueryIntMissing(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	_, err := QueryInt(r, "page")
	if err == nil {
		t.Fatal("expected error for missing param, got nil")
	}
	if statusOf(err) != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", statusOf(err))
	}
}
