// Purpose: Tests for Router.Use and Group.Use verifying that middleware chains compose
// correctly with global chains and group chains in registration order (ADR-009).

package router

import (
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

// recordMW returns a func(http.Handler) http.Handler that appends name+":before" and
// name+":after" to seq (guarded by mu) around the next handler call.
func recordMW(name string, mu *sync.Mutex, seq *[]string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			*seq = append(*seq, name+":before")
			mu.Unlock()
			next.ServeHTTP(w, r)
			mu.Lock()
			*seq = append(*seq, name+":after")
			mu.Unlock()
		})
	}
}

// TestRouterUseAppliesGlobally verifies that middleware registered with Router.Use
// wraps all routes, regardless of which route is matched.
func TestRouterUseAppliesGlobally(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	ro := New()
	ro.Use(recordMW("global", &mu, &seq))

	_ = ro.GET("/a", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler-a")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	_ = ro.GET("/b", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler-b")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	for _, tc := range []struct {
		path    string
		handler string
	}{
		{"/a", "handler-a"},
		{"/b", "handler-b"},
	} {
		seq = nil
		ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, tc.path, nil))
		want := []string{"global:before", tc.handler, "global:after"}
		if !reflect.DeepEqual(seq, want) {
			t.Errorf("%s order = %v, want %v", tc.path, seq, want)
		}
	}
}

// TestGroupUseAppliesOnlyToGroup verifies that Group.Use middleware wraps only routes
// registered through that group and not routes registered on the router directly.
func TestGroupUseAppliesOnlyToGroup(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	ro := New()
	g := ro.Group("/api")
	g.Use(recordMW("group", &mu, &seq))

	_ = ro.GET("/root", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "root")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	_ = g.GET("/users", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "group-handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	// Root route must not see group middleware.
	seq = nil
	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/root", nil))
	if !reflect.DeepEqual(seq, []string{"root"}) {
		t.Errorf("/root order = %v, want [root]", seq)
	}

	// Group route must see group middleware.
	seq = nil
	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/users", nil))
	want := []string{"group:before", "group-handler", "group:after"}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("/api/users order = %v, want %v", seq, want)
	}
}

// TestGlobalAndGroupChainOrder verifies that the global chain runs before the group chain:
// global→group→handler (ADR-009, Phase 3 spec: "A group's chain runs after its parent's").
func TestGlobalAndGroupChainOrder(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	ro := New()
	ro.Use(recordMW("global", &mu, &seq))

	g := ro.Group("/api")
	g.Use(recordMW("group", &mu, &seq))

	_ = g.GET("/resource", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/resource", nil))

	want := []string{
		"global:before", "group:before",
		"handler",
		"group:after", "global:after",
	}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("order = %v\nwant  %v", seq, want)
	}
}

// TestNestedGroupChainOrder verifies that for nested groups the middleware chains compose
// as grandparent→parent→child→handler.
func TestNestedGroupChainOrder(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	ro := New()
	ro.Use(recordMW("global", &mu, &seq))

	api := ro.Group("/api")
	api.Use(recordMW("api", &mu, &seq))

	v1 := api.Group("/v1")
	v1.Use(recordMW("v1", &mu, &seq))

	_ = v1.GET("/items", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/v1/items", nil))

	want := []string{
		"global:before", "api:before", "v1:before",
		"handler",
		"v1:after", "api:after", "global:after",
	}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("nested order = %v\nwant           %v", seq, want)
	}
}

// TestRouterConcurrentRequests verifies no data race when many goroutines send requests
// through a chain simultaneously.
func TestRouterConcurrentRequests(t *testing.T) {
	const goroutines = 20
	const reqsPerGoroutine = 50

	ro := New()
	ro.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, r)
		})
	})
	_ = ro.GET("/ping", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	var wg sync.WaitGroup
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < reqsPerGoroutine; j++ {
				w := httptest.NewRecorder()
				r := httptest.NewRequest(http.MethodGet, "/ping", nil)
				ro.ServeHTTP(w, r)
				if w.Code != http.StatusOK {
					t.Errorf("unexpected status %d", w.Code)
				}
			}
		}()
	}
	wg.Wait()
}

// TestMultipleUseCallsAccumulate verifies that multiple Router.Use calls are additive
// and execute in the order they were registered.
func TestMultipleUseCallsAccumulate(t *testing.T) {
	var mu sync.Mutex
	var seq []string

	ro := New()
	ro.Use(recordMW("first", &mu, &seq))
	ro.Use(recordMW("second", &mu, &seq))

	_ = ro.GET("/", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seq = append(seq, "handler")
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})

	ro.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := []string{
		"first:before", "second:before",
		"handler",
		"second:after", "first:after",
	}
	if !reflect.DeepEqual(seq, want) {
		t.Errorf("order = %v, want %v", seq, want)
	}
}
