# Phase 6 — Dependency Management

**Goal:** a lightweight DI container in `pkg/di/container.go` supporting registration,
interface resolution, singleton and scoped lifecycles, and circular-dependency detection.

## Scope
1. Register a service by key or interface, providing a constructor function.
2. Resolve a service, constructing its dependencies as needed.
3. Two lifecycles: **singleton** (one instance per container, built at most once) and
   **scoped** (one instance per request scope, disposed when the scope ends).
4. Detect circular dependencies during resolution and return a descriptive error.
5. Create a request-scoped child container and attach it to the request context.

## Out of Scope
- Automatic constructor injection by scanning struct fields.
- Configuration file or annotation-driven wiring.
- Lazy proxies and interception.

## Design Constraints
- Explicit registration over reflective auto-wiring; keep reflection to the minimum needed
  for interface keys (ADR-010).
- Resolution errors are returned, never panicked.
- A cycle is detected by tracking the resolution path; the error message names the full
  cycle (`A -> B -> C -> A`).
- Singleton construction is thread-safe and happens at most once even under concurrent
  resolution, including when construction fails: a failed constructor must not leave a
  half-built instance cached, and the error must be returned to every concurrent caller.
- Scoped instances never leak across requests. A scoped service resolved from the root
  container is an error, not a silent singleton.
- Disposal runs in reverse construction order, and a disposal failure does not prevent the
  remaining disposals.
- The container is only constructed at the composition root (`cmd/`, `examples/`).
  `pkg/router`, `pkg/middleware`, and `pkg/errors` must not import it.

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Resolve an unregistered key | Descriptive error naming the key |
| Resolve a singleton twice | Same instance both times |
| Resolve a scoped service twice in one scope | Same instance |
| Resolve a scoped service in two scopes | Different instances |
| Resolve a scoped service from the root | Error |
| Registered `A` depends on `B` depends on `A` | Error naming the cycle |
| Constructor returns an error | Propagated with the key in the message |
| Concurrent resolution of one singleton | Constructor runs exactly once |
| Duplicate registration of one key | Error, or documented last-wins — pick one and test it |
| Scope ends | Disposables disposed in reverse order |

## Acceptance Criteria
- [ ] Every row of the behavior table has a test.
- [ ] `go test -race` passes with a test that resolves one singleton from many goroutines.
- [ ] Cycle errors name the full path, verified by string assertion.
- [ ] Scoped containers are attachable to and retrievable from a request context.
- [ ] No package in layers 1–3 other than `pkg/di` imports `pkg/di`.

## Test Plan
Record in `test/plans/phase6.md`. Include a test that asserts the layering rule by scanning
imports, or document why it is verified by review instead.
