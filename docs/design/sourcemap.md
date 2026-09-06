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
