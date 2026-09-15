# Phase 5 Test Plan — Centralized Error Handling

## Scope

Tests for the `pkg/errors` package (error type, sentinels, constructors, context helpers)
and the central error handler (`Handle`). Also regression coverage for the updated
`pkg/middleware` recovery middleware and `pkg/router` 404/405 responses.

## Test Files

| File | Package | Description |
|---|---|---|
| `pkg/errors/errors_test.go` | `errors` | HTTPError struct and all constructors |
| `pkg/errors/handler_test.go` | `errors` | Handle function, envelope format, leak prevention |
| `pkg/middleware/middleware_test.go` | `middleware` | Recovery regression (now produces JSON envelope) |
| `pkg/router/router_test.go` | `router` | 404/405 regression (now produce JSON envelope) |

## Tested Behaviors

### Error Type (`pkg/errors/errors.go`)

| Test | Assertion |
|---|---|
| `TestNew` | `New(status, code, msg)` sets all three fields |
| `TestError` | `Error()` contains the status code |
| `TestErrorWithCause` | `Error()` includes cause when Cause is non-nil |
| `TestStatusCode` | `StatusCode()` returns Status |
| `TestConstructors` | All nine constructors produce correct Status and Code |
| `TestInternalCause` | `Internal(msg, cause)` wraps cause with status 500 |
| `TestUnprocessableEntityDetails` | `UnprocessableEntity` populates Details slice |
| `TestUnwrap` | `Unwrap()` returns Cause (nil when no Cause) |
| `TestSentinels` | Each sentinel var has the expected Status and Code |
| `TestErrorsIsThroughWraps` | `errors.Is` finds a sentinel through two Cause wraps |
| `TestErrorsAsHTTPError` | `errors.As` finds `*HTTPError` through `fmt.Errorf` wrap |
| `TestRequestIDContextRoundTrip` | `WithRequestID`/`RequestIDFromContext` round-trip |
| `TestRequestIDContextEmpty` | `RequestIDFromContext` returns "" when no ID is set |

### Error Handler (`pkg/errors/handler.go`)

| Test | Assertion |
|---|---|
| `TestHandleNotFound` | Sentinel 404 → status 404, code "not_found", Content-Type JSON |
| `TestHandleEnvelopeShape` | Top-level JSON has exactly one key: "error" |
| `TestHandleProductionNoCause` | Cause text is absent from body in production mode |
| `TestHandleDebugBodyUnchanged` | Debug mode: cause in log, body identical to production |
| `TestHandleRequestID` | Request ID from context appears in envelope |
| `TestHandleNoRequestID` | request_id key is omitted from JSON when absent |
| `TestHandleUnmappedError` | Plain error → 500, generic message, original text not leaked |
| `TestHandleWithDetails` | Validation error details appear in envelope |
| `TestHandleDetailsOmittedWhenEmpty` | "details" absent from JSON when there are no details |
| `TestHandleBodyNeverContainsStackOrPath` | Body never contains "goroutine" or module path |
| `TestHandleBodyNeverContainsStackDebugMode` | Same guarantee holds in debug mode |
| `TestHandleRecoveryIndistinguishable` | Panic-derived 500 has identical envelope to other 500s |
| `TestHandleNilLogger` | Handle works without crashing when logger is nil |

### Recovery Regression (`pkg/middleware/middleware_test.go`)

| Test | Assertion |
|---|---|
| `TestRecoveryPanic` | Panic → 500 HTTP status (envelope format changed; status preserved) |
| `TestRecoveryErrAbortHandler` | `ErrAbortHandler` is re-panicked |
| `TestRecoveryAfterPartialResponse` | Partial-response panic: original status preserved |
| `TestRecoveryNilLogger` | Recovery works with nil logger |

### Router Regression (`pkg/router/router_test.go`)

| Test | Assertion |
|---|---|
| `TestNotFound` | Unknown path → 404 (now JSON envelope) |
| `TestMethodNotAllowed` | Wrong method → 405, Allow header present |
| `TestTrailingSlashMismatch` | Trailing-slash path → 404 |
| `TestRouteGroup` | Group with non-existent path → 404 |

## Definition of Done

- [x] `go build ./...` succeeds with no output.
- [x] `go vet ./...` succeeds with no findings.
- [x] `gofmt -l .` prints nothing.
- [x] `go test ./... -race -cover` passes with no failures.
- [x] Every client-facing error body is verified to not contain "goroutine" or the module path.
- [x] Production-mode responses are verified to contain no cause text.
- [x] Stack capture is confirmed to be off by default (only `runtime/debug.Stack()` in debug mode).
- [x] `errors.Is` and `errors.As` work through wrapping, tested.
- [x] Router 404 and 405 responses use the standard envelope.
- [x] Recovery-produced errors are indistinguishable from other internal errors to a client.
