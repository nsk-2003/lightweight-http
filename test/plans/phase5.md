# Phase 5 — Centralized Error Handling: Test Plan

## Scope

Tests for the extended `pkg/errors` package (JSON envelope, sentinels, Handler,
request-ID helpers), updated `pkg/middleware` (Recovery produces JSON envelope),
and updated `pkg/router` (404 and 405 use the envelope). All tests use
`net/http/httptest`; no real ports are bound.

## Test files

- `pkg/errors/errors_test.go` (package `errors_test` — black-box)
- `pkg/middleware/middleware_test.go` (package `middleware_test` — black-box)
- `pkg/router/router_test.go` (package `router_test` — black-box)

## pkg/errors tests (Phase 5 additions)

| Test | Behavioral requirement covered |
|---|---|
| `TestNewCoded_SetsAllFields` | `NewCoded` sets Code, ErrCode, and Message |
| `TestSentinelErrors_StatusAndCode` | All nine sentinel vars have correct HTTP status and machine-readable code |
| `TestSentinelErrors_IsWorksThrough2Wraps` | `errors.Is` finds a sentinel through two `fmt.Errorf` wraps |
| `TestSentinelErrors_AsWorksForHTTPError` | `errors.As` extracts `*HTTPError` from a wrapped error |
| `TestHTTPError_WithDetails_ReturnsCopy` | `WithDetails` returns a new pointer; original is unchanged |
| `TestHTTPError_WithDetails_SentinelUnchanged` | Calling `WithDetails` on a sentinel does not mutate the sentinel |
| `TestWithRequestID_RoundTrip` | `WithRequestID` + `RequestIDFromContext` round-trips correctly |
| `TestRequestIDFromContext_MissingReturnsEmpty` | Empty context returns `""` for request ID |
| `TestHandler_WritesCorrectStatus` | `Handler.ServeError` writes the HTTP status from the error |
| `TestHandler_WritesJSONContentType` | `Handler.ServeError` sets `Content-Type: application/json` |
| `TestHandler_EnvelopeHasSingleTopLevelKey` | Response JSON has exactly one top-level key: `"error"` |
| `TestHandler_EnvelopeFields_CodeMessageStatus` | Inner `error` object carries `code`, `message`, and `status` |
| `TestHandler_EmptyDetails_Omitted` | `details` key is absent when no details are set |
| `TestHandler_WithDetails_Included` | `details` array is present and correct when set via `WithDetails` |
| `TestHandler_PlainError_Returns500Generic` | Plain `error` (non-HTTPError) → 500 with generic message; original not exposed |
| `TestHandler_WithRequestID_Included` | Request ID from context appears as `request_id` in the envelope |
| `TestHandler_WithoutRequestID_Omitted` | `request_id` key is absent when context has no ID |
| `TestHandler_ProductionMode_NoCauseText` | Production mode: cause text never appears in response body |
| `TestHandler_NeverEmitsGoroutineOrRepoPath` | Response body never contains `"goroutine"` or the repository path, in both production and debug mode |
| `TestWriteError_WritesJSONEnvelope` | `WriteError` convenience function produces correct status, Content-Type, and JSON shape |

## pkg/middleware tests (Phase 5 changes)

| Test | Behavioral requirement covered |
|---|---|
| `TestRecovery_PanicConvertsTo500` | (updated) Recovery still returns 500; log still contains "panic recovered" |
| `TestRecovery_ErrAbortHandler_Repanics` | (updated) `http.ErrAbortHandler` is still re-panicked |
| `TestRecovery_AlreadyWroteHeader_LogsAndDoesNotWrite500` | (updated) Committed response is not overwritten |
| `TestRecovery_PanicBody_IsJSONEnvelope` | NEW: panic response body is valid JSON with top-level `"error"` key |
| `TestRecovery_PanicBody_NoGoroutineOrRepoPath` | NEW: panic response body does not contain `"goroutine"`, repo path, or the panic value string |

All existing Phase 3 chain tests (execution order, short-circuit, context propagation,
header forwarding, concurrent no-race) are unchanged and continue to pass.

`Recovery` signature changed from `Recovery(log)` to `Recovery(log, debug bool)` to support
optional stack-trace logging in debug mode. All test call sites updated accordingly.

## pkg/router tests (Phase 5 changes)

| Test | Behavioral requirement covered |
|---|---|
| `TestRouter_NotFound_IsJSONEnvelope` | NEW: 404 response is `application/json` with `"error"` top-level key |
| `TestRouter_MethodNotAllowed_IsJSONEnvelope` | NEW: 405 response is `application/json` with `"error"` key; `Allow` header still present |

## Key design decisions

### Central error handler
`errors.Handler` is the single point of error serialization. It holds `*slog.Logger`
and `debug bool`, both set at construction time — never derived from the request
(ADR-008). `WriteError` is a zero-configuration convenience wrapper that uses
`slog.Default()` and production mode.

### Wire format is fixed
The envelope `{"error":{"code","message","status","request_id","details"}}` is the
only shape ever emitted. `details` and `request_id` are `omitempty` and only appear
when populated. No other top-level keys are permitted (ADR-006).

### Cause isolation
The wrapped cause travels in `HTTPError.cause` (unexported). It is logged by the
handler but the JSON encoder never sees it — only `Message` reaches the client.

### Sentinel immutability
`WithDetails` copies the receiver struct before appending details, so sentinel vars
remain unmodified across calls. Package-level sentinel vars (`ErrNotFound`, etc.) are
safe to share across goroutines.

### Recovery rewired (ADR-007)
`middleware.Recovery` now imports `pkg/errors` and calls `errors.NewHandler.ServeError`
instead of `http.Error`. The panic value is logged before `ServeError` is called, so
no double-logging occurs (the bare `New(500, …)` passed in has no wrapped cause).

### Stack traces stay in logs
`runtime/debug.Stack()` is captured inside the `defer func()` in `Recovery` (giving
the correct goroutine context) and written only to `slog`. It is never encoded into the
response body, satisfying ADR-008 and the "no goroutine substring in body" test.

## Coverage

- `pkg/errors`: 83.6% statement coverage
- `pkg/middleware`: 81.5% statement coverage
- `pkg/router`: 88.1% statement coverage

## Commands run to verify

```
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All checks clean; all tests passed with no race conditions detected.
