# Phase 3 — Middleware System Test Plan

## Scope

Tests for the `pkg/middleware` package and the middleware integration added to `pkg/router`.

## Test Files

| File | Package | Purpose |
|---|---|---|
| `pkg/middleware/middleware_test.go` | `middleware` | Unit tests for Chain, Middleware type, and Recovery |
| `pkg/router/router_use_test.go` | `router` | Integration tests for Router.Use and Group.Use |

## Test Cases

### pkg/middleware — Chain Composition

| Test | What it verifies |
|---|---|
| `TestExecutionOrder` | Chain of A, B, C executes in order A→B→C→handler→C→B→A, recorded by a sequence slice |
| `TestChainEmpty` | Composing zero middleware returns the original handler reference unchanged |
| `TestChainShortCircuit` | Middleware that skips next.ServeHTTP prevents handler execution; response is what the middleware wrote |
| `TestContextPropagation` | Value placed in context by middleware A is visible in middleware B and the handler (ADR-005) |
| `TestChainAppend` | Chain.Append returns a new immutable chain; the receiver is unmodified |
| `TestMiddlewareWritesHeaderThenCallsNext` | A header set by middleware before calling next is present in the final response |

### pkg/middleware — Recovery

| Test | What it verifies |
|---|---|
| `TestRecoveryPanic` | Handler panic → 500 response; does not propagate to test |
| `TestRecoveryErrAbortHandler` | Panic value of http.ErrAbortHandler is re-panicked, not converted to 500 (ADR-007) |
| `TestRecoveryAfterPartialResponse` | Panic after WriteHeader does not overwrite the committed status code |
| `TestRecoveryNilLogger` | Passing nil logger falls back to slog.Default() without panic |

### pkg/middleware — Concurrency

| Test | What it verifies |
|---|---|
| `TestConcurrentRequests` | 20 goroutines × 50 requests through the same chain; passes -race with correct counter |

### pkg/router — Use() Integration

| Test | What it verifies |
|---|---|
| `TestRouterUseAppliesGlobally` | Router.Use middleware wraps all routes |
| `TestGroupUseAppliesOnlyToGroup` | Group.Use middleware wraps only routes registered through that group |
| `TestGlobalAndGroupChainOrder` | Global chain runs before group chain: global→group→handler |
| `TestNestedGroupChainOrder` | Nested groups compose: global→parent→child→handler |
| `TestMultipleUseCallsAccumulate` | Multiple Router.Use calls are additive and preserve registration order |
| `TestRouterConcurrentRequests` | Many goroutines drive the same router+chain simultaneously; passes -race |

## Concurrency Test Detail

`TestConcurrentRequests` (in `middleware_test.go`) launches 20 goroutines, each sending 50 requests through a single constructed `http.Handler` (the result of `Chain.Then`). An atomic counter incremented by a middleware is checked at the end to verify all 1,000 requests were processed.

`TestRouterConcurrentRequests` (in `router_use_test.go`) launches 20 goroutines sending requests through a Router with one global middleware and one registered route. The test verifies every response is 200 OK.

Both tests are run with `go test -race` to catch data races on the chain slice.

## How to Run

```bash
source ./environment.sh && go test ./pkg/middleware/... -race -cover -v
source ./environment.sh && go test ./pkg/router/... -race -cover -v
source ./environment.sh && go test ./... -race -cover
```
