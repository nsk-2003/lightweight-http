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
| `go.mod` | Go module declaration; defines module path and minimum Go version. |
| `environment.sh` | Loads development environment variables; source before running any Go command. |
| `pkg/errors/doc.go` | Package declaration stub for the errors foundation package (Phase 1 placeholder). |
| `pkg/router/doc.go` | Package-level doc comment for the router package. |
| `pkg/router/router.go` | Router and Group types: method registration, ServeHTTP dispatch, and route group support. |
| `pkg/router/trie.go` | Segment trie implementation: O(segments) path matching without backtracking (ADR-004). |
| `pkg/router/params.go` | Unexported context key and accessor functions for path and query parameters (ADR-005). |
| `pkg/router/router_test.go` | Table-driven tests and benchmark for routing dispatch, path params, query access, and groups. |
| `pkg/middleware/doc.go` | Package declaration stub for the middleware composition package (Phase 1 placeholder). |
| `pkg/di/doc.go` | Package declaration stub for the dependency injection package (Phase 1 placeholder). |
| `pkg/observability/doc.go` | Package declaration stub for the observability package (Phase 1 placeholder). |
