# Phase 3 — Middleware System

**Goal:** a composable middleware chain in `pkg/middleware/middleware.go` that wraps
handlers, propagates context, and recovers from panics.

## Scope
1. Define the middleware type and a chain that composes any number of middleware into one
   handler.
2. Allow middleware to modify the request before the handler runs and the response after.
3. Propagate values through `context.Context` from one middleware to the next and to the
   handler.
4. Recover from panics in handlers and downstream middleware, converting them into a
   500-class response.
5. Support attaching a chain globally and per route group.

## Out of Scope
- The final error wire format (Phase 5). In this phase, recovery may emit a minimal 500;
  Phase 5 replaces the body with the structured envelope.
- Logging, metrics, and tracing middleware (Phase 7).

## Design Constraints
- Execution order is registration order: the first middleware registered is the outermost
  and sees the request first and the response last (ADR-009).
- A group's chain runs after its parent's chain, in the parent-to-child direction.
- Middleware must not mutate the shared request struct in place in a way visible to
  siblings; derive a new request with `r.WithContext(ctx)`.
- Context keys are unexported types, never strings (ADR-005).
- Recovery is the only place in the codebase that calls `recover()` (ADR-007). It must
  re-panic on `http.ErrAbortHandler` so the server's own handling is preserved.
- Recovery must not write a body if the handler already wrote the response header; detect
  and log that case instead.
- Composing zero middleware returns the original handler unchanged.

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Chain of A, B, C | Request order A→B→C→handler; response order handler→C→B→A |
| Middleware writes a header then calls next | Header is present in the final response |
| Middleware short-circuits without calling next | Handler never runs; response is what the middleware wrote |
| Handler panics | Recovery converts it; the server does not drop the connection |
| Panic value is `http.ErrAbortHandler` | Re-panicked, not converted |
| Context value set in A | Visible in B, C, and the handler |

## Acceptance Criteria
- [ ] Order of execution is asserted by a test that records a sequence, not by inspection.
- [ ] Short-circuiting middleware is supported and tested.
- [ ] Panic recovery returns a 500-class response and logs the incident.
- [ ] Group-level chains compose with global chains in the documented order.
- [ ] No data race under `go test -race` with concurrent requests through one chain.

## Test Plan
Record in `test/plans/phase3.md`. Include a concurrency test that drives the same chain from
multiple goroutines.
