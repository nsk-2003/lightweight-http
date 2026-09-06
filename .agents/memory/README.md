# Agent Memory

Durable notes an agent writes for its future self. Read every file here before starting
work; append to it when you learn something a future session would otherwise rediscover
the hard way.

## What belongs here
- Decisions the user made in chat that are not yet captured in `docs/design/ADR.md`.
- Non-obvious constraints discovered while building (a stdlib API that does not behave as
  documented, a platform quirk, a flaky test and its cause).
- The current phase status: what is finished, what is explicitly deferred.
- Dead ends: approaches that were tried and rejected, and why.

## What does not belong here
- Anything that belongs in a spec, an ADR, or the source map. Put it there instead and note
  the pointer here.
- Secrets, tokens, credentials, or copies of environment files.
- Large code dumps. Reference the file path and line range instead.

## Format
One markdown file per topic, named `NN-topic.md` (for example `01-phase-status.md`).
Newest entry at the top of the file, each entry stamped with the date:

```markdown
## 2026-01-15 — Router path parameter matching
Observation, decision, or dead end in two to five lines.
Pointer: pkg/router/router.go, docs/design/ADR.md ADR-004
```

Keep each entry short enough to be worth re-reading at the start of every session.
