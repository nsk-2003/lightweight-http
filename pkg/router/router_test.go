// Purpose: Black-box tests for the router package covering method dispatch, path
// parameters, query values, route groups, error handling, the behavioral
// requirement table from docs/specifications/phase2.md, and Phase 5 JSON
// envelope requirements for 404 and 405 responses.
package router_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/lightweight-http/pkg/router"
)

// Compile-time assertion: Router must satisfy http.Handler.
var _ http.Handler = (*router.Router)(nil)

// hit returns a handler that records whether it was called.
func hit(called *bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
	}
}

// ─── Method dispatch ─────────────────────────────────────────────────────────

func TestRouter_MethodDispatch(t *testing.T) {
	type registerFn func(r *router.Router, pattern string, h http.HandlerFunc) error

	cases := []struct {
		method string
		reg    registerFn
	}{
		{"GET", func(r *router.Router, p string, h http.HandlerFunc) error { return r.GET(p, h) }},
		{"POST", func(r *router.Router, p string, h http.HandlerFunc) error { return r.POST(p, h) }},
		{"PUT", func(r *router.Router, p string, h http.HandlerFunc) error { return r.PUT(p, h) }},
		{"DELETE", func(r *router.Router, p string, h http.HandlerFunc) error { return r.DELETE(p, h) }},
		{"PATCH", func(r *router.Router, p string, h http.HandlerFunc) error { return r.PATCH(p, h) }},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			r := router.New()
			called := false
			if err := tc.reg(r, "/ping", hit(&called)); err != nil {
				t.Fatalf("register %s: %v", tc.method, err)
			}
			req := httptest.NewRequest(tc.method, "/ping", nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if !called {
				t.Errorf("%s /ping: handler not called", tc.method)
			}
		})
	}
}

// ─── Path parameters ─────────────────────────────────────────────────────────

func TestRouter_PathParam_Single(t *testing.T) {
	r := router.New()
	var gotID string
	if err := r.GET("/users/:id", func(w http.ResponseWriter, req *http.Request) {
		gotID = router.Params(req.Context())["id"]
	}); err != nil {
		t.Fatal(err)
	}

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users/42", nil))

	if gotID != "42" {
		t.Errorf("want id=42, got %q", gotID)
	}
}

func TestRouter_PathParam_Multiple(t *testing.T) {
	r := router.New()
	var gotUser, gotPost string
	if err := r.GET("/users/:user/posts/:post", func(w http.ResponseWriter, req *http.Request) {
		p := router.Params(req.Context())
		gotUser = p["user"]
		gotPost = p["post"]
	}); err != nil {
		t.Fatal(err)
	}

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users/alice/posts/99", nil))

	if gotUser != "alice" || gotPost != "99" {
		t.Errorf("want user=alice post=99, got user=%q post=%q", gotUser, gotPost)
	}
}

// ─── 404 / 405 ───────────────────────────────────────────────────────────────

