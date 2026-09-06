# Phase 8 — Validation and Example: Test Plan

## Scope

End-to-end tests for the `examples/items` package, exercising every route in
the suggested surface table through `httptest.NewServer`. These tests verify
that all seven framework components work together: routing with path parameters
and a route group, the middleware chain, typed JSON parsing, structured
responses, the error envelope, DI-resolved service, and observability output.

The `cmd/server/main.go` binary is covered by `go build ./...` (verified to
compile and link) and by the server smoke test described below. It is a
`package main` and does not have its own test file.

## Test file

- `examples/items/items_test.go` (package `items_test` — external/black-box)

## Server wiring under test

```
newServer(t) ─► di.NewContainer() ─► ctr.Register("store", NewStore, Singleton)
              ─► items.NewHandler(ctr, rec, slog.Discard, debug=false)
              ─► httptest.NewServer(handler)
```

Middleware order: `RequestID → Logger → Recorder.Middleware → Recovery → Router`

## Test table

| Test | Route | Behavioral requirement covered |
|---|---|---|
| `TestHealth` | GET /api/v1/health | 200, `Content-Type: application/json`, body `{"status":"ok"}` |
| `TestItems_ListEmpty` | GET /api/v1/items | 200, empty JSON array on a fresh store |
| `TestItems_CreateAndGet` | POST + GET /api/v1/items/:id | 201 with `id` field; subsequent GET returns same `id` |
| `TestItems_Create_EmptyName` | POST /api/v1/items | 400 error envelope with `code:"bad_request"` and non-empty details list |
| `TestItems_GetNotFound` | GET /api/v1/items/nonexistent | 404 error envelope with `code:"not_found"` |
| `TestItems_DeleteAndConfirm` | DELETE /api/v1/items/:id | 204 on existing item; subsequent GET returns 404 |
| `TestItems_DeleteNotFound` | DELETE /api/v1/items/ghost | 404 error envelope |
| `TestItems_ListLimit` | GET /api/v1/items?limit=2 | Returns at most 2 items when 3 exist |
| `TestMetrics_Endpoint` | GET /api/v1/metrics | 200, non-empty JSON array after two prior health requests |
| `TestBoom_PanicIsRecovered` | GET /api/v1/boom | 500 error envelope (panic recovered by Recovery middleware) |
| `TestRequestID_HeaderEchoed` | GET /api/v1/health | `X-Request-ID` response header is non-empty |
| `TestRouteGroup_PathParams` | POST + GET /api/v1/items/:id | Path parameter `:id` is correctly extracted inside `/api/v1` route group |

## Server smoke test (manual)

```bash
source ./environment.sh
go run ./cmd/server &
SERVER_PID=$!

curl http://localhost:8080/api/v1/health
# → {"status":"ok"}

curl -X POST http://localhost:8080/api/v1/items \
  -H 'Content-Type: application/json' -d '{"name":"widget"}'
# → {"id":"1","name":"widget"}

curl http://localhost:8080/api/v1/items/1
# → {"id":"1","name":"widget"}

curl -X DELETE http://localhost:8080/api/v1/items/1
# → HTTP 204

curl http://localhost:8080/api/v1/boom
# → {"error":{"code":"internal_error",...}}

kill $SERVER_PID
# → server logs "shutting down gracefully" then "server stopped"
```

## Key design decisions

### cmd/server imports examples/items

`cmd/server/main.go` imports `examples/items` to obtain `NewHandler` and
`NewStore`. Both are in Layer 4 (Application). This is accepted as the
composition-root pattern: `cmd/` acts as the final wiring point and may use
application-level packages from `examples/`.

### DI key as exported constant

`items.StoreKey = "store"` is exported so tests and `cmd/server/main.go` agree
on the registration key without magic strings.

### mustRegister panic on startup

Route registration errors (duplicate pattern, bad prefix) are programming
mistakes. `mustRegister` panics immediately at construction time rather than
returning an error that a caller might silently ignore.

### List returns nil-safe slice

`Store.List` returns `make([]*Item, 0, ...)`, ensuring the JSON encoder writes
`[]` rather than `null` when the store is empty.

### Limit via QueryValues, not QueryParam

`limit` is optional; `router.QueryParam` returns an error on absent keys.
`router.QueryValues(req).Get("limit")` returns `""` when absent, keeping
handler code free of error-case boilerplate for optional parameters.

## Coverage

- `examples/items`: 89.4% statement coverage (from `go test -race -cover`)
- All prior packages unchanged; their coverage figures from earlier phases hold.

## Commands run to verify

```
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All checks clean; all tests passed with no race conditions detected.
