// Purpose: Black-box tests for request parsing helpers — BindJSON, BindForm,
// QueryParam, QueryParamInt, PathParam — covering all Phase 4 behavioral
// requirements and the behavior table from docs/specifications/phase4.md.
package router_test

import (
	stderrors "errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	httperrors "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/router"
)

// ─── helpers ──────────────────────────────────────────────────────────────────

// codeOf extracts the HTTP status code from an HTTPError, or 0 if err is nil,
// or -1 if err is not an HTTPError.
func codeOf(t *testing.T, err error) int {
	t.Helper()
	return httperrors.CodeOf(err)
}

// assertHTTPError fails if err is nil or not an HTTPError with the given code.
func assertHTTPError(t *testing.T, err error, wantCode int) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected an HTTPError with code %d, got nil", wantCode)
	}
	var he *httperrors.HTTPError
	if !stderrors.As(err, &he) {
		t.Fatalf("expected *HTTPError, got %T: %v", err, err)
	}
	if he.Code != wantCode {
		t.Errorf("want HTTP status %d, got %d (message: %s)", wantCode, he.Code, he.Message)
	}
}

// ─── BindJSON ────────────────────────────────────────────────────────────────

func TestBindJSON_ValidBody_ParsedSuccessfully(t *testing.T) {
	body := `{"name":"Alice","age":30}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	if err := router.BindJSON(req, &dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dst.Name != "Alice" || dst.Age != 30 {
		t.Errorf("want {Alice 30}, got {%s %d}", dst.Name, dst.Age)
	}
}

func TestBindJSON_MalformedJSON_Returns400(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`{bad}`))
	req.Header.Set("Content-Type", "application/json")

	var dst map[string]any
	err := router.BindJSON(req, &dst)
	assertHTTPError(t, err, http.StatusBadRequest)

	// Error message must NOT echo the raw body.
	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if strings.Contains(he.Message, "bad") {
		t.Errorf("error message must not echo raw body content, got: %s", he.Message)
	}
}

func TestBindJSON_UnknownField_StrictMode_Returns400(t *testing.T) {
	body := `{"name":"Alice","extra":"field"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	err := router.BindJSON(req, &dst)
	assertHTTPError(t, err, http.StatusBadRequest)

	// Message must name the offending field.
	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if !strings.Contains(he.Message, "extra") {
		t.Errorf("want message naming offending field %q, got: %s", "extra", he.Message)
	}
}

func TestBindJSON_UnknownField_LenientMode_Succeeds(t *testing.T) {
	body := `{"name":"Bob","extra":"ignored"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst struct {
		Name string `json:"name"`
	}
	if err := router.BindJSON(req, &dst, router.WithStrictJSON(false)); err != nil {
		t.Fatalf("lenient mode must not fail on unknown field, got: %v", err)
	}
	if dst.Name != "Bob" {
		t.Errorf("want Name=Bob, got %q", dst.Name)
	}
}

func TestBindJSON_BodyExceedsLimit_Returns413(t *testing.T) {
	// Use a 4-byte body limit so this tiny payload triggers the limit.
	body := `{"name":"Alice"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst map[string]any
	err := router.BindJSON(req, &dst, router.WithBodyLimit(4))
	assertHTTPError(t, err, http.StatusRequestEntityTooLarge)
}

func TestBindJSON_WrongContentType_Returns415(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`<root/>`))
	req.Header.Set("Content-Type", "application/xml")

	var dst map[string]any
	err := router.BindJSON(req, &dst)
	assertHTTPError(t, err, http.StatusUnsupportedMediaType)
}

func TestBindJSON_NoContentType_Succeeds(t *testing.T) {
	// Missing Content-Type header should not cause a 415; treat as acceptable.
	body := `{"key":"val"}`
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))

	var dst map[string]any
	if err := router.BindJSON(req, &dst); err != nil {
		t.Fatalf("missing Content-Type should be accepted, got: %v", err)
	}
}

func TestBindJSON_CustomBodyLimit_Enforced(t *testing.T) {
	// A body just over the custom limit triggers 413.
	body := `{"k":"vvvvvvvvvv"}` // > 8 bytes
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	var dst map[string]any
	err := router.BindJSON(req, &dst, router.WithBodyLimit(8))
	assertHTTPError(t, err, http.StatusRequestEntityTooLarge)
}

func TestBindJSON_DefaultBodyLimitFinite(t *testing.T) {
	// Verify DefaultBodyLimit is accessible and positive.
	if router.DefaultBodyLimit <= 0 {
		t.Errorf("DefaultBodyLimit must be positive, got %d", router.DefaultBodyLimit)
	}
}

// ─── BindForm ─────────────────────────────────────────────────────────────────

