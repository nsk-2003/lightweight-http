# Phase 1 — Project Setup

**Goal:** a compilable, empty Go module with the agreed directory layout and toolchain
baseline. No framework behavior yet.

## Scope
1. Initialize the Go module:
   ```bash
   go mod init github.com/example/lightweight-http
   ```
2. Declare Go 1.25 or later in `go.mod`.
3. Ensure the directory structure exists: `cmd/`, `pkg/router/`, `pkg/middleware/`,
   `pkg/errors/`, `pkg/di/`, `pkg/observability/`, `examples/`.
4. Create `environment.sh` (and `environment.bat` on Windows) per `docs/devenv.md` if it is
   not already present locally.
5. Verify the toolchain: `go version` reports 1.25 or later.

## Out of Scope
- Any framework code. Do not create router, middleware, error, DI, or observability source
  files in this phase.
- Any `main` function beyond what Phase 8 specifies.

## Design Constraints
- `go.mod` must contain no `require` entries for third-party modules (ADR-001).
- A `go.sum` is only created if the module graph requires it. With zero dependencies, no
  `go.sum` is expected; do not fabricate one.
- Empty directories are not tracked by git. Keep the `.gitkeep` placeholder in a directory
  until real source files land there, then remove it.

## Acceptance Criteria
- [ ] `go.mod` exists with module path `github.com/example/lightweight-http` and a `go`
      directive of 1.25 or later.
- [ ] `go list -m all` prints exactly one module: the main module.
- [ ] All seven required directories exist.
- [ ] `go build ./...` succeeds (it will be a no-op with no packages).
- [ ] `go vet ./...` succeeds.
- [ ] `docs/design/sourcemap.md` has been reviewed; no new source files were added.

## Test Plan
Record in `test/plans/phase1.md`: the toolchain version observed, the output of
`go list -m all`, and the directory listing.
