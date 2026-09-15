// Purpose: Tests for the router package covering routing dispatch, path parameters, query access, groups, and error cases.

package router

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// compile-time assertion: Router must satisfy http.Handler.
var _ http.Handler = (*Router)(nil)

// handlerRecorder returns an http.HandlerFunc that records whether it was called.
func handlerRecorder(called *bool, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		*called = true
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(body))
	}
}

func noopHandler(w http.ResponseWriter, r *http.Request) {}

// TestMethodDispatch verifies that all five HTTP methods are registered and dispatched correctly.
func TestMethodDispatch(t *testing.T) {
	methods := []string{
		http.MethodGet,
		http.MethodPost,
		http.MethodPut,
		http.MethodDelete,
		http.MethodPatch,
	}
	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			ro := New()
			called := false
			if err := ro.handle(method, "/ping", handlerRecorder(&called, "ok")); err != nil {
				t.Fatalf("register: %v", err)
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(method, "/ping", nil)
			ro.ServeHTTP(w, r)
			if !called {
				t.Errorf("handler not called for method %s", method)
			}
			if w.Code != http.StatusOK {
				t.Errorf("status = %d, want 200", w.Code)
			}
		})
	}
}

// TestStaticRouteDispatch verifies handler selection for multiple static routes.
func TestStaticRouteDispatch(t *testing.T) {
	ro := New()
	if err := ro.GET("/foo", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("foo")) }); err != nil {
		t.Fatal(err)
	}
	if err := ro.GET("/bar", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("bar")) }); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		path string
		want string
	}{
		{"/foo", "foo"},
		{"/bar", "bar"},
	} {
		w := httptest.NewRecorder()
		ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if got := w.Body.String(); got != tc.want {
			t.Errorf("GET %s: body=%q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestSinglePathParam verifies extraction of a single path parameter.
func TestSinglePathParam(t *testing.T) {
	ro := New()
	var gotID string
	if err := ro.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
		gotID = PathParam(r, "id")
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/42", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotID != "42" {
		t.Errorf("PathParam(id) = %q, want %q", gotID, "42")
	}
}

// TestMultiplePathParams verifies extraction of multiple path parameters in one pattern.
func TestMultiplePathParams(t *testing.T) {
	ro := New()
	var gotUserID, gotPostID string
	if err := ro.GET("/users/:userId/posts/:postId", func(w http.ResponseWriter, r *http.Request) {
		gotUserID = PathParam(r, "userId")
		gotPostID = PathParam(r, "postId")
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/users/7/posts/99", nil))
	if gotUserID != "7" {
		t.Errorf("userId = %q, want %q", gotUserID, "7")
	}
	if gotPostID != "99" {
		t.Errorf("postId = %q, want %q", gotPostID, "99")
	}
}

// TestStaticWinsOverParam verifies that a static segment has higher priority than a parameter segment.
func TestStaticWinsOverParam(t *testing.T) {
	ro := New()
	if err := ro.GET("/users/me", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("me"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := ro.GET("/users/:id", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("param:" + PathParam(r, "id")))
	}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ path, want string }{
		{"/users/me", "me"},
		{"/users/42", "param:42"},
	} {
		w := httptest.NewRecorder()
		ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, tc.path, nil))
		if got := w.Body.String(); got != tc.want {
			t.Errorf("GET %s: body=%q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestNotFound verifies that a 404 is returned when no route matches.
func TestNotFound(t *testing.T) {
	ro := New()
	if err := ro.GET("/exists", noopHandler); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/missing", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("status = %d, want 404", w.Code)
	}
}

// TestMethodNotAllowed verifies that a 405 with a correct Allow header is returned
// when the path matches but the method does not.
func TestMethodNotAllowed(t *testing.T) {
	ro := New()
	if err := ro.GET("/res", noopHandler); err != nil {
		t.Fatal(err)
	}
	if err := ro.POST("/res", noopHandler); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/res", nil))
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want 405", w.Code)
	}

	allow := w.Header().Get("Allow")
	if allow == "" {
		t.Fatal("Allow header missing on 405 response")
	}
	// Both GET and POST must appear in the Allow header.
	if !strings.Contains(allow, "GET") {
		t.Errorf("Allow header %q does not contain GET", allow)
	}
	if !strings.Contains(allow, "POST") {
		t.Errorf("Allow header %q does not contain POST", allow)
	}
}

// TestTrailingSlashMismatch documents the chosen behavior: trailing slash is significant.
// /users and /users/ are distinct patterns and do not match each other.
func TestTrailingSlashMismatch(t *testing.T) {
	ro := New()
	if err := ro.GET("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("no-slash"))
	}); err != nil {
		t.Fatal(err)
	}

	// /users matches.
	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users", nil))
	if w.Code != http.StatusOK || w.Body.String() != "no-slash" {
		t.Errorf("/users: status=%d body=%q", w.Code, w.Body.String())
	}

	// /users/ does NOT match.
	w = httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/users/", nil))
	if w.Code != http.StatusNotFound {
		t.Errorf("/users/ (trailing slash): status=%d, want 404", w.Code)
	}
}

// TestDuplicateRouteError verifies that registering the same method and pattern twice
// returns an error rather than silently overwriting the handler.
func TestDuplicateRouteError(t *testing.T) {
	ro := New()
	if err := ro.GET("/dup", noopHandler); err != nil {
		t.Fatalf("first registration: %v", err)
	}
	if err := ro.GET("/dup", noopHandler); err == nil {
		t.Fatal("second registration: expected error for duplicate route, got nil")
	}
}

// TestDuplicateParamNameError verifies that a pattern with duplicate parameter names
// is rejected at registration time.
func TestDuplicateParamNameError(t *testing.T) {
	ro := New()
	err := ro.GET("/a/:id/b/:id", noopHandler)
	if err == nil {
		t.Fatal("expected error for duplicate param name :id, got nil")
	}
}

// TestPercentEncodedPathParam verifies that percent-encoded path segments are decoded
// before being stored as path parameter values.
func TestPercentEncodedPathParam(t *testing.T) {
	ro := New()
	var gotName string
	if err := ro.GET("/items/:name", func(w http.ResponseWriter, r *http.Request) {
		gotName = PathParam(r, "name")
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	// %41 decodes to "A"; the path param should be "ABC".
	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/items/%41BC", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if gotName != "ABC" {
		t.Errorf("name = %q, want %q", gotName, "ABC")
	}
}

// TestQueryParams verifies that query string values are accessible, including
// repeated keys and empty values.
func TestQueryParams(t *testing.T) {
	ro := New()
	var (
		singleVal string
		multiVals []string
		emptyVal  string
		absentVal string
	)
	if err := ro.GET("/search", func(w http.ResponseWriter, r *http.Request) {
		singleVal = QueryParam(r, "q")
		multiVals = r.URL.Query()["tag"]
		emptyVal = QueryParam(r, "empty")
		absentVal = QueryParam(r, "absent")
		w.WriteHeader(http.StatusOK)
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/search?q=hello&tag=go&tag=http&empty=", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if singleVal != "hello" {
		t.Errorf("q = %q, want %q", singleVal, "hello")
	}
	if len(multiVals) != 2 || multiVals[0] != "go" || multiVals[1] != "http" {
		t.Errorf("tag = %v, want [go http]", multiVals)
	}
	if emptyVal != "" {
		t.Errorf("empty = %q, want %q", emptyVal, "")
	}
	if absentVal != "" {
		t.Errorf("absent = %q, want %q", absentVal, "")
	}
}

// TestRouteGroup verifies that a route group prepends its prefix to all registered routes.
func TestRouteGroup(t *testing.T) {
	ro := New()
	v1 := ro.Group("/api/v1")
	if err := v1.GET("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	}); err != nil {
		t.Fatal(err)
	}
	if err := v1.POST("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("create"))
	}); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		method, path, want string
		status             int
	}{
		{http.MethodGet, "/api/v1/users", "users", http.StatusOK},
		{http.MethodPost, "/api/v1/users", "create", http.StatusOK},
		{http.MethodGet, "/users", "", http.StatusNotFound},
	} {
		w := httptest.NewRecorder()
		ro.ServeHTTP(w, httptest.NewRequest(tc.method, tc.path, nil))
		if w.Code != tc.status {
			t.Errorf("%s %s: status=%d, want %d", tc.method, tc.path, w.Code, tc.status)
		}
		if tc.want != "" && w.Body.String() != tc.want {
			t.Errorf("%s %s: body=%q, want %q", tc.method, tc.path, w.Body.String(), tc.want)
		}
	}
}

// TestNestedGroups verifies that nested groups compose their prefixes correctly.
func TestNestedGroups(t *testing.T) {
	ro := New()
	api := ro.Group("/api")
	v2 := api.Group("/v2")
	inner := v2.Group("/items")
	if err := inner.GET("/:id", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(PathParam(r, "id")))
	}); err != nil {
		t.Fatal(err)
	}

	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v2/items/77", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}
	if w.Body.String() != "77" {
		t.Errorf("body = %q, want %q", w.Body.String(), "77")
	}
}

// TestGroupAllMethods verifies that all five methods are accessible through a group.
func TestGroupAllMethods(t *testing.T) {
	ro := New()
	g := ro.Group("/v1")

	type registration struct {
		method string
		fn     func(string, http.HandlerFunc) error
		body   string
		called bool
	}
	regs := []*registration{
		{method: http.MethodGet, body: "get"},
		{method: http.MethodPost, body: "post"},
		{method: http.MethodPut, body: "put"},
		{method: http.MethodDelete, body: "del"},
		{method: http.MethodPatch, body: "patch"},
	}

	for _, reg := range regs {
		reg := reg // capture
		fn := func(pattern string, h http.HandlerFunc) error {
			return g.handle(reg.method, pattern, h)
		}
		if err := fn("/thing", handlerRecorder(&reg.called, reg.body)); err != nil {
			t.Fatalf("register %s: %v", reg.method, err)
		}
	}

	for _, reg := range regs {
		w := httptest.NewRecorder()
		ro.ServeHTTP(w, httptest.NewRequest(reg.method, "/v1/thing", nil))
		if !reg.called {
			t.Errorf("handler not called for %s", reg.method)
		}
		if w.Body.String() != reg.body {
			t.Errorf("%s: body=%q, want %q", reg.method, w.Body.String(), reg.body)
		}
		reg.called = false
	}
}

// TestRootPattern verifies that "/" can be registered and matched.
func TestRootPattern(t *testing.T) {
	ro := New()
	if err := ro.GET("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("root"))
	}); err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	ro.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))
	if w.Code != http.StatusOK || w.Body.String() != "root" {
		t.Errorf("root: status=%d body=%q", w.Code, w.Body.String())
	}
}

