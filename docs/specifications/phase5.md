# Phase 5 — Centralized Error Handling

**Goal:** one error type and one wire format in `pkg/errors/errors.go`, with HTTP status
mapping, optional stack traces, and panic routing.

## Scope
1. A framework error type carrying an HTTP status, a stable machine-readable code, a
   client-safe message, an optional wrapped cause, and optional structured details.
2. Mapping from error to HTTP status, including sentinel errors for the common cases
   (bad request, unauthorized, forbidden, not found, conflict, unsupported media type,
   unprocessable entity, too many requests, internal).
3. A single JSON error envelope used by every error response.
4. Optional stack trace capture, enabled only in debug mode.
5. A central error handler that the router, middleware, and recovery all route through.

## Out of Scope
- Retry policies, circuit breakers, and rate limiting.
- Localization of error messages.

## Design Constraints
- The type implements `error` and `Unwrap`, so `errors.Is` and `errors.As` work through the
  whole chain (ADR-006).
- Wrapping preserves the cause; the cause is logged but never serialized to the client.
- In production mode the client body contains only the status, code, message, and optional
  safe details. No stack trace, no internal path, no wrapped message, no Go type names
  (ADR-008).
- Debug mode is set explicitly at construction time, never read from a request header or
  query parameter.
- An unmapped error becomes a 500 with a generic message; the real error is logged with the
  request ID.
- Every error response includes the request ID when one is present, so a client report can
  be correlated with a log line.
- The recovery middleware from Phase 3 is rewired to produce a framework error rather than
  its minimal 500 (ADR-007).

## Error Envelope
The exact shape is fixed for the life of the project; changing it requires an ADR.

```json
{
  "error": {
    "code": "invalid_request",
    "message": "field \"email\" is required",
    "status": 400,
    "request_id": "01H...",
    "details": [{"field": "email", "reason": "required"}]
  }
}
```

`details` is omitted when empty. No other top-level keys are permitted.

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Sentinel not-found error | 404, code `not_found`, generic message |
| Wrapped database error, production mode | 500, generic message, cause only in the log |
| Wrapped database error, debug mode | 500, cause and stack trace in the log; body unchanged |
| Panic in a handler | 500 through the error handler, envelope identical to other 500s |
| Validation error with field details | 400 with populated `details` |
| `errors.Is` against a sentinel through two wraps | true |

## Acceptance Criteria
- [ ] One envelope shape for every error the framework emits, asserted by tests.
- [ ] Production-mode responses are verified to contain no cause text and no stack frames.
- [ ] Stack capture is off by default and costs nothing when disabled.
- [ ] `errors.Is` and `errors.As` work through wrapping, tested.
- [ ] Router 404 and 405 responses now use the envelope.
- [ ] Recovery-produced errors are indistinguishable from other internal errors to a client.

## Test Plan
Record in `test/plans/phase5.md`. Include a test that fails if any client-facing body
contains the substring `goroutine` or the repository path.