func TestRouter_NotFound(t *testing.T) {
	r := router.New()
	_ = r.GET("/exists", hit(new(bool)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404, got %d", w.Code)
	}
}

func TestRouter_MethodNotAllowed_StatusAndAllowHeader(t *testing.T) {
	r := router.New()
	_ = r.GET("/resource", hit(new(bool)))
	_ = r.POST("/resource", hit(new(bool)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("DELETE", "/resource", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("want 405, got %d", w.Code)
	}
	allow := w.Header().Get("Allow")
	if !strings.Contains(allow, "GET") {
		t.Errorf("Allow header %q missing GET", allow)
	}
	if !strings.Contains(allow, "POST") {
		t.Errorf("Allow header %q missing POST", allow)
	}
}

// ─── Route groups ─────────────────────────────────────────────────────────────

func TestRouter_Group_PrefixCompose(t *testing.T) {
	r := router.New()
	called := false
	v1 := r.Group("/api/v1")
	if err := v1.GET("/users", hit(&called)); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/api/v1/users", nil))

	if !called {
		t.Error("group handler not called for /api/v1/users")
	}
	if w.Code != http.StatusOK {
		t.Errorf("want 200, got %d", w.Code)
	}
}

func TestRouter_Group_AllMethods(t *testing.T) {
	r := router.New()
	g := r.Group("/v2")

	type regFn func(pattern string, h http.HandlerFunc) error
	cases := []struct {
		method string
		reg    regFn
	}{
		{"GET", g.GET},
		{"POST", g.POST},
		{"PUT", g.PUT},
		{"DELETE", g.DELETE},
		{"PATCH", g.PATCH},
	}

	for _, tc := range cases {
		t.Run(tc.method, func(t *testing.T) {
			called := false
			path := "/item-" + tc.method
			if err := tc.reg(path, hit(&called)); err != nil {
				t.Fatalf("register %s: %v", tc.method, err)
			}
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, "/v2"+path, nil))
			if !called {
				t.Errorf("group %s handler not called", tc.method)
			}
		})
	}
}

func TestRouter_NestedGroups(t *testing.T) {
	r := router.New()
	called := false
	v1 := r.Group("/api/v1")
	admin := v1.Group("/admin")
	if err := admin.GET("/settings", hit(&called)); err != nil {
		t.Fatal(err)
	}

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/api/v1/admin/settings", nil))

	if !called {
		t.Error("nested group handler not called for /api/v1/admin/settings")
	}
}

// ─── Query values ─────────────────────────────────────────────────────────────

func TestRouter_QueryValues_Single(t *testing.T) {
	r := router.New()
	var got string
	_ = r.GET("/search", func(w http.ResponseWriter, req *http.Request) {
		got = router.QueryValues(req).Get("q")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/search?q=hello", nil))

	if got != "hello" {
		t.Errorf("want q=hello, got %q", got)
	}
}

func TestRouter_QueryValues_Repeated(t *testing.T) {
	r := router.New()
	var got []string
	_ = r.GET("/filter", func(w http.ResponseWriter, req *http.Request) {
		got = router.QueryValues(req)["a"]
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/filter?a=1&a=2", nil))

	if len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Errorf("want [1 2], got %v", got)
	}
}

func TestRouter_QueryValues_Empty(t *testing.T) {
	r := router.New()
	var got string
	_ = r.GET("/empty", func(w http.ResponseWriter, req *http.Request) {
		// key present with no value: ?k=
		got = router.QueryValues(req).Get("k")
	})

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/empty?k=", nil))

	if got != "" {
		t.Errorf("want empty string, got %q", got)
	}
}

// ─── Percent-encoded paths ────────────────────────────────────────────────────

func TestRouter_PathParam_PercentDecoded(t *testing.T) {
	r := router.New()
	var gotID string
	_ = r.GET("/items/:id", func(w http.ResponseWriter, req *http.Request) {
		gotID = router.Params(req.Context())["id"]
	})

	// net/http decodes the path before calling ServeHTTP; %20 → space
	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/items/hello%20world", nil))

	if gotID != "hello world" {
		t.Errorf("want 'hello world', got %q", gotID)
	}
}

// ─── Trailing slash ───────────────────────────────────────────────────────────

// Trailing slash is a distinct pattern: /users and /users/ are different routes.
func TestRouter_TrailingSlash_Distinct(t *testing.T) {
	r := router.New()
	calledNoSlash := false
	calledSlash := false
	_ = r.GET("/users", hit(&calledNoSlash))
	_ = r.GET("/users/", hit(&calledSlash))

	t.Run("no trailing slash calls correct handler", func(t *testing.T) {
		calledNoSlash = false
		calledSlash = false
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users", nil))
		if !calledNoSlash {
			t.Error("/users handler not called")
		}
		if calledSlash {
			t.Error("/users/ handler incorrectly called")
		}
	})

	t.Run("trailing slash calls correct handler", func(t *testing.T) {
		calledNoSlash = false
		calledSlash = false
		r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users/", nil))
		if !calledSlash {
			t.Error("/users/ handler not called")
		}
		if calledNoSlash {
			t.Error("/users handler incorrectly called")
		}
	})
}

func TestRouter_TrailingSlash_404WhenUnregistered(t *testing.T) {
	r := router.New()
	_ = r.GET("/users", hit(new(bool)))

	// /users/ not registered — expect 404
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest("GET", "/users/", nil))

	if w.Code != http.StatusNotFound {
		t.Errorf("want 404 for unregistered /users/, got %d", w.Code)
	}
}

// ─── Registration errors ──────────────────────────────────────────────────────

func TestRouter_DuplicateParamNames_Error(t *testing.T) {
	r := router.New()
	err := r.GET("/users/:id/posts/:id", hit(new(bool)))
	if err == nil {
		t.Error("expected error for duplicate parameter name :id in one pattern")
	}
}

func TestRouter_DuplicateRoute_Error(t *testing.T) {
	r := router.New()
	if err := r.GET("/users", hit(new(bool))); err != nil {
		t.Fatalf("first register: %v", err)
	}
	if err := r.GET("/users", hit(new(bool))); err == nil {
		t.Error("expected error for duplicate GET /users registration")
	}
}

// ─── Static wins over parameter ───────────────────────────────────────────────

func TestRouter_StaticWinsOverParam(t *testing.T) {
	r := router.New()
	calledStatic := false
	calledParam := false
	_ = r.GET("/users/list", hit(&calledStatic))
	_ = r.GET("/users/:id", hit(&calledParam))

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/users/list", nil))

	if !calledStatic {
		t.Error("static handler (/users/list) should have been called")
	}
	if calledParam {
		t.Error("param handler (/users/:id) must not be called when static matches")
	}
}

// ─── Behavioral requirements table (phase2.md) ────────────────────────────────

func TestRouter_BehavioralTable(t *testing.T) {
	r := router.New()
	_ = r.GET("/resource", hit(new(bool)))
	_ = r.POST("/resource", hit(new(bool)))

	cases := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantAllow  string // substring expected in Allow header; empty = not checked
	}{
		{"path+method match", "GET", "/resource", http.StatusOK, ""},
		{"path match method mismatch", "DELETE", "/resource", http.StatusMethodNotAllowed, "GET"},
		{"no path match", "GET", "/noexist", http.StatusNotFound, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
			if w.Code != tc.wantStatus {
				t.Errorf("want %d, got %d", tc.wantStatus, w.Code)
			}
			if tc.wantAllow != "" {
				allow := w.Header().Get("Allow")
				if !strings.Contains(allow, tc.wantAllow) {
					t.Errorf("Allow header %q missing %q", allow, tc.wantAllow)
				}
			}
		})
	}
}

// ─── Root path ────────────────────────────────────────────────────────────────

func TestRouter_RootPath(t *testing.T) {
	r := router.New()
	called := false
	_ = r.GET("/", hit(&called))

	r.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest("GET", "/", nil))

	if !called {
		t.Error("root handler not called for GET /")
	}
}

