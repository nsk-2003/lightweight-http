# Phase 4 — Request and Response Handling

**Goal:** typed request parsing and a structured response writer, so handlers work with
values rather than raw streams.

## Scope
1. Typed request parsing from four sources: JSON body, form body, query string, and path
   parameters.
2. A structured response writer supporting status codes, headers, and JSON or plain-text
   payloads.
3. Content negotiation based on the `Accept` and `Content-Type` headers.
4. Error serialization hand-off: parsing failures produce framework errors, not ad-hoc
   responses.

## Out of Scope
- The error envelope itself (Phase 5) — this phase returns errors; Phase 5 renders them.
- Streaming responses, file uploads beyond basic multipart form values, and templating.

## Design Constraints
- Bodies are read through `http.MaxBytesReader` with a configurable limit; the default must
  be finite. An oversized body is a 413, not an out-of-memory.
- JSON decoding uses `encoding/json` with `DisallowUnknownFields` configurable, defaulting
  to strict.
- Reject a body whose `Content-Type` does not match the parser being used.
- Never read the body twice. If a parser consumes it, say so in the doc comment.
- Query and form values are explicitly converted to their target types with clear errors on
  failure; no silent zero values.
- The response writer must write the status code exactly once and must be safe to call
  after the header is written (it logs and no-ops rather than panicking).
- Set `Content-Type` on every response body the framework writes.
- Handlers return an error rather than writing an error response themselves (ADR-006).

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Valid JSON body matching the target type | Parsed successfully |
| Malformed JSON | 400 with a message that does not echo the raw body |
| Unknown field, strict mode | 400 naming the offending field |
| Body exceeds the limit | 413 |
| `Content-Type: application/xml` to a JSON parser | 415 |
| Missing required query parameter | 400 naming the parameter |
| Non-numeric value for an integer parameter | 400 naming the parameter |
| `Accept: application/json` | JSON response |
| `Accept: text/plain` | Text response |
| `Accept: */*` or absent | JSON default |
| Unsupported `Accept` | 406 |
| 204 response | No body, no `Content-Type` |

## Acceptance Criteria
- [ ] All four input sources are parseable and tested.
- [ ] Every row of the behavior table has a test.
- [ ] The response writer records the status and byte count so Phase 7 metrics can read them.
- [ ] No handler in the codebase writes an error response directly.
- [ ] Body size limit is configurable and enforced by default.

## Test Plan
Record in `test/plans/phase4.md`. Include fixtures in `test/testdata/` for valid, malformed,
and oversized payloads.
