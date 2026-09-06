# Phase 1 Test Plan — Project Setup

## Scope
Verifies the Go module is initialized with the correct module path and Go version directive,
all required directories exist, and the standard toolchain checks pass. No framework source
files exist at the end of this phase.

## Cases
| # | Case | Type | Input / Setup | Expected | Result |
|---|---|---|---|---|---|
| 1 | Toolchain version is 1.25 or later | e2e | `go version` | Version string contains go1.25 or higher | pass |
| 2 | Module path is `github.com/example/lightweight-http` | e2e | `go list -m all` | Exactly one line: `github.com/example/lightweight-http` | pass |
| 3 | `go.mod` has no third-party require entries | e2e | `cat go.mod` | No `require` block with external dependencies | pass |
| 4 | All seven required directories exist | e2e | `ls` of each dir | `cmd/`, `pkg/router/`, `pkg/middleware/`, `pkg/errors/`, `pkg/di/`, `pkg/observability/`, `examples/` all present | pass |
| 5 | `go build ./...` succeeds with no output | e2e | `go build ./...` | Exit 0, no output | pass |
| 6 | `go vet ./...` succeeds with no findings | e2e | `go vet ./...` | Exit 0, no output | pass |
| 7 | No new source files in sourcemap | review | `docs/design/sourcemap.md` | Sourcemap table still empty (no source files added this phase) | pass |

## Edge Cases Considered
- `go.sum` must not be created when there are no dependencies (ADR-001).
- Empty directories tracked only by `.gitkeep` are intentional and must remain.
- The `go` directive in `go.mod` must be at least 1.25; the working toolchain version
  (1.26.5) satisfies this.

## Not Covered
- Framework behavior (explicitly out of scope for Phase 1).
- Windows `environment.bat` (macOS-only environment).

## Notes
- `go vet ./...` exits 1 on modules with no packages; a root-level `doc.go` (package
  declaration + documentation only, no behavior) was added as the technical minimum to give
  the toolchain a package to operate on. This file is listed in `docs/design/sourcemap.md`.
- `go mod init` sets the `go` directive to the installed toolchain version (1.26.5), which
  satisfies the "Go 1.25 or later" requirement.
- No `go.sum` was created — the module has zero dependencies (ADR-001 satisfied).

## Evidence
```
go version                -> go version go1.26.5 darwin/arm64
go list -m all            -> github.com/example/lightweight-http
go build ./...            -> (no output, exit 0)
go vet ./...              -> (no output, exit 0)
gofmt -l .                -> (no output)
go test ./... -race -cover -> ? github.com/example/lightweight-http [no test files]
Full DoD chain exit code  -> 0

Directory listing (seven required dirs):
  /Users/nsk/lightweight-http/cmd
  /Users/nsk/lightweight-http/examples
  /Users/nsk/lightweight-http/pkg/di
  /Users/nsk/lightweight-http/pkg/errors
  /Users/nsk/lightweight-http/pkg/middleware
  /Users/nsk/lightweight-http/pkg/observability
  /Users/nsk/lightweight-http/pkg/router
```
