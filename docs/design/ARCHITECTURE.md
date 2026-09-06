# ARCHITECTURE OF THE LIGHTWEIGHT HTTP FRAMEWORK

## Key Architecture Guidelines
Always follow the decisions in `ADR.md`.

### Layering
Packages are organized in layers. A package may depend on a lower layer, never on a higher
layer or a sibling in the same layer.

| Layer | Packages | May depend on |
|---|---|---|
| 4 — Application | `cmd/`, `examples/` | all layers |
| 3 — Composition | `pkg/middleware`, `pkg/observability` | layers 1–2 |
| 2 — Core | `pkg/router`, `pkg/di` | layer 1 |
| 1 — Foundation | `pkg/errors` | standard library only |

Rules:
- Module dependencies must be acyclic. `go build ./...` enforces this, but design for it
  rather than discovering it at compile time.
- `pkg/errors` is the only package every other package may import. It must import nothing
  from `pkg/`.
- `pkg/router` must not import `pkg/middleware`. Middleware wraps a router-produced
  handler; the router does not know middleware exists.
- `pkg/observability` is consumed through the middleware chain, not called directly from
  the router.
- `pkg/di` must not be imported by `pkg/router`, `pkg/middleware`, or `pkg/errors`. The
  container is a composition-root tool, used by `cmd/` and `examples/`.
- Nothing under `pkg/` may import anything under `cmd/` or `examples/`.

### Package Responsibilities
- `pkg/errors` — error type, HTTP status mapping, structured serialization, optional stack
  capture. Owns the wire format of every error response.
- `pkg/router` — method and path matching, path parameters, route groups, query access,
  typed request binding, and the structured response writer.
- `pkg/middleware` — the chain abstraction and built-in middleware (recovery, logging,
  request ID, timeout).
- `pkg/di` — service registration and resolution with singleton and scoped lifecycles, and
  circular-dependency detection.
- `pkg/observability` — trace context, metrics collection, structured logging, exposed as
  middleware-compatible hooks.

### Implementation Guidelines
- Every exported type, function, and method carries a doc comment starting with its name.
- Every file starts with a `Purpose:` comment describing why the file exists.
- Handlers are plain functions over the framework's request/response abstractions; the
  framework must remain compatible with `http.Handler` at its boundary so a user can mount
  it on any `net/http` server.
- Keep allocations off the hot path: reuse buffers where it is measurable, but do not
  optimize before there is a benchmark showing the cost.
- Public API changes require an ADR entry.
- Generate and update the package design diagram in `docs/design/packagedesign.md`, in
  markdown with mermaid.js diagrams.

## Technology Stack
- language : Go 1.25 or later
- dependencies : Go standard library only (`net/http`, `net/url`, `context`, `encoding/json`,
  `log/slog`, `sync`, `time`, `errors`, `fmt`, `reflect` only where unavoidable in `pkg/di`)
- build tool : the `go` toolchain (`go build`, `go vet`, `go test`)
- unit test framework : the built-in `testing` package, plus `net/http/httptest`
- module path : `github.com/example/lightweight-http`

No third-party module may be added. If a task appears to require one, stop and ask.

## Technology-Stack-Specific Instructions for Code Generation

```bash
# Build all packages
go build ./...

# Build a static server binary
CGO_ENABLED=0 GOOS=linux go build -o build/server ./cmd/server

# Static checks
go vet ./...
gofmt -l .

# Tests
go test ./... -race -cover
```

Conventions:
- Package-local unit tests live beside the code as `<file>_test.go` (ADR-002).
- Use `net/http/httptest` for handler tests; do not bind real ports in unit tests.
- Table-driven tests are the default style.
- Benchmarks, where they exist, live in `<file>_test.go` as `BenchmarkXxx`.

## Design Documents
- `docs/design/ADR.md` : architecture decision records. Binding.
- `docs/design/sourcemap.md` : list of source files and their purpose. Use this to decide
  which files to modify during code generation.
- `docs/design/packagedesign.md` : package/module dependency diagram.
