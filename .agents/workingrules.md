# Working Rules that Coding Agents must follow

## Working Rules
- Make small, focused changes.
- Preserve behavior unless the task explicitly requires a change.
- Do not change public APIs unless instructed.
- Follow existing project patterns before introducing new ones.
- Write the unit tests before making any changes.
- Run the unit tests and ensure that all tests are passing after making any changes.
- Implement one phase at a time. Do not pre-build features from a later phase.
- Do not overwrite any file in `docs/`.
- Modify files in `docs/` only when explicitly told by the user. Do not modify on your own.
  The two exceptions, which you must always keep current, are `docs/design/sourcemap.md`
  and `docs/design/packagedesign.md`.
- Store your memories in `.agents/memory/`.

## Do Not
- Implement anything that conflicts with approved specs or architecture decisions.
- Add a third-party dependency. This project is standard-library only (ADR-001).
- Rewrite large parts of the codebase unless explicitly asked.
- Reformat unrelated files.
- Remove TODOs/comments without addressing their intent.
- Assume undocumented behavior is safe to change.
- Generate or modify files that the user did not explicitly ask for.
- Claim that a build, vet, or test step passed without having run it in this session.

## When Unsure
- Ask for clarification instead of guessing.
- Briefly state trade-offs in review notes.
- If working rules conflict with explicit user instructions, ask for clarification, then do
  as the user instructs.

## Code Generation
- Add a `Purpose` comment at the top of every new file.
- Add an entry for every new source file in `docs/design/sourcemap.md`. The entry must
  contain the file path relative to the project root and the purpose of the file.
- Use `sourcemap.md` to decide which existing files to modify.
- Analyze changes in the specification and design documents using the version control diff
  command, then update the code to match the differences.

## Go-Specific Rules
- Target the Go version declared in `docs/design/ARCHITECTURE.md`. Do not raise it.
- Standard library only. `go.mod` must have an empty `require` block; there must be no
  `go.sum` entries for third-party modules.
- Exported identifiers carry doc comments that begin with the identifier name.
- Return errors; do not panic in library code. The one permitted panic-handling site is the
  recovery middleware, which converts a panic into a framework error (Phase 5).
- Wrap errors with `fmt.Errorf("...: %w", err)` so callers can use `errors.Is`/`errors.As`.
- Accept interfaces, return concrete types.
- Every exported function that can block takes a `context.Context` as its first parameter.
- Guard shared mutable state with `sync` primitives and verify with `go test -race`.
- Do not use `interface{}`/`any` where a generic type parameter or concrete type works.
- Keep `init()` functions out of the codebase; wire things explicitly.

## Security Rules
- Never interpolate request-derived values into file paths, shell commands, or SQL.
- Validate and bound all request input: body size limits, content-type checks, and explicit
  parsing of path and query parameters.
- Do not log secrets, credentials, tokens, cookies, or authorization headers.
- Error responses returned to clients must not leak stack traces or internal paths unless
  the framework is explicitly in debug mode (Phase 5).
- Read configuration and secrets from environment variables; never hardcode them.
- Set server timeouts (`ReadHeaderTimeout`, `ReadTimeout`, `WriteTimeout`, `IdleTimeout`)
  on every `http.Server` you construct.

## Code Review
- Validate compliance with project guidelines.
- Re-run the quality gate (`.agents/skills/go-quality-gate/SKILL.md`) as part of review.

## Executing Commands with Environment Configuration

Always chain the environment setup script before executing commands so required variables
and paths are configured.

### Unix/Linux/macOS (Bash/Zsh)
```bash
# Basic command execution with environment setup
source ./environment.sh && command-here

# Examples:
source ./environment.sh && go build ./...
source ./environment.sh && go test ./... -race -cover
source ./environment.sh && go run ./cmd/server
```

### Windows (PowerShell)
```powershell
# Basic command execution with environment setup
.\environment.bat && command-here

# Examples:
.\environment.bat && go build ./...
.\environment.bat && go vet ./...
```

### Best Practices
- **Always chain before executing**: use `&&` so the environment loads before the command runs.
- **Verify environment**: the script prints a confirmation line (e.g. "Environment configured").
- **Project root**: run the script from the repository root.
- **CI/CD pipelines**: include the environment chaining step in all build and test scripts.
