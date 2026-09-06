# Phase 2 — Core Router: Test Plan

## Scope
Tests for `pkg/router` covering all behavioral requirements from
`docs/specifications/phase2.md`. All handler tests use `net/http/httptest`;
no real ports are bound.

## Test file
`pkg/router/router_test.go` (package `router_test` — black-box).

## Compile-time assertion
```go
var _ http.Handler = (*router.Router)(nil)
```
Fails to compile if `Router` does not implement `http.Handler`.

## Test cases

| Test | Behavioral requirement covered |
|---|---|
| `TestRouter_MethodDispatch/{GET,POST,PUT,DELETE,PATCH}` | All five methods registerable and dispatched correctly |
| `TestRouter_PathParam_Single` | Single `:name` parameter extracted and stored in context |
| `TestRouter_PathParam_Multiple` | Multiple parameters in one pattern all extracted |
| `TestRouter_NotFound` | Unmatched path → 404 |
| `TestRouter_MethodNotAllowed_StatusAndAllowHeader` | Path known, method not registered → 405 with Allow header listing registered methods |
| `TestRouter_Group_PrefixCompose` | Group prefix prepended correctly (`/api/v1` + `/users`) |
| `TestRouter_Group_AllMethods` | All five methods registerable through a Group |
| `TestRouter_NestedGroups` | Nested groups concatenate prefixes (`/api/v1` + `/admin` + `/settings`) |
| `TestRouter_QueryValues_Single` | Single query param accessible via `router.QueryValues` |
| `TestRouter_QueryValues_Repeated` | Repeated key (`?a=1&a=2`) returns both values |
| `TestRouter_QueryValues_Empty` | Empty-value key (`?k=`) returns empty string |
| `TestRouter_PathParam_PercentDecoded` | `%20` in path segment decoded to space before param extraction |
| `TestRouter_TrailingSlash_Distinct` | `/users` and `/users/` are distinct routes; each calls only its handler |
| `TestRouter_TrailingSlash_404WhenUnregistered` | Request to `/users/` returns 404 when only `/users` is registered |
| `TestRouter_DuplicateParamNames_Error` | Pattern with `:id` appearing twice returns a registration error |
| `TestRouter_DuplicateRoute_Error` | Registering the same method+pattern twice returns an error |
| `TestRouter_StaticWinsOverParam` | Static `/users/list` beats param `/users/:id` for request `/users/list` |
| `TestRouter_BehavioralTable` | Table-driven: match→200, method-mismatch→405+Allow, no-match→404 |
| `TestRouter_RootPath` | `GET /` dispatches to root handler |
| `BenchmarkRouter_Match` | 90-route table (~3 prefixes × 10 resources × 3 methods); benchmarks a deep param match |

## Trailing slash decision
Trailing slash is **significant**: `/users` and `/users/` are distinct route
patterns. A request to `/users/` will 404 if only `/users` is registered (and
vice versa). This is tested by `TestRouter_TrailingSlash_Distinct` and
`TestRouter_TrailingSlash_404WhenUnregistered`.

## No-backtracking documentation
At each segment level, static children are tried first. If a static child
exists at a level but the deeper match fails, the router does **not** retry
the wildcard child. This keeps matching O(path segments) and is tested by
`TestRouter_StaticWinsOverParam`.

## Coverage
88.3 % statement coverage achieved with the race detector enabled.

## Commands run to verify
```
source ./environment.sh && go test ./pkg/router/... -race -cover -v
```
All tests passed; no race conditions detected.
