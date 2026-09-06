# Phase 3 — Middleware System: Test Plan

## Scope
Tests for `pkg/middleware` covering all behavioral requirements from
`docs/specifications/phase3.md`. All handler tests use `net/http/httptest`;
no real ports are bound.

## Test file
`pkg/middleware/middleware_test.go` (package `middleware_test` — black-box).

## Test cases

| Test | Behavioral requirement covered |
|---|---|
| `TestChain_ExecutionOrder` | Chain of A, B, C: request order A→B→C→handler; response order handler→C→B→A, asserted by recording a named-event sequence |
| `TestChain_ZeroMiddleware_ReturnsOriginalHandler` | Composing zero middleware returns the original handler unchanged |
| `TestChain_ShortCircuit` | Middleware that does not call next prevents the handler from running and controls the response |
| `TestChain_ContextPropagation` | Context value set in middleware A is visible in the downstream handler |
| `TestChain_MiddlewareHeaderForwarding` | Header set by middleware before calling next is present in the final response |
| `TestChain_GroupComposesWithGlobal` | Global chain [A, B] composed with group chain [C] executes in parent-to-child order A→B→C→handler |
| `TestChain_Concurrent_NoRace` | Same chain driven from 20 goroutines × 50 requests each; no data race under `go test -race` |
| `TestRecovery_PanicConvertsTo500` | Handler panic yields a 500 response and a log entry containing "panic recovered" |
| `TestRecovery_ErrAbortHandler_Repanics` | `http.ErrAbortHandler` is re-panicked, not swallowed, so the server's abort handling is preserved |
| `TestRecovery_AlreadyWroteHeader_LogsAndDoesNotWrite500` | When the response header is already committed, Recovery logs but does not attempt to write a second header |

## Key design decisions recorded by tests

### Execution order
The first middleware registered is the outermost wrapper. `Chain(A, B, C)(handler)`
produces the call stack A→B→C→handler on the way in, and handler→C→B→A on the way out.
This is asserted by recording named `"X-before"` / `"X-after"` strings and comparing
the full slice — not by inspection.

### Group chain composition
`global(group(handler))` where `global = Chain(A, B)` and `group = Chain(C)` produces
execution order A→B→C→handler. This is the documented parent-to-child direction.

### ErrAbortHandler re-panic
Recovery calls `panic(v)` for `v == http.ErrAbortHandler` inside the deferred recovery
function. The test captures the re-panic using an outer `defer func() { caught = recover() }()`
and asserts `caught == http.ErrAbortHandler`.

### Already-committed response
`Recovery` wraps the `http.ResponseWriter` in a thin `responseWriter` that sets a
`wrote` flag on the first `WriteHeader` or `Write` call. If the flag is set when a
panic is caught, Recovery logs the panic but does not call `http.Error` (which would
race with the already-written status).

## Concurrency test
`TestChain_Concurrent_NoRace` spins up 20 goroutines each sending 50 requests through
the same composed handler. The test must be run with the `-race` flag (standard in
`go test ./... -race`).

## Coverage
86.4% statement coverage achieved with the race detector enabled.

## Commands run to verify
```
source ./environment.sh && go test ./pkg/middleware/... -race -cover -v
```
All 10 tests passed; no race conditions detected.
