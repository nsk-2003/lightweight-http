# Phase 6 — Dependency Management: Test Plan

## Scope

Tests for the `pkg/di` package: `NewContainer`, `Register`, `Resolve`, `NewScope`,
`Scope.Resolve`, `Scope.Dispose`, `ContextWithScope`, `ScopeFromContext`.
All tests exercise the public API (black-box) from the `di_test` package.
No real ports are bound; no HTTP involved.

## Test file

- `pkg/di/container_test.go` (package `di_test` — black-box)

## Test table

| Test | Behavioral requirement covered |
|---|---|
| `TestRegister_Duplicate_ReturnsError` | Duplicate registration of one key → error |
| `TestResolve_Unregistered_ReturnsDescriptiveError` | Resolve an unregistered key → descriptive error naming the key |
| `TestSingleton_ResolvedTwice_SameInstance` | Resolve a singleton twice → same instance both times |
| `TestSingleton_ConcurrentResolution_ConstructorRunsOnce` | Concurrent resolution of one singleton → constructor runs exactly once; `go test -race` clean |
| `TestScoped_SameScope_SameInstance` | Resolve a scoped service twice in one scope → same instance |
| `TestScoped_TwoScopes_DifferentInstances` | Resolve a scoped service in two scopes → different instances |
| `TestScoped_FromRoot_ReturnsError` | Resolve a scoped service from the root container → error naming the key |
| `TestCircularDependency_TwoNode_NamesFullCycle` | A depends on B depends on A → error with `->` notation naming both keys |
| `TestCircularDependency_ThreeNode_NamesFullPath` | A → B → C → A → error naming all three keys and using `->` notation |
| `TestConstructor_Singleton_ErrorPropagatedWithKey` | Singleton constructor returns error → propagated, key named in message |
| `TestConstructor_Scoped_ErrorPropagatedWithKey` | Scoped constructor returns error → propagated, key named in message |
| `TestScope_Dispose_ReverseConstructionOrder` | Scope ends → disposables disposed in reverse construction order |
| `TestScope_Dispose_FailureDoesNotPreventRemainingDisposals` | Disposal failure → logged, remaining disposals still run |
| `TestContext_ScopeRoundTrip` | `ContextWithScope` + `ScopeFromContext` round-trips the exact same pointer |
| `TestContext_NoScope_ReturnsFalse` | `ScopeFromContext` on empty context → `(nil, false)` |
| `TestScope_CanResolve_Singleton_FromScope` | Scope can resolve singleton services from the parent container |
| `TestScoped_DependsOnSingleton` | Scoped service whose constructor resolves a singleton → resolves correctly |

## Layering rule

The layering rule (no package in layers 1–3 other than `pkg/di` imports `pkg/di`) is
verified by review: `pkg/errors`, `pkg/router`, and `pkg/middleware` have no import of
`github.com/example/lightweight-http/pkg/di` in their source files. `go build ./...` also
enforces the acyclic constraint; it passes clean.

## Key design decisions

### String keys, explicit registration
Keys are `string`; duplicate registration is an error (not silent last-wins). This forces
explicit, auditable wiring and prevents accidental override of a service at composition time.

### Singleton: sync.Once + cached error
`sync.Once` guarantees the constructor runs at most once even under concurrent calls.
If construction fails, the error is cached and returned to all concurrent and future callers;
no half-built instance is stored.

### Scoped: per-key sync.Once via sync.Map
Each key in a Scope gets its own `*scopedEntry` (holding a `sync.Once`), stored in a
`sync.Map`. `sync.Map.LoadOrStore` races safely; the winner's `Once.Do` runs the
constructor. This avoids holding a scope-wide lock during construction and prevents the
deadlock that would arise if a scoped service's constructor resolves another scoped service
in the same scope.

### Circular-dependency detection
The resolution path is tracked as `[]string` threaded through the `internalResolver`.
On each call to `resolve`, the incoming `key` is checked against every element of `path`.
A hit produces `"A -> B -> ... -> A"` using `strings.Join`. The path slice is copied with
`make`+`copy` before capture in closures to prevent slice-aliasing across concurrent callers.

### Disposal in reverse construction order
Each `Scope` maintains a `[]disposableEntry` slice appended in the order services implement
`Disposable`. `Scope.Dispose` snapshots the slice under `dispMu`, then iterates in reverse.
Disposal errors are passed to `slog.Default().Error` but do not abort the loop.

### Context integration (ADR-005)
`scopeKey{}` is an unexported struct type used as the context key, preventing collisions
with keys from other packages. `ContextWithScope` / `ScopeFromContext` are the only
public entry points.

## Coverage

- `pkg/di`: 91.5% statement coverage (from `go test -race -cover`)

## Commands run to verify

```
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All checks clean; all tests passed with no race conditions detected.
