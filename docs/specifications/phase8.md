# Phase 8 — Validation and Example

**Goal:** wire every component together in a runnable server, prove it works with a real
example, and document it.

## Scope
1. `cmd/server/main.go` that constructs the DI container, registers services, builds the
   middleware chain, mounts routes, and starts an `http.Server` with graceful shutdown.
2. At least one functional example under `examples/` that a reader can run and exercise
   with `curl`.
3. A clean quality gate: `go build ./...`, `go vet ./...`, `gofmt -l .`, and
   `go test ./... -race`.
4. A concise `README.md` covering architecture, usage, and design decisions.

## Out of Scope
- Deployment manifests, containers, and CI configuration.
- Persistence. The example uses an in-memory store.

## Design Constraints
- The example must exercise every phase: routing with a path parameter, a route group, the
  middleware chain, typed request parsing, a structured response, a deliberate error path,
  a DI-resolved service, and observability output.
- Configuration (port, log level, debug mode) comes from environment variables with safe
  defaults. Never hardcode secrets.
- The `http.Server` sets `ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, and
  `IdleTimeout`.
- Graceful shutdown on `SIGINT` and `SIGTERM` using `signal.NotifyContext` and
  `srv.Shutdown` with a bounded timeout.
- The example is buildable by `go build ./...`; it must not be a code block in a markdown
  file only.
- `main` stays thin: wiring only, no business logic.

## Suggested Example Surface
A minimal in-memory resource API under a versioned group:

| Method | Path | Behavior |
|---|---|---|
| GET | `/api/v1/health` | 200, static JSON, no auth |
| GET | `/api/v1/items` | 200, list, supports `?limit=` |
| POST | `/api/v1/items` | 201, JSON body, validation errors as 400 with details |
| GET | `/api/v1/items/:id` | 200 or 404 through the error envelope |
| DELETE | `/api/v1/items/:id` | 204 or 404 |
| GET | `/api/v1/metrics` | 200, metrics snapshot |
| GET | `/api/v1/boom` | Panics deliberately, to demonstrate recovery |

## README Requirements
The README must contain, in this order:
1. What the project is and the standard-library-only constraint.
2. Quick start: build, run, and three or four `curl` commands with their expected output.
3. Architecture overview with the package table and the layering rule.
4. A short usage section per package: router, middleware, request/response, errors, DI,
   observability.
5. Design decisions: a condensed list pointing at `docs/design/ADR.md`.
6. Project layout tree.
7. Testing instructions.

Keep it concise. It documents what exists; it is not a tutorial.

## Acceptance Criteria
- [ ] `go build ./...` succeeds with no output.
- [ ] `go vet ./...` reports nothing.
- [ ] `gofmt -l .` prints nothing.
- [ ] `go test ./... -race -cover` passes.
- [ ] `go list -m all` shows only the main module.
- [ ] The server starts, serves every route in the table, and shuts down cleanly on SIGINT.
- [ ] Each `curl` example in the README was actually run and its output matches.
- [ ] `docs/design/sourcemap.md` lists every source file in the repository.
- [ ] `docs/design/packagedesign.md` matches the real import graph.
- [ ] The complete file tree and the compilation status are reported to the user.

## Test Plan
Record in `test/plans/phase8.md`. Include an end-to-end test that starts the server with
`httptest.NewServer` and drives the full route table.
