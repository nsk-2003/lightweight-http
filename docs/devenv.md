# Development Environment Setup

Use this checklist for onboarding a new developer or starting a fresh agent session.

1. Review the tech stack in `docs/design/ARCHITECTURE.md`.
2. Verify the required tools are installed:
   - Go 1.25 or later — `go version`
   - `gofmt` (ships with Go) — `gofmt -l .`
   - git
3. If anything is missing:
   - Ask whether it is already installed and where.
   - If not installed, guide the setup of the required software.
4. Load the environment. A generic `environment.sh` is provided and tracked; it derives all
   paths from its own location, so it works unmodified on any machine. Windows users create
   an equivalent `environment.bat`.
5. Ensure the environment file:
   - Adds the required tool paths (`$GOROOT/bin`, `$GOBIN`).
   - Sets the required environment variables (see the table below).
   - Defines the repository paths (root, source, test).
   - Prints a confirmation line so you can see it ran.
6. Keep machine-specific values out of the tracked `environment.sh`. Put them in
   `environment.local.sh`, which is ignored by git, and source it from your shell profile.
   Never commit a file containing credentials.

## Environment Variables

| Variable | Purpose | Default |
|---|---|---|
| `LWHTTP_ROOT` | Repository root | derived from the script location |
| `LWHTTP_PORT` | Port the example server listens on | `8080` |
| `LWHTTP_LOG_LEVEL` | `debug`, `info`, `warn`, `error` | `info` |
| `LWHTTP_DEBUG` | Enables stack capture in errors (never in production) | `false` |
| `CGO_ENABLED` | `0` for a static binary | `0` |

Secrets are never hardcoded and never committed. If the project ever needs one, read it
from the environment and document the variable here.

## Running Commands

Always chain the environment script before a command so paths and variables are set:

```bash
source ./environment.sh && go build ./...
source ./environment.sh && go test ./... -race -cover
source ./environment.sh && go run ./cmd/server
```

## Verifying the Setup

```bash
source ./environment.sh && go version        # 1.25 or later
source ./environment.sh && go env GOMODCACHE # writable path
source ./environment.sh && go list -m all    # only the main module
```

If `go list -m all` prints more than the main module, a third-party dependency crept in.
Remove it — the project is standard-library only (ADR-001).

## Editor Setup (optional)
- Enable format-on-save with `gofmt` or `goimports`.
- Enable `go vet` on save if your editor supports it.
- Turn off any tool that auto-adds imports from third-party modules.
