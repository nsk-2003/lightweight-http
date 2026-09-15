# Test Plan — Phase 6: Dependency Management

## Scope

Tests for the DI container in `pkg/di/container.go` and the layering rule enforced by
`test/unit/di_layering_test.go`.

## Behavioral Coverage

Each row of the spec's behavioral table is covered by a named test in
`pkg/di/container_test.go`:

| Behavior | Test |
|---|---|
| Resolve unregistered key | `TestResolveUnregistered` |
| Resolve singleton twice — same instance | `TestSingletonSameInstance` |
| Resolve scoped service twice in one scope — same instance | `TestScopedSameInstanceWithinScope` |
| Resolve scoped service in two scopes — different instances | `TestScopedDifferentInstancesAcrossScopes` |
| Resolve scoped service from root — error | `TestScopedFromRootIsError` |
| A → B → C → A circular dependency | `TestCircularDependencyError` |
| Constructor returns error — propagated with key | `TestConstructorErrorPropagated` |
| Concurrent singleton — constructor runs exactly once | `TestConcurrentSingletonConstructedOnce` |
| Duplicate registration | `TestDuplicateRegistrationError` |
| Scope ends — disposables disposed in reverse order | `TestScopeDisposeReverseOrder` |
| Disposal failure does not prevent other disposals | `TestDisposalFailureDoesNotPreventOthers` |

## Additional Tests

| Behavior | Test |
|---|---|
| Scope attached to and retrieved from context | `TestContextScopeAttachAndRetrieve` |
| ScopeFromContext returns false when absent | `TestContextScopeAbsent` |
| InterfaceKey registration and resolution | `TestInterfaceKeyRegistrationAndResolution` |
| Resolver resolves transitive dependencies | `TestResolverResolveDependency` |

## Layering Rule

`test/unit/di_layering_test.go` — `TestDINotImportedByLowerLayers` runs `go list -deps`
against `pkg/errors`, `pkg/router`, and `pkg/middleware` and asserts that
`pkg/di` does not appear in their transitive dependency set. This is the automated layering
assertion called for by the spec.

## Race Detector

`TestConcurrentSingletonConstructedOnce` spawns 100 goroutines all resolving the same
singleton concurrently. The race detector (enabled by `go test -race`) verifies there is no
data race and the constructor ran exactly once.

## Cycle Error String Assertion

`TestCircularDependencyError` registers a 3-node cycle (A → B → C → A) and asserts that
the returned error contains all three key names and the `->` separator, confirming the full
path is named in the message.

## Definition of Done Evidence

Run with:

```bash
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All steps must produce no output (build/vet/fmt) and all tests must pass.
