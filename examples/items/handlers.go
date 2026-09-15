// Purpose: HTTP handlers and route registration for the items API example (Phase 8).

package items

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/example/lightweight-http/pkg/di"
	httperr "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// StoreKey is the DI container key for the *ItemStore singleton.
const StoreKey = "items.ItemStore"

// metricsRow is the JSON shape for one entry in GET /api/v1/metrics.
type metricsRow struct {
	Route          string        `json:"route"`
	Method         string        `json:"method"`
	RequestCount   int64         `json:"request_count"`
	TotalLatencyMs float64       `json:"total_latency_ms"`
	StatusCodes    map[int]int64 `json:"status_codes"`
}

// RegisterRoutes registers all items API routes on the given route group.
// The DI container c must have an *ItemStore registered under StoreKey.
// col is the metrics collector wired into the middleware chain.
// debug enables verbose error logging via errors.Handle.
func RegisterRoutes(v1 *router.Group, c *di.Container, col *observability.InProcessCollector, debug bool) {
	must := func(err error) {
		if err != nil {
			panic(fmt.Sprintf("items: route registration failed: %v", err))
		}
	}

	must(v1.GET("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := router.WriteJSON(w, r, http.StatusOK, map[string]string{"status": "ok"}); err != nil {
			httperr.Handle(w, r, err, nil, debug)
		}
	}))

	must(v1.GET("/items", func(w http.ResponseWriter, r *http.Request) {
		store := resolveStore(c, w, r, debug)
		if store == nil {
			return
		}
		limit := 0
		if s := router.QueryParam(r, "limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		list := store.List(limit)
		if list == nil {
			list = []*Item{}
		}
		if err := router.WriteJSON(w, r, http.StatusOK, list); err != nil {
			httperr.Handle(w, r, err, nil, debug)
		}
	}))

	must(v1.POST("/items", func(w http.ResponseWriter, r *http.Request) {
		store := resolveStore(c, w, r, debug)
		if store == nil {
			return
		}
		var req CreateItemRequest
		if err := router.ParseJSON(r, &req); err != nil {
			httperr.Handle(w, r, err, nil, debug)
			return
		}
		if req.Name == "" {
			httperr.Handle(w, r, httperr.UnprocessableEntity("validation failed",
				httperr.Detail{Field: "name", Reason: "name is required"},
			), nil, debug)
			return
		}
		item := store.Create(req)
		if err := router.WriteJSON(w, r, http.StatusCreated, item); err != nil {
			httperr.Handle(w, r, err, nil, debug)
		}
	}))

	must(v1.GET("/items/:id", func(w http.ResponseWriter, r *http.Request) {
		store := resolveStore(c, w, r, debug)
		if store == nil {
			return
		}
		id := router.PathParam(r, "id")
		item, ok := store.Get(id)
		if !ok {
			httperr.Handle(w, r, httperr.NotFound("item not found"), nil, debug)
			return
		}
		if err := router.WriteJSON(w, r, http.StatusOK, item); err != nil {
			httperr.Handle(w, r, err, nil, debug)
		}
	}))

	must(v1.DELETE("/items/:id", func(w http.ResponseWriter, r *http.Request) {
		store := resolveStore(c, w, r, debug)
		if store == nil {
			return
		}
		id := router.PathParam(r, "id")
		if !store.Delete(id) {
			httperr.Handle(w, r, httperr.NotFound("item not found"), nil, debug)
			return
		}
		router.NoContent(w)
	}))

	must(v1.GET("/metrics", func(w http.ResponseWriter, r *http.Request) {
		snap := col.Snapshot()
		rows := make([]metricsRow, 0, len(snap))
		for k, v := range snap {
			rows = append(rows, metricsRow{
				Route:          k.Route,
				Method:         k.Method,
				RequestCount:   v.RequestCount,
				TotalLatencyMs: v.TotalLatencyMs,
				StatusCodes:    v.StatusCodes,
			})
		}
		if err := router.WriteJSON(w, r, http.StatusOK, rows); err != nil {
			httperr.Handle(w, r, err, nil, debug)
		}
	}))

	must(v1.GET("/boom", func(w http.ResponseWriter, r *http.Request) {
		panic("deliberate panic: testing recovery middleware")
	}))
}

// resolveStore resolves the *ItemStore from the DI container.
// On failure it writes a 500 and returns nil so the caller can return immediately.
func resolveStore(c *di.Container, w http.ResponseWriter, r *http.Request, debug bool) *ItemStore {
	v, err := c.Resolve(StoreKey)
	if err != nil {
		httperr.Handle(w, r, httperr.Internal("service unavailable", err), nil, debug)
		return nil
	}
	return v.(*ItemStore)
}
