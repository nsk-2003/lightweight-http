---
name: go-quality-gate
description: The mandatory build, vet, format, and test gate for this Go project. Use after every code change and before claiming any phase, fix, or task is complete.
---

# Go Quality Gate

Never claim work is complete, compiling, or passing without running this gate in the
current session and showing the output.

## The Gate

Run in order. Stop at the first failure, fix it, then restart the gate from the top.

```bash
source ./environment.sh && gofmt -l .
source ./environment.sh && go build ./...
source ./environment.sh && go vet ./...
source ./environment.sh && go test ./... -race -cover
```

Expected results:

| Command | Pass condition |
|---|---|
| `gofmt -l .` | prints nothing |
| `go build ./...` | exit 0, no output |
| `go vet ./...` | exit 0, no findings |
| `go test ./... -race -cover` | all packages `ok` or `no test files` |

## Dependency Check

This project is standard-library only (ADR-001). After any change that touches `go.mod`:

```bash
source ./environment.sh && go mod tidy && go list -m all
```

`go list -m all` must print exactly one line: the main module. If a third-party module
appears, remove the import — do not vendor it and do not add it to `go.mod`.

## Fixing Failures

- `gofmt` findings: run `gofmt -w <file>`. Never hand-format.
- Build errors: fix the root cause. Do not comment out the caller or delete the test.
- `go vet` findings: treat every finding as a real bug until you can explain why it is not.
  If you must suppress one, explain why in your response, not in a code comment.
- Race detector findings: the race is real. Add proper synchronization; do not paper over
  it with a sleep or a channel that only widens the window.
- Test failures: read the failure, form a hypothesis, and confirm the hypothesis before
  changing code. Never adjust the assertion to match broken behavior.

## Reporting

Report the gate result as a short evidence block, for example:

```
gofmt -l .            -> (no output)
go build ./...        -> ok
go vet ./...          -> ok
go test ./... -race   -> ok  (router 0.4s, middleware 0.2s, ... coverage 82.1%)
```

If any step was not run, say so explicitly instead of implying the gate passed.