// ─── Phase 5 JSON envelope for 404 and 405 ───────────────────────────────────

// TestRouter_NotFound_IsJSONEnvelope asserts that a 404 response from the router
// uses the standard error envelope (Phase 5 acceptance criterion).
func TestRouter_NotFound_IsJSONEnvelope(t *testing.T) {
	r := router.New()
	_ = r.GET("/exists", hit(new(bool)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))

	if w.Code != http.StatusNotFound {
		t.Fatalf("want 404, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("404 response must be application/json, got %q", ct)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("404 body is not valid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if _, ok := env["error"]; !ok {
		t.Error("404 response must have top-level 'error' key in JSON body")
	}
}

// TestRouter_MethodNotAllowed_IsJSONEnvelope asserts that a 405 response uses
// the standard error envelope and still carries the Allow header (Phase 5).
func TestRouter_MethodNotAllowed_IsJSONEnvelope(t *testing.T) {
	r := router.New()
	_ = r.GET("/resource", hit(new(bool)))

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/resource", nil))

	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("want 405, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Fatalf("405 response must be application/json, got %q", ct)
	}
	if allow := w.Header().Get("Allow"); !strings.Contains(allow, "GET") {
		t.Fatalf("405 response must still include Allow header, got %q", allow)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
		t.Fatalf("405 body is not valid JSON: %v\nbody: %s", err, w.Body.String())
	}
	if _, ok := env["error"]; !ok {
		t.Error("405 response must have top-level 'error' key in JSON body")
	}
}

// ─── Benchmark ────────────────────────────────────────────────────────────────

func BenchmarkRouter_Match(b *testing.B) {
	r := router.New()
	prefixes := []string{"/api/v1", "/api/v2", "/internal"}
	resources := []string{
		"users", "posts", "comments", "tags", "categories",
		"orders", "products", "reviews", "sessions", "tokens",
	}
	nop := func(w http.ResponseWriter, req *http.Request) {}
	for _, prefix := range prefixes {
		g := r.Group(prefix)
		for _, res := range resources {
			_ = g.GET("/"+res, nop)
			_ = g.GET("/"+res+"/:id", nop)
			_ = g.POST("/"+res, nop)
		}
	}

	// Target: a param route deep in the table
	req := httptest.NewRequest("GET", "/api/v2/products/42", nil)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
	}
}
