# Phase 2 Test Plan — Core Router

## Approach

All handler tests use `net/http/httptest` (no real ports). Tests are written in
`pkg/router/router_test.go` (package-internal, per ADR-002) before the implementation.
A benchmark is included to exercise the router against a table of 36 routes.

## Behavioral Requirements Coverage

| Situation | Test | Expected result |
|---|---|---|
| Path matches, method matches | `TestMethodDispatch`, `TestStaticRouteDispatch` | Handler runs, 200 OK |
| Path matches, method does not | `TestMethodNotAllowed` | 405 with Allow header listing registered methods |
| No path match | `TestNotFound` | 404 |
| Trailing slash mismatch | `TestTrailingSlashMismatch` | 404 — trailing slash is significant; `/users` ≠ `/users/` |
| Duplicate parameter names in one pattern | `TestDuplicateParamNameError` | Registration error |
| Percent-encoded path segment | `TestPercentEncodedPathParam` | Decoded before parameter extraction (uses `r.URL.Path`) |
| Repeated query key (`?a=1&a=2`) | `TestQueryParams` | Both values retrievable via `r.URL.Query()["tag"]` |

## Acceptance Criteria Status

- [x] All five methods (GET, POST, PUT, DELETE, PATCH) are registerable and dispatch correctly (`TestMethodDispatch`, `TestGroupAllMethods`).
- [x] Path parameters are extracted for single (`TestSinglePathParam`) and multiple parameters (`TestMultiplePathParams`) in one pattern.
- [x] Query values are readable, including repeated (`tag=go&tag=http`) and empty (`empty=`) values (`TestQueryParams`).
- [x] Nested groups compose their prefixes correctly (`TestNestedGroups`).
- [x] 404 and 405 behave as specified; 405 includes `Allow` header (`TestNotFound`, `TestMethodNotAllowed`).
- [x] Table-driven tests cover every row in the behavioral table above.
- [x] The router satisfies `http.Handler`, verified by a compile-time assertion (`var _ http.Handler = (*Router)(nil)`).

## Additional Tests

| Test | What it verifies |
|---|---|
| `TestStaticWinsOverParam` | Static segment `/users/me` beats `/:id` when both could match |
| `TestRouteGroup` | Group prefixes routes correctly; un-prefixed path returns 404 |
| `TestDuplicateRouteError` | Duplicate method+pattern returns error, does not overwrite |
| `TestRootPattern` | `"/"` can be registered and matched |

## Benchmark

`BenchmarkRouterMatch` — 36 routes registered across all five methods with static and
parameterised segments. Measures dispatch latency for `GET /users/:id/posts/:postId`.

## Verification Commands Run

```
source ./environment.sh && go build ./...
# (no output — success)

source ./environment.sh && go vet ./...
# (no output — success)

source ./environment.sh && gofmt -l .
# (no output — success)

source ./environment.sh && go test ./... -race -cover
# ?   github.com/example/lightweight-http/pkg/di          [no test files]
# ?   github.com/example/lightweight-http/pkg/errors      [no test files]
# ?   github.com/example/lightweight-http/pkg/middleware  [no test files]
# ?   github.com/example/lightweight-http/pkg/observability [no test files]
# ok  github.com/example/lightweight-http/pkg/router      1.786s  coverage: 87.9% of statements
```

## Design Decisions

- **Trailing slash is significant.** `/users` and `/users/` are distinct patterns. No automatic
  redirect. Documented in `TestTrailingSlashMismatch`.
- **No backtracking.** When a static child matches at a segment, the traversal commits. No
  fallback to the wildcard child if the subtree yields no match (O(segments) guarantee).
- **Conflicting wildcard names at the same trie position are rejected.** If two routes use
  different parameter names at the same trie level (e.g., `/:id` and `/:name`), registration
  returns an error.
- **Registration returns errors.** Duplicate routes and invalid patterns return `error` rather
  than panicking (per working rules: "return errors; do not panic in library code").
