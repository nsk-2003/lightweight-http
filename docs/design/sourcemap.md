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
| `pkg/middleware/doc.go` | Package declaration for the middleware composition package. |
| `pkg/middleware/middleware.go` | Middleware function type and Chain composition abstraction; applies middleware in registration order (ADR-009). |
| `pkg/middleware/recovery.go` | Recovery middleware that converts handler panics into 500 responses and re-panics on http.ErrAbortHandler (ADR-007). |
| `pkg/middleware/middleware_test.go` | Tests for chain execution order, short-circuiting, context propagation, panic recovery, and concurrency safety. |
| `pkg/router/router_use_test.go` | Integration tests for Router.Use and Group.Use verifying global and group chain composition order. |
| `test/plans/phase3.md` | Test plan for Phase 3: middleware chain, recovery, and router integration. |
| `pkg/di/doc.go` | Package declaration stub for the dependency injection package (Phase 1 placeholder). |
| `pkg/observability/doc.go` | Package declaration stub for the observability package (Phase 1 placeholder). |
| `pkg/errors/errors.go` | HTTPError type and constructor functions for framework errors with HTTP status codes (ADR-006). |
| `pkg/errors/errors_test.go` | Tests for HTTPError creation, status codes, and the error string format. |
| `pkg/router/request.go` | Typed request parsing for JSON bodies, form data, and query parameters with size and content-type enforcement. |
| `pkg/router/request_test.go` | Tests for ParseJSON, ParseForm, RequireQuery, and QueryInt covering every behavioral requirement row. |
| `pkg/router/response.go` | ResponseWriter with status/byte tracking and content-negotiated write helpers (WriteJSON, WriteText, Respond, NoContent). |
| `pkg/router/response_test.go` | Tests for the ResponseWriter wrapper and the Respond/WriteJSON/WriteText/NoContent helpers. |
| `test/plans/phase4.md` | Test plan for Phase 4: request parsing and response writing behavioral coverage. |
| `test/testdata/valid.json` | Valid JSON fixture used by ParseJSON success tests. |
| `test/testdata/malformed.json` | Syntactically invalid JSON fixture used by malformed-body tests. |
| `test/testdata/unknown_field.json` | JSON fixture with an extra field used by strict-mode unknown-field tests. |
