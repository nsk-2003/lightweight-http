// Purpose: End-to-end black-box tests for the items example package — drives
// every route through httptest.NewServer to verify the full middleware and
// routing stack (Phase 8 acceptance criterion).
package items_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/lightweight-http/examples/items"
	"github.com/example/lightweight-http/pkg/di"
	"github.com/example/lightweight-http/pkg/observability"
)

// newServer builds a wired-up test server with a fresh store and recorder.
func newServer(t *testing.T) (*httptest.Server, *observability.Recorder) {
	t.Helper()
	ctr := di.NewContainer()
	if err := ctr.Register(items.StoreKey, func(r di.Resolver) (any, error) {
		return items.NewStore(), nil
	}, di.Singleton); err != nil {
		t.Fatal(err)
	}
	rec := &observability.Recorder{}
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	h := items.NewHandler(ctr, rec, log, false)
	return httptest.NewServer(h), rec
}

func TestHealth(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want application/json", ct)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("body: got %v, want status=ok", body)
	}
}

func TestItems_ListEmpty(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/items")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status: got %d, want 200", resp.StatusCode)
	}
	var list []any
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 0 {
		t.Errorf("list: want empty, got %v", list)
	}
}

func TestItems_CreateAndGet(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	// POST to create
	resp, err := http.Post(srv.URL+"/api/v1/items",
		"application/json", strings.NewReader(`{"name":"widget"}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", resp.StatusCode)
	}
	var created map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode created: %v", err)
	}
	id, _ := created["id"].(string)
	if id == "" {
		t.Fatal("created item has no id")
	}
	if created["name"] != "widget" {
		t.Errorf("name: got %v, want widget", created["name"])
	}

	// GET the created item
	resp2, err := http.Get(srv.URL + "/api/v1/items/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("get: got %d, want 200", resp2.StatusCode)
	}
	var got map[string]any
	if err := json.NewDecoder(resp2.Body).Decode(&got); err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if got["id"] != id {
		t.Errorf("id: got %v, want %v", got["id"], id)
	}
}

func TestItems_Create_EmptyName(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Post(srv.URL+"/api/v1/items",
		"application/json", strings.NewReader(`{"name":""}`))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("got %d, want 400", resp.StatusCode)
	}
	var env map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("missing error envelope")
	}
	if errObj["code"] != "bad_request" {
		t.Errorf("error code: got %v, want bad_request", errObj["code"])
	}
	// Details list must be present
	details, _ := errObj["details"].([]any)
	if len(details) == 0 {
		t.Error("expected details list for validation error")
	}
}

func TestItems_GetNotFound(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/items/nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
	var env map[string]any
	json.NewDecoder(resp.Body).Decode(&env)
	errObj, _ := env["error"].(map[string]any)
	if errObj == nil || errObj["code"] != "not_found" {
		t.Errorf("expected not_found error envelope, got %v", env)
	}
}

func TestItems_DeleteAndConfirm(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	// Create
	resp, err := http.Post(srv.URL+"/api/v1/items",
		"application/json", strings.NewReader(`{"name":"to-delete"}`))
	if err != nil {
		t.Fatal(err)
	}
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	id := created["id"].(string)

	// Delete
	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/items/"+id, nil)
	resp2, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusNoContent {
		t.Errorf("delete: got %d, want 204", resp2.StatusCode)
	}

	// Confirm it is gone
	resp3, err3 := http.Get(srv.URL + "/api/v1/items/" + id)
	if err3 != nil {
		t.Fatal(err3)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusNotFound {
		t.Errorf("after delete: got %d, want 404", resp3.StatusCode)
	}
}

func TestItems_DeleteNotFound(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/items/ghost", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("got %d, want 404", resp.StatusCode)
	}
}

func TestItems_ListLimit(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	for _, name := range []string{"alpha", "beta", "gamma"} {
		http.Post(srv.URL+"/api/v1/items", "application/json",
			strings.NewReader(`{"name":"`+name+`"}`))
	}

	resp, err := http.Get(srv.URL + "/api/v1/items?limit=2")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	var list []any
	json.NewDecoder(resp.Body).Decode(&list)
	if len(list) != 2 {
		t.Errorf("limit=2: got %d items, want 2", len(list))
	}
}

func TestMetrics_Endpoint(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	// Generate traffic so the snapshot is non-empty
	http.Get(srv.URL + "/api/v1/health")
	http.Get(srv.URL + "/api/v1/health")

	resp, err := http.Get(srv.URL + "/api/v1/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("got %d, want 200", resp.StatusCode)
	}
	var metrics []any
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if len(metrics) == 0 {
		t.Error("metrics: want non-empty snapshot after traffic")
	}
}

func TestBoom_PanicIsRecovered(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/boom")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("got %d, want 500", resp.StatusCode)
	}
	var env map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	errObj, _ := env["error"].(map[string]any)
	if errObj == nil {
		t.Fatal("expected error envelope from recovered panic, got nil")
	}
}

func TestRequestID_HeaderEchoed(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()

	if id := resp.Header.Get("X-Request-ID"); id == "" {
		t.Error("X-Request-ID header absent from response")
	}
}

func TestRouteGroup_PathParams(t *testing.T) {
	srv, _ := newServer(t)
	defer srv.Close()

	// Create an item to verify path-param routing inside /api/v1 group
	resp, _ := http.Post(srv.URL+"/api/v1/items",
		"application/json", strings.NewReader(`{"name":"grouped"}`))
	var created map[string]any
	json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()
	id := created["id"].(string)

	// Access via the :id path parameter
	resp2, err := http.Get(srv.URL + "/api/v1/items/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("got %d, want 200", resp2.StatusCode)
	}
	var got map[string]any
	json.NewDecoder(resp2.Body).Decode(&got)
	if got["name"] != "grouped" {
		t.Errorf("name via :id path param: got %v, want grouped", got["name"])
	}
}
