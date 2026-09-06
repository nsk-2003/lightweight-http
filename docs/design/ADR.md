# Architecture Decision Record (ADR) Index

This is an index of key architecture decision records.

The index format is of two types:
1. `<relative document path>` : {{short 2-3 line description of document content}}
2. {{decision description for short decisions}}

Decisions here are binding. To change one, ask the user and record the supersession; do not
silently deviate.

## ADR Index

- **ADR-001 — Standard library only.** No third-party modules, no vendoring, no code copied
  from third-party frameworks. `go.mod` must contain no `require` entries beyond the module
  and Go directives. Rationale: the project's purpose is to demonstrate the core
  abstractions, which is defeated if a library supplies them.
- **ADR-002 — Tests live beside the code.** Package-local unit tests are `<file>_test.go` in
  the same package directory, per Go convention. `test/plans/` holds the written test plans
  and `test/testdata/` holds shared fixtures. `test/unit/` is reserved for black-box tests
  that must not import package internals.
- **ADR-003 — The framework stays `net/http`-compatible at its boundary.** The router
  implements `http.Handler`, and user handlers are adaptable to and from
  `http.HandlerFunc`. Rationale: interoperability with the existing ecosystem of servers
  and middleware is worth more than a bespoke handler signature.
- **ADR-004 — Routing uses an explicit trie/segment matcher, not regular expressions.** Path
  patterns support static segments and `:name` parameters only. No regex routes, no
  backtracking matcher. Rationale: predictable O(path segments) matching and no
  catastrophic-backtracking risk from user-supplied patterns.
- **ADR-005 — Request-scoped values travel in `context.Context`.** Path parameters, request
  ID, trace context, and scoped DI containers are attached to the request context using
  unexported key types. No global mutable state, no `sync.Map` keyed by request pointer.
- **ADR-006 — One error type, one wire format.** All errors returned to clients pass through
  `pkg/errors`, which maps them to an HTTP status and a single JSON envelope. Handlers
  return errors; they do not write error responses themselves.
- **ADR-007 — Panics are converted, never propagated.** The recovery middleware is the only
  place that calls `recover()`. It converts the panic into a `pkg/errors` internal error and
  routes it through the normal error handler. Library code never panics deliberately.
- **ADR-008 — Stack traces are opt-in and never leak by default.** Stack traces are captured
  only when debug mode is enabled and are written to logs. The client-facing error body
  never contains a stack trace, internal path, or wrapped internal message in production
  mode.
- **ADR-009 — Middleware executes in registration order.** The first middleware registered
  is the outermost wrapper and therefore sees the request first and the response last.
  Chains compose: a group's chain runs after its parent's.
- **ADR-010 — DI resolution is explicit and reflection is minimized.** Prefer constructor
  functions registered against a key or interface over reflective auto-wiring. Circular
  dependencies are detected during resolution and returned as an error, never as a stack
  overflow.
- **ADR-011 — Structured logging uses `log/slog`.** No custom logging abstraction beyond a
  thin interface for injection in tests. Logs are key-value structured; secrets, tokens,
  cookies, and authorization headers are never logged.
- **ADR-012 — Metrics are in-process and pull-based.** Request count, latency, and status
  code counters are held in memory behind a small interface, exposed via a snapshot method.
  No exporter, no network protocol, no third-party metrics format.
- `./packagedesign.md` : Documents the package/module design and their dependencies in
  mermaid.js format. Keep it in sync with the real import graph.