// TestRoutePattern verifies that RoutePattern returns the matched route pattern from context.
func TestRoutePattern(t *testing.T) {
	tests := []struct {
		name        string
		pattern     string
		requestPath string
		wantPattern string
	}{
		{"static", "/health", "/health", "/health"},
		{"with param", "/users/:id", "/users/42", "/users/:id"},
		{"root", "/", "/", "/"},
		{"multi param", "/a/:x/b/:y", "/a/1/b/2", "/a/:x/b/:y"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ro := New()
			var got string
			if err := ro.GET(tc.pattern, func(w http.ResponseWriter, r *http.Request) {
				got = RoutePattern(r)
				w.WriteHeader(http.StatusOK)
			}); err != nil {
				t.Fatal(err)
			}
			ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tc.requestPath, nil))
			if got != tc.wantPattern {
				t.Errorf("RoutePattern = %q, want %q", got, tc.wantPattern)
			}
		})
	}
}

// BenchmarkRouterMatch benchmarks route matching against a table of several dozen routes.
func BenchmarkRouterMatch(b *testing.B) {
	ro := New()

	routes := []struct {
		method, pattern string
	}{
		{http.MethodGet, "/"},
		{http.MethodGet, "/health"},
		{http.MethodGet, "/users"},
		{http.MethodPost, "/users"},
		{http.MethodGet, "/users/:id"},
		{http.MethodPut, "/users/:id"},
		{http.MethodDelete, "/users/:id"},
		{http.MethodGet, "/users/:id/profile"},
		{http.MethodPut, "/users/:id/profile"},
		{http.MethodGet, "/users/:id/posts"},
		{http.MethodPost, "/users/:id/posts"},
		{http.MethodGet, "/users/:id/posts/:postId"},
		{http.MethodPut, "/users/:id/posts/:postId"},
		{http.MethodDelete, "/users/:id/posts/:postId"},
		{http.MethodGet, "/products"},
		{http.MethodPost, "/products"},
		{http.MethodGet, "/products/:sku"},
		{http.MethodPut, "/products/:sku"},
		{http.MethodDelete, "/products/:sku"},
		{http.MethodGet, "/products/:sku/reviews"},
		{http.MethodPost, "/products/:sku/reviews"},
		{http.MethodGet, "/products/:sku/reviews/:reviewId"},
		{http.MethodGet, "/orders"},
		{http.MethodPost, "/orders"},
		{http.MethodGet, "/orders/:orderId"},
		{http.MethodPatch, "/orders/:orderId"},
		{http.MethodGet, "/orders/:orderId/items"},
		{http.MethodGet, "/categories"},
		{http.MethodGet, "/categories/:catId"},
		{http.MethodGet, "/categories/:catId/products"},
		{http.MethodGet, "/tags"},
		{http.MethodGet, "/tags/:tag"},
		{http.MethodGet, "/search"},
		{http.MethodGet, "/admin/dashboard"},
		{http.MethodGet, "/admin/users"},
		{http.MethodDelete, "/admin/users/:id"},
	}

	for _, rt := range routes {
		if err := ro.handle(rt.method, rt.pattern, noopHandler); err != nil {
			b.Fatalf("register %s %s: %v", rt.method, rt.pattern, err)
		}
	}

	req := httptest.NewRequest(http.MethodGet, "/users/42/posts/7", nil)
	w := httptest.NewRecorder()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Body.Reset()
		ro.ServeHTTP(w, req)
	}
}