func TestBindForm_ValidBody_ParsedSuccessfully(t *testing.T) {
	body := "name=Alice&age=30"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if err := router.BindForm(req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	name, err := router.FormParam(req, "name")
	if err != nil {
		t.Fatalf("FormParam error: %v", err)
	}
	if name != "Alice" {
		t.Errorf("want name=Alice, got %q", name)
	}
}

func TestBindForm_WrongContentType_Returns415(t *testing.T) {
	req := httptest.NewRequest("POST", "/", strings.NewReader(`<xml/>`))
	req.Header.Set("Content-Type", "application/xml")

	err := router.BindForm(req)
	assertHTTPError(t, err, http.StatusUnsupportedMediaType)
}

// ─── FormParam ────────────────────────────────────────────────────────────────

func TestFormParam_Missing_Returns400NamingParam(t *testing.T) {
	body := "other=val"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = router.BindForm(req)

	_, err := router.FormParam(req, "name")
	assertHTTPError(t, err, http.StatusBadRequest)

	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if !strings.Contains(he.Message, "name") {
		t.Errorf("error message must name the parameter %q, got: %s", "name", he.Message)
	}
}

func TestFormParamInt_Valid(t *testing.T) {
	body := "count=7"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = router.BindForm(req)

	n, err := router.FormParamInt(req, "count")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 7 {
		t.Errorf("want 7, got %d", n)
	}
}

func TestFormParamInt_NonNumeric_Returns400NamingParam(t *testing.T) {
	body := "count=abc"
	req := httptest.NewRequest("POST", "/", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	_ = router.BindForm(req)

	_, err := router.FormParamInt(req, "count")
	assertHTTPError(t, err, http.StatusBadRequest)

	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if !strings.Contains(he.Message, "count") {
		t.Errorf("error must name the parameter %q, got: %s", "count", he.Message)
	}
}

// ─── QueryParam ──────────────────────────────────────────────────────────────

func TestQueryParam_Present_ReturnsValue(t *testing.T) {
	req := httptest.NewRequest("GET", "/search?q=hello", nil)
	v, err := router.QueryParam(req, "q")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != "hello" {
		t.Errorf("want %q, got %q", "hello", v)
	}
}

func TestQueryParam_Missing_Returns400NamingParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/search", nil)
	_, err := router.QueryParam(req, "q")
	assertHTTPError(t, err, http.StatusBadRequest)

	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if !strings.Contains(he.Message, "q") {
		t.Errorf("error must name the missing parameter %q, got: %s", "q", he.Message)
	}
}

func TestQueryParam_EmptyValue_Succeeds(t *testing.T) {
	// ?k= means the key is present with empty value — not missing.
	req := httptest.NewRequest("GET", "/?k=", nil)
	v, err := router.QueryParam(req, "k")
	if err != nil {
		t.Fatalf("key present with empty value must succeed, got: %v", err)
	}
	if v != "" {
		t.Errorf("want empty string, got %q", v)
	}
}

// ─── QueryParamInt ───────────────────────────────────────────────────────────

func TestQueryParamInt_Valid_ReturnsInt(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=25", nil)
	n, err := router.QueryParamInt(req, "limit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 25 {
		t.Errorf("want 25, got %d", n)
	}
}

func TestQueryParamInt_Missing_Returns400(t *testing.T) {
	req := httptest.NewRequest("GET", "/", nil)
	_, err := router.QueryParamInt(req, "limit")
	assertHTTPError(t, err, http.StatusBadRequest)
}

func TestQueryParamInt_NonNumeric_Returns400NamingParam(t *testing.T) {
	req := httptest.NewRequest("GET", "/?limit=abc", nil)
	_, err := router.QueryParamInt(req, "limit")
	assertHTTPError(t, err, http.StatusBadRequest)

	var he *httperrors.HTTPError
	stderrors.As(err, &he)
	if !strings.Contains(he.Message, "limit") {
		t.Errorf("error must name the parameter %q, got: %s", "limit", he.Message)
	}
}

// ─── PathParam ───────────────────────────────────────────────────────────────

func TestPathParam_Present_ReturnsValue(t *testing.T) {
	r := router.New()
	var gotID string
	_ = r.GET("/items/:id", func(w http.ResponseWriter, req *http.Request) {
		id, err := router.PathParam(req, "id")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		gotID = id
		w.WriteHeader(http.StatusOK)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/items/42", nil))

	if gotID != "42" {
		t.Errorf("want id=42, got %q", gotID)
	}
}

func TestPathParam_Absent_Returns400(t *testing.T) {
	// Simulate a handler that asks for a param that the route didn't set.
	r := router.New()
	var gotErr error
	_ = r.GET("/items", func(w http.ResponseWriter, req *http.Request) {
		_, gotErr = router.PathParam(req, "id")
		w.WriteHeader(http.StatusOK)
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/items", nil))

	assertHTTPError(t, gotErr, http.StatusBadRequest)
}
