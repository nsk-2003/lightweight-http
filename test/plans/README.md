# Test Plans

One plan per phase, named `phaseN.md`. Write the plan **before** the implementation, then
fill in the results as you go. A plan is a record of what was verified and how, not a
restatement of the specification.

## Plan Template

```markdown
# Phase N Test Plan — <phase name>

## Scope
What behavior this plan covers, and what it explicitly does not.

## Cases
| # | Case | Type | Input / Setup | Expected | Result |
|---|---|---|---|---|---|
| 1 | | unit / integration / e2e / bench | | | pass / fail / deferred |

## Edge Cases Considered
Empty input, nil, zero values, boundary sizes, concurrency, client disconnect, oversized
payloads, malformed encoding.

## Not Covered
What is deliberately untested, and why.

## Evidence
gofmt -l .            -> (no output)
go build ./...        -> ok
go vet ./...          -> ok
go test ./... -race   -> ok, coverage NN.N%
```

## Rules
- Every acceptance criterion in the phase spec maps to at least one row in the Cases table.
- Every row of a Behavioral Requirements table in the phase spec maps to a case.
- A case marked `pass` must correspond to a test that was seen to fail before the
  implementation existed.
- Paste real command output in the Evidence block. Do not write it from memory.

## Conventions
- Package-local unit tests live beside the code as `*_test.go` (ADR-002).
- `test/unit/` holds black-box tests that must not see package internals.
- `test/testdata/` holds shared fixtures. Go tooling ignores directories named `testdata`.
- Handler tests use `net/http/httptest`; do not bind real ports in unit tests.
- Table-driven tests are the default style.
