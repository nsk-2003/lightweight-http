# Test Plan — Phase 8: Validation and Example

## Scope

Phase 8 wires all framework components into a runnable server and proves correctness with
an end-to-end test suite. No new framework packages are introduced; all new code is at the
application layer (`cmd/`, `examples/`, `test/unit/`).

## Unit Tests

No new unit tests are required for Phase 8 — all business logic added in this phase lives
in the application layer (`examples/items`), which is validated by the e2e tests below.
Framework package unit tests from Phases 2–7 continue to pass unchanged.

## End-to-End Tests (`test/unit/e2e_test.go`)

Each test starts a fresh `httptest.NewServer` with the full middleware chain and route table.
The `newTestServer` helper registers the `ItemStore` singleton, wires
`RequestID → MetricsMiddleware → Recovery`, and mounts routes under `/api/v1` via
`items.RegisterRoutes`.

| Test | Route(s) exercised | Assertions |
|---|---|---|
| `TestE2EHealth` | `GET /api/v1/health` | 200; `{"status":"ok"}` |
| `TestE2EItemsCRUD` | `POST`, `GET /items`, `GET /items/:id`, `DELETE /items/:id` | 201 create; 200 list (len=1); 200 get by ID; 204 delete; 404 get after delete |
| `TestE2EItemsValidation` | `POST /api/v1/items` (missing name) | 422; body contains `"name"` field |
| `TestE2EItemsLimitQuery` | `GET /api/v1/items?limit=1` | Creates 2 items; list with limit=1 returns exactly 1 |
| `TestE2EMetrics` | `GET /api/v1/metrics` | 200; non-empty JSON array after a prior request |
| `TestE2EBoomRecovers` | `GET /api/v1/boom` | 500; body contains `"internal_error"` |
| `TestE2ENotFound` | `GET /api/v1/nope` | 404 |
| `TestE2ERequestIDHeader` | `GET /api/v1/health` | X-Request-ID header present in response |

## Acceptance Criteria Checklist

- [ ] `go build ./...` succeeds with no output.
- [ ] `go vet ./...` reports nothing.
- [ ] `gofmt -l .` prints nothing.
- [ ] `go test ./... -race -cover` passes.
- [ ] `go list -m all` shows only the main module.
- [ ] Server starts, serves every route in the table, and shuts down cleanly on SIGINT.
- [ ] Each `curl` example in the README was actually run and its output matches.
- [ ] `docs/design/sourcemap.md` lists every source file.
- [ ] `docs/design/packagedesign.md` matches the real import graph.

## Manual Smoke Test

```bash
source ./environment.sh && go run ./cmd/server &
SERVER_PID=$!

curl -s http://localhost:8080/api/v1/health
# {"status":"ok"}

curl -s -X POST -H 'Content-Type: application/json' \
     -d '{"name":"gadget"}' http://localhost:8080/api/v1/items
# {"id":"1","name":"gadget"}

curl -s http://localhost:8080/api/v1/items/1
# {"id":"1","name":"gadget"}

curl -s http://localhost:8080/api/v1/boom
# {"error":{"code":"internal_error","message":"an internal error occurred","status":500,...}}

kill $SERVER_PID
```
