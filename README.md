# lightweight-http

A minimal, production-ready HTTP framework written in **Go standard library only** — no
third-party dependencies. It demonstrates the core abstractions every HTTP framework needs:
routing, middleware, request/response handling, centralized errors, dependency injection,
and observability.

`go list -m all` prints exactly one module.

---

## Quick Start

### Build and run

```bash
source ./environment.sh
go build ./...
go run ./cmd/server
```

Environment variables (all optional, safe defaults shown):

| Variable | Default | Meaning |
|---|---|---|
| `LWHTTP_PORT` | `8080` | TCP port the server listens on |
| `LWHTTP_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |
| `LWHTTP_DEBUG` | _(unset)_ | Set to `true` to enable stack-trace logging |

### Example curl session

```bash
# Health check
$ curl http://localhost:8080/api/v1/health
{"status":"ok"}

# Create an item
$ curl -X POST http://localhost:8080/api/v1/items \
    -H 'Content-Type: application/json' \
    -d '{"name":"widget"}'
{"id":"1","name":"widget"}

# List items
$ curl http://localhost:8080/api/v1/items
[{"id":"1","name":"widget"}]

# Fetch by ID
$ curl http://localhost:8080/api/v1/items/1
{"id":"1","name":"widget"}

# 404 through the error envelope
$ curl http://localhost:8080/api/v1/items/missing
{"error":{"code":"not_found","message":"Not Found","status":404,"request_id":"9d6d79607dd5afd837fbdcc0c498f456"}}

# Delete
$ curl -i -X DELETE http://localhost:8080/api/v1/items/1
HTTP/1.1 204 No Content

# Validation error — empty name
$ curl -X POST http://localhost:8080/api/v1/items \
    -H 'Content-Type: application/json' \
    -d '{"name":""}'
{"error":{"code":"bad_request","message":"Bad Request","status":400,"request_id":"03b1cbed253ac7e235515813ddc2ef70","details":[{"field":"name","reason":"name is required"}]}}

# Panic recovery — returns 500 error envelope
$ curl http://localhost:8080/api/v1/boom
{"error":{"code":"internal_error","message":"Internal Server Error","status":500,"request_id":"7360486f2f17b9c1e15b41b7228e4a9a"}}

# In-process metrics snapshot
$ curl http://localhost:8080/api/v1/metrics
[{"method":"GET","pattern":"/api/v1/health","requests":1,"total_ms":0,"statuses":{"200":1}}]
```

---

## Architecture

### Layer table

| Layer | Packages | May depend on |
|---|---|---|
| 4 — Application | `cmd/server`, `examples/items` | all layers |
| 3 — Composition | `pkg/middleware`, `pkg/observability` | layers 1–2 |
| 2 — Core | `pkg/router`, `pkg/di` | layer 1 |
| 1 — Foundation | `pkg/errors` | standard library only |

Dependencies point **downward only**. `pkg/errors` is the single package that every other
package may import; it imports nothing from `pkg/`.

### Package responsibilities

| Package | Responsibility |
|---|---|
| `pkg/errors` | `HTTPError` type, HTTP-status mapping, one JSON envelope, sentinel errors, optional stack traces, request-ID context helpers |
| `pkg/router` | Trie-based method+path matching, path parameters, route groups, typed request parsing, structured response writer |
| `pkg/middleware` | `Middleware` type, `Chain` composer, `Recovery` (panic → 500) |
| `pkg/di` | Singleton/scoped lifecycle container, circular-dependency detection, context helpers |
| `pkg/observability` | `RequestID`, `Logger`, `Recorder` (metrics) middleware; all composable, all pull-based |
| `examples/items` | In-memory items API that exercises every framework package |
| `cmd/server` | Composition root: wires DI, middleware, and handler; starts `http.Server`; graceful shutdown |

---

## Usage

### Router

```go
r := router.New()
r.GET("/users/:id", func(w http.ResponseWriter, req *http.Request) {
    id, err := router.PathParam(req, "id")
    if err != nil {
        httperrors.WriteError(w, req, err)
        return
    }
    router.JSON(w, http.StatusOK, map[string]string{"id": id})
})

// Route group — prefixes every route with /api/v1
v1 := r.Group("/api/v1")
v1.GET("/health", healthHandler)
v1.POST("/items", createHandler)
```

### Middleware

```go
chain := middleware.Chain(
    observability.RequestID(),   // outermost
    observability.Logger(log),
    rec.Middleware(),
    middleware.Recovery(log, debug),
)
http.ListenAndServe(":8080", chain(r))
```

### Request parsing

```go
// JSON body
var input struct { Name string `json:"name"` }
if err := router.BindJSON(req, &input); err != nil {
    httperrors.WriteError(w, req, err)
    return
}

