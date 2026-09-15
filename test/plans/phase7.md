# Phase 7 Test Plan — Observability

## Scope
Request ID middleware, in-process metrics collector and middleware, structured logging
middleware, and the route-pattern context propagation added to the router.

## Test Files
- `pkg/router/router_test.go` — added `TestRoutePattern`
- `pkg/observability/requestid_test.go`
- `pkg/observability/metrics_test.go`
- `pkg/observability/logging_test.go`

## Router: Route Pattern in Context

| Test | What it verifies |
|---|---|
| `TestRoutePattern/static` | Static route `/health` → `RoutePattern` returns `"/health"` |
| `TestRoutePattern/with param` | Param route `/users/:id` → returns `"/users/:id"` for any concrete path |
| `TestRoutePattern/root` | Root pattern `"/"` → returns `"/"` |
| `TestRoutePattern/multi param` | Multiple params → full pattern returned |

## Request ID Middleware

| Test | What it verifies |
|---|---|
| `TestRequestIDGenerated` | No incoming header → generated ID in context and response header |
| `TestRequestIDAdopted` | Valid incoming ID → adopted unchanged in context and response header |
| `TestRequestIDOverLong` | 200-char ID → replaced with generated ID |
| `TestRequestIDInvalidChars` | Space/bang/slash/null → each replaced with generated ID |
| `TestRequestIDEmpty` | Empty header value → generated ID |
| `TestRequestIDUnique` | 10 consecutive requests → 10 distinct IDs |

## Metrics Middleware

| Test | What it verifies |
|---|---|
| `TestMetricsAccurateCount` | 3 requests → RequestCount=3, StatusCodes[200]=3 |
| `TestMetricsPatternAggregation` | `/users/1`, `/users/2`, `/users/3` → all counted under `"/users/:id"` |
| `TestMetricsStatusCodesSeparate` | 200 and 500 routes tallied under separate status keys |
| `TestMetricsConcurrent` | 50 goroutines × 20 requests → exact total under `-race` |
| `TestMetricsLatencyRecordedOnPanic` | Handler panics → metrics middleware (wrapping recovery) still records count and 500 status |
| `TestMetricsSnapshotIsCopy` | Mutating the returned snapshot does not affect the collector |

## Logging Middleware

| Test | What it verifies |
|---|---|
| `TestLoggingStandardFields` | All required fields present: request_id, method, route, path, status, duration_ms, bytes, remote_addr |
| `TestLoggingAuthorizationHeaderAbsent` | Request with Authorization header → no log value contains the token |
| `TestLoggingCookieHeaderAbsent` | Request with Cookie header → no log value contains the cookie |
| `TestLoggingErrorFieldIncluded` | `SetLogError` called → `error` field present in log record |
| `TestLoggingErrorFieldOmittedWhenNil` | No error set → `error` field absent from log record |
| `TestLoggingRoutePatternFromRouter` | Route `/users/:id` → `route` field is pattern, `path` is concrete path |
| `TestLoggingNilLoggerFallsBack` | Nil logger → falls back to `slog.Default()` without panic |

## Middleware Ordering

Recommended registration order (outermost to innermost):
1. `RequestID()` — assigns/validates request ID, echoes on response
2. `LoggingMiddleware(l)` — logs after inner layers complete
3. `MetricsMiddleware(c)` — records latency and status after recovery runs
4. `middleware.Recovery(l)` — converts panics into 500 responses

This ordering ensures:
- The request ID is in context when the logger reads it.
- The metrics middleware wraps Recovery, so panicking handlers still produce a recorded 500.
- The log line is emitted after all processing (including panic recovery) is complete.

## In-Memory slog.Handler Pattern
All logging tests use a `captureHandler` that implements `slog.Handler` and stores records
in a slice. This avoids parsing stdout and allows precise attribute-level assertions.

## Acceptance Criteria Verification
- [x] Request ID generation, adoption, validation, and echo are tested.
- [x] Metrics snapshot returns accurate counts after a known sequence of requests.
- [x] Latency is recorded for panicking requests too.
- [x] Log output is asserted against the standard field set using a test handler.
- [x] A test asserts that a request carrying an Authorization header produces no log line containing its value.
- [x] Middleware ordering guidance is documented in `pkg/observability/doc.go`.
