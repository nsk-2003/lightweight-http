# lightweight-http

A minimal, production-ready HTTP framework in Go using **only the Go standard library**.
No third-party dependencies — `go list -m all` prints exactly one module.

It demonstrates the core abstractions every HTTP framework needs: routing, middleware,
request/response handling, centralized error handling, dependency injection, and observability.

## Quick Start

```bash
# Build everything
source ./environment.sh && go build ./...

# Run the server (default port 8080)
go run ./cmd/server

# Or run the standalone example
go run ./examples/items/cmd
```

### curl Examples

```bash
# Health check
curl -s http://localhost:8080/api/v1/health
# {"status":"ok"}

# Create an item
curl -s -X POST -H 'Content-Type: application/json' \
     -d '{"name":"gadget","note":"first item"}' \
     http://localhost:8080/api/v1/items
# {"id":"1","name":"gadget","note":"first item"}

# List items (supports ?limit=N)
curl -s http://localhost:8080/api/v1/items
# [{"id":"1","name":"gadget","note":"first item"}]

# Get item by ID (path parameter)
curl -s http://localhost:8080/api/v1/items/1
# {"id":"1","name":"gadget","note":"first item"}

# Validation error — name is required (422 with field details)
curl -s -X POST -H 'Content-Type: application/json' \
     -d '{"note":"missing name"}' \
     http://localhost:8080/api/v1/items
# {"error":{"code":"unprocessable_entity","message":"validation failed","status":422,"request_id":"<id>","details":[{"field":"name","reason":"name is required"}]}}

# Delete an item (returns 204 No Content, empty body)
curl -o /dev/null -w "%{http_code}\n" -X DELETE http://localhost:8080/api/v1/items/1
# 204

# Metrics snapshot
curl -s http://localhost:8080/api/v1/metrics
# [{"route":"/api/v1/items/:id","method":"GET","request_count":1,"total_latency_ms":0.015,"status_codes":{"200":1}},...]

# Deliberate panic — Recovery middleware converts it to 500
curl -s http://localhost:8080/api/v1/boom
# {"error":{"code":"internal_error","message":"an internal error occurred","status":500,"request_id":"<id>"}}
```

### Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `PORT` | `8080` | TCP port to listen on |
| `LOG_LEVEL` | `info` | Structured log level (`debug`, `info`, `warn`, `error`) |
| `DEBUG` | `false` | Enables stack traces in error logs (never in client response) |

## Architecture

### Package Layers

| Layer | Packages | May depend on |
|---|---|---|
| 4 — Application | `cmd/`, `examples/` | all layers |
| 3 — Composition | `pkg/middleware`, `pkg/observability` | layers 1–2 |
| 2 — Core | `pkg/router`, `pkg/di` | layer 1 |
| 1 — Foundation | `pkg/errors` | standard library only |

**Layering rule:** dependencies point downward only. `pkg/router` does not know `pkg/middleware`
exists. `pkg/di` is never imported by framework packages — only by `cmd/` and `examples/`.
Nothing under `pkg/` imports `cmd/` or `examples/`.

### Package Responsibilities

| Package | Responsibility |
|---|---|
| `pkg/errors` | `HTTPError` type, HTTP-status mapping, one JSON wire format for every error |
| `pkg/router` | Trie-based method+path dispatch, path parameters, route groups, typed request parsing, structured response writer |
| `pkg/middleware` | `Middleware` type, `Chain` composer, built-in Recovery middleware |
| `pkg/di` | Singleton/scoped DI container with circular-dependency detection |
| `pkg/observability` | RequestID, structured logging, in-process metrics — each as composable middleware |

## Package Usage

### Router

```go
ro := router.New()
ro.Use(middleware1, middleware2) // global chain, outermost first

v1 := ro.Group("/api/v1")
v1.GET("/items/:id", func(w http.ResponseWriter, r *http.Request) {
    id := router.PathParam(r, "id")
    // ...
})
http.ListenAndServe(":8080", ro)
```

### Middleware

```go
// Compose a chain
chain := middleware.New(
    observability.RequestID(),
    observability.LoggingMiddleware(logger),
    middleware.Recovery(logger),
)
handler := chain.Then(myHandler)

// Or register directly on the router
ro.Use(observability.RequestID(), middleware.Recovery(logger))
```

### Request Parsing

