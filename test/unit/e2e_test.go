// Purpose: End-to-end test that starts the full server stack with httptest.NewServer and
// drives every route in the /api/v1 table, verifying the complete middleware chain (Phase 8).

package unit_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/example/lightweight-http/examples/items"
	"github.com/example/lightweight-http/pkg/di"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// newTestServer builds a complete server (DI + middleware + routes) backed by httptest.
func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	c := di.New()
	if err := c.Register(items.StoreKey, func(_ *di.Resolver) (any, error) {
		return items.NewItemStore(), nil
	}, di.Singleton); err != nil {
		t.Fatalf("DI register: %v", err)
	}
	col := observability.NewInProcessCollector()
	ro := router.New()
	ro.Use(
		observability.RequestID(),
		observability.MetricsMiddleware(col),
		middleware.Recovery(nil),
	)
	v1 := ro.Group("/api/v1")
	items.RegisterRoutes(v1, c, col, false)
	return httptest.NewServer(ro)
}

func TestE2EHealth(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
	var body map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" {
		t.Errorf("status field = %q, want %q", body["status"], "ok")
	}
}

func TestE2EItemsCRUD(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := srv.Client()

	// POST — create
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/items",
		strings.NewReader(`{"name":"widget","note":"blue"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("POST /items: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("create status = %d, want 201", resp.StatusCode)
	}
	var created items.Item
	if err := json.NewDecoder(resp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" || created.Name != "widget" {
		t.Errorf("unexpected created item: %+v", created)
	}

	// GET list
	resp2, err2 := client.Get(srv.URL + "/api/v1/items")
	if err2 != nil {
		t.Fatalf("GET /items: %v", err2)
	}
	defer resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Errorf("list status = %d, want 200", resp2.StatusCode)
	}
	var list []items.Item
	if err := json.NewDecoder(resp2.Body).Decode(&list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("list len = %d, want 1", len(list))
	}

	// GET :id — path parameter exercise
	resp3, err3 := client.Get(srv.URL + "/api/v1/items/" + created.ID)
	if err3 != nil {
		t.Fatalf("GET /items/:id: %v", err3)
	}
	defer resp3.Body.Close()
	if resp3.StatusCode != http.StatusOK {
		t.Errorf("get :id status = %d, want 200", resp3.StatusCode)
	}

	// DELETE :id
	delReq, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/items/"+created.ID, nil)
	resp4, err4 := client.Do(delReq)
	if err4 != nil {
		t.Fatalf("DELETE /items/:id: %v", err4)
	}
	defer resp4.Body.Close()
	if resp4.StatusCode != http.StatusNoContent {
		t.Errorf("delete status = %d, want 204", resp4.StatusCode)
	}

	// GET :id after delete → 404 via error envelope
	resp5, err5 := client.Get(srv.URL + "/api/v1/items/" + created.ID)
	if err5 != nil {
		t.Fatalf("GET deleted item: %v", err5)
	}
	defer resp5.Body.Close()
	if resp5.StatusCode != http.StatusNotFound {
		t.Errorf("get deleted status = %d, want 404", resp5.StatusCode)
	}
	body, _ := io.ReadAll(resp5.Body)
	if !bytes.Contains(body, []byte(`"not_found"`)) {
		t.Errorf("404 body missing not_found code: %s", body)
	}
}

func TestE2EItemsLimitQuery(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := srv.Client()

	for _, name := range []string{"alpha", "beta"} {
		req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/items",
			strings.NewReader(`{"name":"`+name+`"}`))
		req.Header.Set("Content-Type", "application/json")
		resp, _ := client.Do(req)
		resp.Body.Close()
	}

	resp, err := client.Get(srv.URL + "/api/v1/items?limit=1")
	if err != nil {
		t.Fatalf("GET /items?limit=1: %v", err)
	}
	defer resp.Body.Close()
	var list []items.Item
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list) != 1 {
		t.Errorf("limit=1 returned %d items, want 1", len(list))
	}
}

func TestE2EItemsValidation(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// Missing name → 422 with field detail
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/v1/items",
		strings.NewReader(`{"note":"no name"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatalf("POST invalid: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("validation status = %d, want 422", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("name")) {
		t.Errorf("validation body missing field name: %s", body)
	}
}

func TestE2EMetrics(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()
	client := srv.Client()

	// Trigger a request to populate metrics
	resp0, _ := client.Get(srv.URL + "/api/v1/health")
	resp0.Body.Close()

	resp, err := client.Get(srv.URL + "/api/v1/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("metrics status = %d, want 200", resp.StatusCode)
	}
	// Snapshot is a JSON array of metric rows
	var rows []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&rows); err != nil {
		t.Fatalf("decode metrics: %v", err)
	}
	if len(rows) == 0 {
		t.Error("metrics snapshot is empty after a request was made")
	}
}

func TestE2EBoomRecovers(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	// /boom panics deliberately; Recovery middleware must convert it to 500
	resp, err := srv.Client().Get(srv.URL + "/api/v1/boom")
	if err != nil {
		t.Fatalf("GET /boom: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("boom status = %d, want 500", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(body, []byte("internal_error")) {
		t.Errorf("boom body missing internal_error: %s", body)
	}
}

func TestE2ENotFound(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/nope")
	if err != nil {
		t.Fatalf("GET /nope: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown route status = %d, want 404", resp.StatusCode)
	}
}

func TestE2ERequestIDHeader(t *testing.T) {
	srv := newTestServer(t)
	defer srv.Close()

	resp, err := srv.Client().Get(srv.URL + "/api/v1/health")
	if err != nil {
		t.Fatalf("GET /health: %v", err)
	}
	defer resp.Body.Close()
	if resp.Header.Get("X-Request-ID") == "" {
		t.Error("X-Request-ID header missing from response")
	}
}