// Optional query param
limit := 0
if s := router.QueryValues(req).Get("limit"); s != "" {
    limit, _ = strconv.Atoi(s)
}

// Path param
id, err := router.PathParam(req, "id")
```

### Response helpers

```go
router.JSON(w, http.StatusOK, payload)      // application/json
router.Text(w, http.StatusOK, "plain text") // text/plain
router.NoContent(w)                          // 204
router.Respond(w, req, http.StatusOK, data) // content-negotiated
```

### Errors

```go
// Sentinel
httperrors.WriteError(w, req, httperrors.ErrNotFound)

// With details (validation)
httperrors.WriteError(w, req, httperrors.ErrBadRequest.WithDetails(
    httperrors.Detail{Field: "name", Reason: "name is required"},
))

// Wrapping — cause is logged, never sent to the client
return httperrors.Wrap(http.StatusInternalServerError, "db unavailable", err)
```

Wire the centralized error handler for consistent logging:

```go
eh := httperrors.NewHandler(log, debug)
eh.ServeError(w, req, err)
```

### Dependency injection

```go
ctr := di.NewContainer()
ctr.Register("store", func(r di.Resolver) (any, error) {
    return NewStore(), nil
}, di.Singleton)

svc, err := ctr.Resolve("store")
store := svc.(*Store)

// Scoped lifecycle (one per request)
scope := ctr.NewScope()
defer scope.Dispose()
svc, err = scope.Resolve("repo")
```

### Observability

```go
rec := &observability.Recorder{} // zero value ready to use

// Snapshot (pull-based, no exporter)
snap := rec.Snapshot()
for key, m := range snap {
    fmt.Printf("%s %s: %d requests\n", key.Method, key.Pattern, m.Requests)
}
```

Middleware ordering (outermost → innermost):

```
RequestID → Logger → Metrics → Recovery → Handler
```

---

## Design Decisions

Full rationale is in [`docs/design/ADR.md`](docs/design/ADR.md). Condensed:

| ADR | Decision |
|---|---|
| ADR-001 | Standard library only — no third-party modules, ever |
| ADR-002 | Tests beside the code as `*_test.go`; `test/unit/` for black-box tests |
| ADR-003 | Router implements `http.Handler`; handlers are adaptable to `http.HandlerFunc` |
| ADR-004 | Trie matcher, static-wins-over-param, O(segments), no regex, no backtracking |
| ADR-005 | Request-scoped values in `context.Context` via unexported key types |
| ADR-006 | One error type (`HTTPError`), one JSON envelope, all paths route through it |
| ADR-007 | Panics recovered and converted to 500; never propagated to the net/http layer |
| ADR-008 | Stack traces opt-in, logged only; never in client response body |
| ADR-009 | Middleware runs in registration order; first registered is outermost |
| ADR-010 | DI resolution is explicit; circular dependencies are returned as errors |
| ADR-011 | Structured logging via `log/slog`; secrets never logged |
| ADR-012 | Metrics in-process, pull-based, keyed by route pattern not concrete path |

---

## Project Layout

```
.
├── cmd/
│   └── server/
│       └── main.go            # composition root — wires DI, middleware, handler
├── examples/
│   └── items/
│       ├── items.go           # in-memory items API — exercises every framework pkg
│       └── items_test.go      # end-to-end tests via httptest.NewServer
├── pkg/
│   ├── di/
│   │   ├── container.go
│   │   └── container_test.go
│   ├── errors/
│   │   ├── errors.go
│   │   └── errors_test.go
│   ├── middleware/
│   │   ├── middleware.go
│   │   └── middleware_test.go
│   ├── observability/
│   │   ├── observability.go
│   │   └── observability_test.go
│   └── router/
│       ├── context.go
│       ├── request.go
│       ├── request_test.go
│       ├── response.go
│       ├── response_test.go
│       ├── router.go
│       ├── router_test.go
│       └── trie.go
├── docs/
│   ├── design/
│   │   ├── ADR.md
│   │   ├── ARCHITECTURE.md
│   │   ├── packagedesign.md
│   │   └── sourcemap.md
│   └── specifications/
│       └── phase{1..8}.md
├── test/
│   └── plans/
│       └── phase{1..8}.md
├── doc.go
├── go.mod
├── environment.sh
└── AGENTS.md
```

---

## Testing

```bash
source ./environment.sh

# Build
go build ./...

# Static checks
go vet ./...
gofmt -l .

# Tests with race detector and coverage
go test ./... -race -cover

# Verify no third-party dependencies
go list -m all
```

All tests are table-driven, use `net/http/httptest` (no real ports bound in unit tests), and
were written before their implementation (TDD). Each phase's test plan is in `test/plans/`.
