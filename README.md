# lightweight-http (starter template)

A **structurally empty** starter template for building a lightweight HTTP framework in Go
with AI coding agents. It contains no implementation code — only the specifications,
architecture constraints, agent instructions, and workflow guidelines that an agent needs to
generate the implementation phase by phase.

The framework being specified: a minimal, production-ready HTTP framework using **only the
Go standard library**, demonstrating routing, middleware, request/response handling,
centralized errors, dependency injection, and observability.

> This README describes the template. Phase 8 replaces it with the README for the finished
> framework, per `docs/specifications/phase8.md`.

## What is in the box

| Path | What it holds |
|---|---|
| `AGENTS.md` | Agent entry point: objective, folder map, required reading, phase gating, Definition of Done |
| `.agents/workingrules.md` | Binding rules for code generation, Go conventions, security, review |
| `.agents/skills/` | `go-quality-gate` (build/vet/test gate) |
| `.agents/memory/` | Where the agent records durable notes between sessions |
| `docs/specifications/` | `specindex.md` plus one spec per phase, each with acceptance criteria |
| `docs/design/ARCHITECTURE.md` | Layering rules, package responsibilities, tech stack, build commands |
| `docs/design/ADR.md` | Twelve binding architecture decisions |
| `docs/design/sourcemap.md` | File-by-file purpose index, populated as code is generated |
| `docs/design/packagedesign.md` | Target dependency diagram in mermaid, plus how to verify it |
| `docs/devenv.md` | Development environment checklist and variables |
| `test/plans/README.md` | Test plan template and testing conventions |
| `cmd/`, `pkg/*`, `examples/`, `test/`, `build/` | Empty directories with `.gitkeep`, ready for generated code |

Deliberately absent: `go.mod`, `go.sum`, and every `.go` file. Creating them is Phase 1's job.

## How to use this template

1. Clone the repository and create a branch.
2. Review and adjust the constraints before generating anything:
   - `docs/design/ARCHITECTURE.md` — Go version, layering, build commands.
   - `docs/design/ADR.md` — the twelve decisions. Change them now, not mid-build.
   - `docs/specifications/*.md` — tighten any acceptance criterion you care about.
3. Set up the development environment:

```bash
source ./environment.sh && go version
```

4. Drive the build one phase at a time. Use prompts of this shape:

```
Read AGENTS.md and prepare the project for development.
```

```
Implement Phase 1 only, per docs/specifications/phase1.md. Do not generate anything extra.
```

```
Phase 1 is accepted. Implement Phase 2 only, per docs/specifications/phase2.md.
```

5. After each phase, review the diff and require the quality gate evidence before accepting:

```bash
source ./environment.sh && gofmt -l . && go build ./... && go vet ./... && go test ./... -race -cover
```

6. Repeat through Phase 8. Do not let the agent run ahead — the gating is the point.

## Phase sequence

| Phase | Spec | Delivers |
|---|---|---|
| 1 | `phase1.md` | Go module, directory layout, toolchain baseline |
| 2 | `phase2.md` | Router: methods, `:id` path params, query parsing, groups |
| 3 | `phase3.md` | Middleware chain, context propagation, panic recovery |
| 4 | `phase4.md` | Typed request parsing, structured response writer, content negotiation |
| 5 | `phase5.md` | Error type, status mapping, one JSON envelope, optional stack traces |
| 6 | `phase6.md` | DI container: singleton/scoped lifecycles, cycle detection |
| 7 | `phase7.md` | Tracing, in-process metrics, structured logging via `log/slog` |
| 8 | `phase8.md` | `cmd/server` wiring, runnable example, clean gate, final README |

Details and the enforced order are in `docs/specifications/specindex.md`.

## Non-negotiables

These are enforced by `.agents/workingrules.md` and `docs/design/ADR.md`:

- **Standard library only.** `go list -m all` must print exactly one module.
- **One phase at a time.** No pre-building later phases.
- **Tests before implementation**, and `go test -race` clean.
- **Evidence, not assertions.** A phase passes only when the gate output is shown.
- **Every file** gets a `Purpose` comment and a `sourcemap.md` entry.

## Adapting the template

Building something other than an HTTP framework? Keep `.agents/`, `docs/devenv.md`,
`docs/design/sourcemap.md`, and `test/plans/README.md` as they are; they are project-neutral.
Rewrite `AGENTS.md`'s objective and folder map, `ARCHITECTURE.md`, `ADR.md`, and the
`docs/specifications/` set for your domain, then reshape the empty directories to match.
