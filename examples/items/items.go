// Purpose: In-memory items API example — exercises routing with a route group
// and path parameters, typed JSON request parsing, structured responses, the
// error envelope, DI container resolution, and observability middleware (Phase 8).
package items

import (
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/example/lightweight-http/pkg/di"
	httperrors "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
	"github.com/example/lightweight-http/pkg/observability"
	"github.com/example/lightweight-http/pkg/router"
)

// StoreKey is the DI container key under which the Store singleton is registered.
const StoreKey = "store"

// Item is a single resource managed by the in-memory store.
type Item struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Store is a thread-safe in-memory item store. Register one as a Singleton in
// the DI container so all handlers share the same instance.
type Store struct {
	mu     sync.RWMutex
	data   map[string]*Item
	nextID uint64
}

// NewStore creates and returns an empty Store.
func NewStore() *Store {
	return &Store{data: make(map[string]*Item)}
}

// Add inserts a new Item with the given name and returns it. The ID is a
// decimal string derived from an atomically incremented counter.
func (s *Store) Add(name string) *Item {
	id := strconv.FormatUint(atomic.AddUint64(&s.nextID, 1), 10)
	item := &Item{ID: id, Name: name}
	s.mu.Lock()
	s.data[id] = item
	s.mu.Unlock()
	return item
}

// Get returns the item for id, or nil if not found.
func (s *Store) Get(id string) *Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.data[id]
}

// List returns up to limit items. A limit of zero or less returns all items.
func (s *Store) List(limit int) []*Item {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Item, 0, len(s.data))
	for _, item := range s.data {
		out = append(out, item)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// Delete removes the item for id. Returns false if the id was not present.
func (s *Store) Delete(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.data[id]; !ok {
		return false
	}
	delete(s.data, id)
	return true
}

// NewHandler builds and returns the HTTP handler for the items example. It
// resolves the Store from ctr using StoreKey, registers all routes on a shared
// /api/v1 group, and wraps the router with the full middleware chain
// (RequestID → Logger → Metrics → Recovery). debug controls stack-trace logging
// in Recovery and the error handler (ADR-008).
func NewHandler(ctr *di.Container, rec *observability.Recorder, log *slog.Logger, debug bool) http.Handler {
	svc, err := ctr.Resolve(StoreKey)
	if err != nil {
		panic(fmt.Sprintf("items.NewHandler: resolve %q: %v", StoreKey, err))
	}
	store := svc.(*Store)

	r := router.New()
	v1 := r.Group("/api/v1")

	// GET /api/v1/health — static JSON, no auth.
	mustRegister(v1.GET("/health", func(w http.ResponseWriter, req *http.Request) {
		router.JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// GET /api/v1/items — list all items, optional ?limit= query parameter.
	mustRegister(v1.GET("/items", func(w http.ResponseWriter, req *http.Request) {
		limit := 0
		if s := router.QueryValues(req).Get("limit"); s != "" {
			if n, err := strconv.Atoi(s); err == nil && n > 0 {
				limit = n
			}
		}
		list := store.List(limit)
		if list == nil {
			list = []*Item{}
		}
		router.JSON(w, http.StatusOK, list)
	}))

	// POST /api/v1/items — create an item; name must be non-empty.
	mustRegister(v1.POST("/items", func(w http.ResponseWriter, req *http.Request) {
		var input struct {
			Name string `json:"name"`
		}
		if err := router.BindJSON(req, &input); err != nil {
			httperrors.WriteError(w, req, err)
			return
		}
		if input.Name == "" {
			httperrors.WriteError(w, req, httperrors.ErrBadRequest.WithDetails(
				httperrors.Detail{Field: "name", Reason: "name is required"},
			))
			return
		}
		item := store.Add(input.Name)
		router.JSON(w, http.StatusCreated, item)
	}))

	// GET /api/v1/items/:id — fetch one item; 404 via error envelope if absent.
	mustRegister(v1.GET("/items/:id", func(w http.ResponseWriter, req *http.Request) {
		id, err := router.PathParam(req, "id")
		if err != nil {
			httperrors.WriteError(w, req, err)
			return
		}
		item := store.Get(id)
		if item == nil {
			httperrors.WriteError(w, req, httperrors.ErrNotFound)
			return
		}
		router.JSON(w, http.StatusOK, item)
	}))

	// DELETE /api/v1/items/:id — 204 on success, 404 if absent.
	mustRegister(v1.DELETE("/items/:id", func(w http.ResponseWriter, req *http.Request) {
		id, err := router.PathParam(req, "id")
		if err != nil {
			httperrors.WriteError(w, req, err)
			return
		}
		if !store.Delete(id) {
			httperrors.WriteError(w, req, httperrors.ErrNotFound)
			return
		}
		router.NoContent(w)
	}))

	// GET /api/v1/metrics — in-process metrics snapshot (ADR-012).
	mustRegister(v1.GET("/metrics", func(w http.ResponseWriter, req *http.Request) {
		type entry struct {
			Method   string           `json:"method"`
			Pattern  string           `json:"pattern"`
			Requests int64            `json:"requests"`
			TotalMs  int64            `json:"total_ms"`
			Statuses map[string]int64 `json:"statuses"`
		}
		snap := rec.Snapshot()
		result := make([]entry, 0, len(snap))
		for k, v := range snap {
			sc := make(map[string]int64, len(v.StatusCodes))
			for code, n := range v.StatusCodes {
				sc[strconv.Itoa(code)] = n
			}
			result = append(result, entry{
				Method:   k.Method,
				Pattern:  k.Pattern,
				Requests: v.Requests,
				TotalMs:  v.TotalMs,
				Statuses: sc,
			})
		}
		router.JSON(w, http.StatusOK, result)
	}))

	// GET /api/v1/boom — deliberate panic to demonstrate Recovery middleware.
	mustRegister(v1.GET("/boom", func(_ http.ResponseWriter, _ *http.Request) {
		panic("deliberate boom — demonstrating Recovery middleware (ADR-007)")
	}))

	return middleware.Chain(
		observability.RequestID(),
		observability.Logger(log),
		rec.Middleware(),
		middleware.Recovery(log, debug),
	)(r)
}

// mustRegister panics when route registration fails. Registration errors are
// programming mistakes (duplicate routes, bad patterns) that must be caught at
// startup, not deferred to request time.
func mustRegister(err error) {
	if err != nil {
		panic(fmt.Sprintf("items: route registration: %v", err))
	}
}
