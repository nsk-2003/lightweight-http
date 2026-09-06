# source map

Every source file in the project, with its purpose. This is the first place to look when
deciding which file to modify, and the file you must update whenever you add, rename, or
delete a source file.

## Rules
- Add a row the moment you create a file — not at the end of the phase.
- The path is relative to the project root.
- The purpose is one line describing why the file exists, not a list of its functions.
- Test files (`*_test.go`) are listed too.
- If a file no longer has a distinct purpose you can state in one line, it probably should
  not exist.

## Source Files

| Source File | Purpose |
|---|---|
| `doc.go` | Module-level package documentation; provides a compilable root package so toolchain commands (`go vet ./...`, `go test ./...`) have a target even before implementation files exist. |
| `pkg/router/context.go` | Unexported context key type plus `Params` and `QueryValues` accessor functions; keeps path-parameter storage invisible to other packages (ADR-005). |
| `pkg/router/trie.go` | Trie node and matching logic: O(segments) path matching, static-wins-over-param priority, no backtracking (ADR-004). |
| `pkg/router/router.go` | `Router` and `Group` types: method registration, `ServeHTTP` dispatch (404/405), path splitting, and route grouping with prefix composition. |
| `pkg/router/router_test.go` | Black-box tests for the router package covering all phase-2 behavioral requirements and a routing-table benchmark. |
| `pkg/middleware/middleware.go` | `Middleware` type, `Chain` composer, and `Recovery` built-in middleware with panic recovery and response-committed detection. |
| `pkg/middleware/middleware_test.go` | Black-box tests for the middleware package covering all phase-3 behavioral requirements: execution order, short-circuit, context propagation, panic recovery, and concurrency. |
| `pkg/errors/errors.go` | `HTTPError` type, sentinel errors, JSON envelope, central `Handler`/`ServeError`, request-ID context helpers, and `WriteError` convenience wrapper (Phase 5). |
| `pkg/errors/errors_test.go` | Black-box tests for the errors package covering Phase 4 and Phase 5 requirements: constructor, unwrap, sentinels, errors.Is/As through wrapping, JSON envelope shape, production-mode safety, and request ID. |
| `pkg/router/request.go` | Typed request parsing helpers — `BindJSON`, `BindForm`, `FormParam`, `FormParamInt`, `QueryParam`, `QueryParamInt`, `PathParam` — with body limits and Content-Type validation. |
| `pkg/router/request_test.go` | Black-box tests for request parsing covering all phase-4 behavioral requirements: JSON, form, query, and path sources, error codes, and body size limits. |
| `pkg/router/response.go` | `ResponseWriter` wrapper tracking status and bytes for Phase 7 metrics; `JSON`, `Text`, `NoContent`, and `Respond` (content-negotiated) response helpers. |
| `pkg/router/response_test.go` | Black-box tests for the response writer and helpers covering status tracking, byte counting, Content-Type, and content negotiation. |
| `test/testdata/valid.json` | Shared JSON fixture with a valid object used by request-parsing tests. |
| `test/testdata/malformed.json` | Shared JSON fixture with intentionally malformed content used by request-parsing tests. |
| `pkg/di/container.go` | DI container: registration, singleton/scoped lifecycle resolution, circular-dependency detection, request-scope disposal, and context integration (ADR-005, ADR-010). |
| `pkg/di/container_test.go` | Black-box tests for the di package covering all Phase 6 behavioral requirements: registration, singletons, scopes, cycles, constructor errors, disposal ordering, and context helpers. |
| `pkg/observability/observability.go` | RequestID, Logger, and Metrics middleware plus the in-process Recorder — the full Phase 7 observability stack exposed as composable middleware functions (ADR-011, ADR-012). |
| `pkg/observability/observability_test.go` | Black-box tests for the observability package covering all Phase 7 requirements: request ID generation/adoption/validation, standard log fields, Authorization redaction, metrics counting/latency/status, route-pattern aggregation, panic resilience, and race-detector cleanliness. |
