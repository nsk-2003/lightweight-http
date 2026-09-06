# Phase 7 — Observability: Test Plan

## Scope

Tests for the `pkg/observability` package: `RequestID`, `Logger`, `Recorder.Middleware`,
and `Recorder.Snapshot`. All tests exercise the public API (black-box) from the
`observability_test` package. No real ports are bound; `net/http/httptest` only.

Supporting changes tested indirectly:
- `pkg/router`: `RoutePatternHolder`, `WithRoutePatternHolder`, `RoutePattern`,
  `RoutePatternHolderFromContext`, `setRoutePattern` (called from `router.ServeHTTP`).
- `pkg/router/trie.go`: `trieNode.pattern` field, set at registration, returned in `matchResult`.

## Test file

- `pkg/observability/observability_test.go` (package `observability_test` — black-box)

## In-memory slog handler

Log assertions use a `memHandler` that implements `slog.Handler` and captures records
into a `[]memRecord` protected by a mutex. Each record stores `Level`, `Msg`, and a
`map[string]slog.Value` of all attributes. This avoids parsing stdout and is race-safe.

## Test table

| Test | Behavioral requirement covered |
|---|---|
| `TestRequestID_GeneratesWhenAbsent` | No X-Request-ID header → ID generated, stored in context, echoed in response header |
| `TestRequestID_AdoptsValidIncoming` | Valid X-Request-ID header → adopted unchanged in context and response header |
| `TestRequestID_ReplacesOverLongID` | X-Request-ID >128 chars → replaced with generated ID; original never echoed |
| `TestRequestID_ReplacesNonPrintableID` | X-Request-ID containing non-printable ASCII → replaced with generated ID |
| `TestLogger_StandardFields` | Every required log field (request_id, method, route, path, status, duration_ms, bytes, remote_addr) present with correct values |
| `TestLogger_NoAuthorizationHeader` | Request with Authorization header → authorization token value absent from every log field |
| `TestMetrics_CountsRequests` | N requests to same route → Snapshot shows N in Requests counter |
| `TestMetrics_StatusCodes` | Mix of 200 and 500 responses → each status tallied separately in StatusCodes |
| `TestMetrics_AggregatesRoutePattern` | Three concrete paths matching `/users/:id` → all aggregated under one `MetricKey{Pattern: "/users/:id"}` |
| `TestMetrics_RecordsLatency` | Single request → TotalMs is non-negative |
| `TestMetrics_PanicLatencyAndStatusRecorded` | Handler panics; Metrics wraps Recovery → 500 counted, TotalMs non-negative after panic |
| `TestMetrics_ConcurrentRequests` | 50 goroutines × 10 requests → exact count of 500; `go test -race` clean |

## Middleware ordering documented

Recommended composition order (outermost → innermost):

```
RequestID → Logger → Metrics → Recovery → Handler
```

- **RequestID** first: assigns the ID that all downstream layers log and echo.
- **Logger** second: installs the `RoutePatternHolder` so downstream router sets it;
  logs after everything below (including Metrics) returns, capturing final status/bytes.
- **Metrics** third: wraps Recovery so it sees the 500 written by Recovery on panic.
- **Recovery** fourth: converts panics to 500 responses before Metrics reads the status.

## Key design decisions

### RoutePatternHolder — mutable pointer in context

Metrics and Logger need the route pattern after `ServeHTTP` returns, but the router writes
the pattern into the *inner* request context (via `req.WithContext`). Solution: Logger (or
Metrics if used standalone) installs a `*router.RoutePatternHolder` in the context before
calling `next.ServeHTTP`. Because it is a pointer, the router's `setRoutePattern` can mutate
it through the context chain; the outer middleware reads `holder.Pattern` after dispatch
without needing a new context lookup. No context write happens after the handler completes.

### Route pattern stored in trie node

`trieNode.pattern` is set once, at registration time, to the canonical pattern string
(e.g. `"/users/:id"`). `matchResult` carries the pattern; the router calls
`setRoutePattern(ctx, result.pattern)` only when a handler is found. 404/405 paths leave
the holder at its zero value (`""`), which aggregates under an empty pattern key.

### Metrics uses sync.Mutex

A single `sync.Mutex` guards the `map[MetricKey]*routeEntry`. The map is write-only on the
critical path (one entry per unique key, grown lazily) and read only during `Snapshot`.
The race detector confirmed correctness under 50-goroutine concurrency.

### Request ID validation

Only visible ASCII non-space characters (0x21 '!' through 0x7E '~') are accepted; any
code point outside this range (including spaces, tabs, and null bytes) causes replacement.
Maximum length is 128 bytes. Generated IDs are 32-character lowercase hex strings (16
bytes from `crypto/rand`).

### Authorization header redaction

The Logger never reads request headers. The standard log field set (`request_id`, `method`,
`route`, `path`, `status`, `duration_ms`, `bytes`, `remote_addr`) contains no header value.
Redaction is structural, not a filter: there is no code path that could log headers.

## Coverage

- `pkg/observability`: 94.2% statement coverage (from `go test -race -cover`)

## Commands run to verify

```
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All checks clean; all tests passed with no race conditions detected.
