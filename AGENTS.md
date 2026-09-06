# Lightweight HTTP Framework — Agent Guide

## Objective
Build a minimal, production-ready HTTP framework in Go, using **only the Go standard
library**, in phased increments. The framework demonstrates the core abstractions every
HTTP framework needs: routing, middleware, request/response handling, centralized errors,
dependency injection, and observability.

Module path: `github.com/example/lightweight-http`

## Working Rules
Read `.agents/workingrules.md` and strictly follow the rules. They override any habit or
default you may have.

## Project Folder Map
If a folder does not exist, create it as needed.

- `.agents/` : Coding agent configuration files, including SKILL files.
- `.agents/skills/` : Reusable skills the agent must load on demand.
- `.agents/memory/` : Durable notes the agent writes for its future self.
- `docs/` : Project documentation.
- `docs/devenv.md` : Instructions to set up the development environment for a new developer.
- `docs/specifications/` : Requirements and specs.
- `docs/specifications/specindex.md` : Index of specification documents and the phase order.
- `docs/design/` : Architecture and design decisions.
- `docs/design/ARCHITECTURE.md` : Architecture rules and constraints.
- `docs/design/ADR.md` : Architecture decision records.
- `docs/design/sourcemap.md` : Every source file and its purpose.
- `docs/design/packagedesign.md` : Package/module dependency diagram (mermaid).
- `build/` : Build tool configuration and temporary build artifacts.
- `cmd/` : Executable entry points. One subfolder per binary.
- `pkg/` : The framework packages. Each subfolder is a package with a single responsibility.
- `pkg/router/` : HTTP routing, path parameters, route groups.
- `pkg/middleware/` : Composable middleware chain and built-in middleware.
- `pkg/errors/` : Error types, HTTP status mapping, error serialization.
- `pkg/di/` : Dependency injection container.
- `pkg/observability/` : Tracing, metrics, structured logging.
- `examples/` : Runnable examples that exercise the framework as a real user would.
- `test/` : Test plans, test data, and tests that do not belong next to a package.
- `test/plans/` : Test plans, one per phase.
- `test/testdata/` : Fixtures shared by tests.
- `test/unit/` : Cross-package or black-box tests. Package-local unit tests stay beside
  the code as `*_test.go` (see ADR-002).

## Required Reading Before Making Any Changes
- Read the agent configuration files in `.agents/`.
- Read SKILL files as needed from `.agents/skills/`.
- Read your memories from `.agents/memory/`.
- Read `docs/specifications/specindex.md`, then all relevant specification files.
- Follow `docs/design/ARCHITECTURE.md`.
- Follow the decisions in `docs/design/ADR.md`.
- Consult `docs/design/sourcemap.md` to decide which existing files to modify.
- Store your memories in `.agents/memory/`.
- Generate or modify files only when asked.

## Phase Discipline
The work is split into eight phases, specified in `docs/specifications/`. The phases are
sequential and gated:

1. Implement only the phase you were asked for. Do not start the next phase until the
   current one is complete and the user explicitly asks for the next.
2. A phase is complete only when its Definition of Done (below) is satisfied.
3. If a later phase's need forces a change to an earlier phase's public API, stop and ask.

## Definition of Done (every phase)
A phase is done when all of the following hold, verified by running the commands — not by
inspection:

- [ ] Every acceptance criterion in the phase spec is met.
- [ ] `go build ./...` succeeds with no output.
- [ ] `go vet ./...` succeeds with no findings.
- [ ] `gofmt -l .` prints nothing.
- [ ] `go test ./...` passes, and new behavior has tests written before the implementation.
- [ ] `go.mod` lists zero third-party requirements.
- [ ] Every new file has a `Purpose` comment and an entry in `docs/design/sourcemap.md`.
- [ ] `docs/design/packagedesign.md` reflects the real dependency graph.
- [ ] The phase's test plan in `test/plans/` is updated with what was tested.

Report the command output as evidence. Never claim a phase passes without having run the
commands in this session.

## Quick Command Reference
Always load the environment first (see `docs/devenv.md`):

```bash
source ./environment.sh && go build ./...
source ./environment.sh && go vet ./...
source ./environment.sh && gofmt -l .
source ./environment.sh && go test ./... -race -cover
```
