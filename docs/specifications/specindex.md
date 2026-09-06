# Index of Specification Documents

This is the index of specification documents for the lightweight HTTP framework. Update the
index whenever you add a new specification document.

The index format is:
- `<relative document path>` : {{short 2-3 line description of document content}}

## Specifications Index
- `./phase1.md` : Project setup. Go module initialization, directory layout, toolchain
  baseline, and the standard-library-only constraint.
- `./phase2.md` : Core router. Method registration, path parameters, query parsing, and
  route grouping over `net/http` and `net/url`.
- `./phase3.md` : Middleware system. Composable chain, registration-order execution,
  context propagation, and panic recovery.
- `./phase4.md` : Request and response handling. Typed parsing of JSON, form, query, and
  path input; a structured response writer; content negotiation.
- `./phase5.md` : Centralized error handling. Error type, HTTP status mapping, structured
  error responses, optional stack traces, and panic routing.
- `./phase6.md` : Dependency management. A lightweight DI container with singleton and
  scoped lifecycles and circular-dependency detection.
- `./phase7.md` : Observability. Request tracing, request/latency/status metrics, and
  structured logging wired through the middleware chain.
- `./phase8.md` : Validation and example. `cmd/server` wiring, a functional example, a
  clean build and vet, and the project README.

## Phase Implementation Sequence
Strictly follow the sequence below:

1. Phase 1 — Project Setup
2. Phase 2 — Core Router
3. Phase 3 — Middleware System
4. Phase 4 — Request/Response Handling
5. Phase 5 — Centralized Error Handling
6. Phase 6 — Dependency Management
7. Phase 7 — Observability
8. Phase 8 — Validation & Example

Do not start the next phase until the previous phase is fully implemented, its Definition of
Done in `AGENTS.md` is satisfied, and the user explicitly asks for the next phase.

## Cross-Phase Requirements
These apply to every phase and are not repeated in each document:

- Standard library only (ADR-001).
- Tests written before implementation; `go test ./... -race` passes.
- `go build ./...`, `go vet ./...`, and `gofmt -l .` are clean.
- Every new file has a `Purpose` comment and a `docs/design/sourcemap.md` entry.
- Public API changes are recorded in `docs/design/ADR.md`.
- No secret, token, cookie, or authorization header is ever logged.