```go
var body MyRequest
if err := router.ParseJSON(r, &body); err != nil {
    httperr.Handle(w, r, err, nil, false) // 400/413/415 automatically
    return
}
limit, err := router.QueryInt(r, "limit") // 400 if absent or non-integer
```

### Responses

```go
router.WriteJSON(w, r, http.StatusOK, myValue)   // content-negotiated JSON
router.WriteText(w, r, http.StatusOK, "ok")       // content-negotiated text
router.Respond(w, r, http.StatusOK, myValue)      // negotiated (JSON or text)
router.NoContent(w)                                // 204
```

### Errors

```go
// Constructors
httperr.NotFound("item not found")
httperr.BadRequest("invalid payload")
httperr.UnprocessableEntity("validation failed",
    httperr.Detail{Field: "name", Reason: "name is required"},
)
httperr.Internal("db unavailable", err) // cause logged, never sent to client

// Central handler — converts any error to the JSON envelope
httperr.Handle(w, r, err, logger, debugMode)
```

### Dependency Injection

```go
c := di.New()
c.Register("myService", func(r *di.Resolver) (any, error) {
    return NewMyService(), nil
}, di.Singleton)

// Resolve in a handler
v, err := c.Resolve("myService")
svc := v.(*MyService)
```

### Observability

```go
collector := observability.NewInProcessCollector()

ro.Use(
    observability.RequestID(),           // assigns X-Request-ID, echoes on response
    observability.LoggingMiddleware(l),  // structured per-request log
    observability.MetricsMiddleware(c),  // request count, latency, status codes
    middleware.Recovery(l),              // panic → 500, never propagated
)

// Expose metrics
snap := collector.Snapshot() // map[MetricKey]MetricSnapshot
```

## Design Decisions

All binding decisions are recorded in [`docs/design/ADR.md`](docs/design/ADR.md). A condensed list:

- **ADR-001** Standard library only — no third-party modules.
- **ADR-003** Router implements `http.Handler`; compatible with `net/http`.
- **ADR-004** Trie/segment matcher — O(path segments), no regex, no backtracking.
- **ADR-005** Request-scoped values travel in `context.Context` via unexported key types.
- **ADR-006** One error type (`HTTPError`), one JSON wire format for every error response.
- **ADR-007** Panics are converted by Recovery, never propagated to `net/http`.
- **ADR-008** Stack traces are opt-in (`DEBUG=true`) and never appear in client responses.
- **ADR-009** Middleware executes in registration order; first registered = outermost.
- **ADR-010** DI is explicit; reflection is minimized to interface key derivation.
- **ADR-011** Structured logging via `log/slog`; sensitive headers are never logged.
- **ADR-012** Metrics are in-process and pull-based; keyed by route pattern, not raw path.

## Project Layout

```
.
├── cmd/
│   └── server/
│       └── main.go          # Production server entry point
├── examples/
│   └── items/
│       ├── store.go         # In-memory item store (library package)
│       ├── handlers.go      # Route registration via RegisterRoutes
│       └── cmd/
│           └── main.go      # Standalone runnable example
├── pkg/
│   ├── errors/              # HTTPError, Handle, JSON envelope
│   ├── router/              # Router, Group, trie, request parsing, response writing
│   ├── middleware/          # Middleware type, Chain, Recovery
│   ├── di/                  # DI container, Singleton/Scoped lifecycles
│   └── observability/       # RequestID, LoggingMiddleware, MetricsMiddleware
├── test/
│   ├── unit/
│   │   ├── di_layering_test.go   # Layering enforcement
│   │   └── e2e_test.go           # End-to-end route table test
│   ├── plans/               # Test plans per phase
│   └── testdata/            # Shared fixtures
├── docs/
│   ├── design/
│   │   ├── ARCHITECTURE.md
│   │   ├── ADR.md
│   │   ├── sourcemap.md
│   │   └── packagedesign.md
│   └── specifications/      # Phase specs
├── go.mod
└── environment.sh
```

## Testing

```bash
# Load environment first
source ./environment.sh

# All tests with race detector and coverage
go test ./... -race -cover

# Specific package
go test ./pkg/router/... -race -v

# Build gate
go build ./... && go vet ./... && gofmt -l .

# Verify standard-library constraint
go list -m all   # must print exactly one module
```

Package-local tests live beside the code as `*_test.go`. Black-box and cross-package tests
live in `test/unit/`. Test plans are in `test/plans/`.
