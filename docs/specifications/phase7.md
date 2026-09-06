# Phase 7 — Observability

**Goal:** request tracing, in-process metrics, and structured logging in
`pkg/observability/`, wired in through the middleware chain.

## Scope
1. **Tracing** — assign or adopt a request ID and a trace context, propagate it through
   `context.Context`, and echo it on the response.
2. **Metrics** — count requests, record latency, and tally status codes, aggregated by
   route pattern and method.
3. **Logging** — structured request logs with a consistent field set.
4. **Integration** — expose each as middleware that composes with the Phase 3 chain.

## Out of Scope
- Exporting to Prometheus, OTLP, or any wire protocol (ADR-012).
- Sampling strategies and distributed trace propagation beyond a single header.
- Log shipping and rotation.

## Design Constraints
- Structured logging uses `log/slog`; injection for tests goes through a small interface,
  not a global (ADR-011).
- Metrics are held in memory behind an interface with a snapshot method. No network I/O in
  the request path.
- Metrics are keyed by **route pattern** (`/users/:id`), never by the concrete path
  (`/users/12345`), to avoid unbounded cardinality.
- Adopt an incoming request ID header when present and well-formed; otherwise generate one.
  Validate it — never echo an unbounded or unsanitized client-supplied value into logs or
  headers.
- Latency is measured with `time.Since` around the downstream handler, including the time
  spent in inner middleware.
- Counters are updated with `sync/atomic` or a mutex; the race detector must be clean.
- Never log request bodies, `Authorization` headers, `Cookie` headers, tokens, or query
  parameters known to carry secrets. Maintain an explicit redaction list.
- Observability failure must never fail a request: a logging or metrics error is swallowed
  and counted, not propagated.

## Standard Log Fields
Every request log line carries exactly these fields, in this order:

`timestamp`, `level`, `msg`, `request_id`, `method`, `route`, `path`, `status`,
`duration_ms`, `bytes`, `remote_addr`, `error` (omitted when nil).

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Request without a request ID header | One generated, present in the response and the log |
| Request with a valid request ID header | Adopted and echoed unchanged |
| Request with an over-long or invalid request ID | Replaced with a generated one |
| 200 and 500 responses | Both counted, tallied under their own status |
| Two paths matching one pattern | Aggregated under the same metric key |
| Handler panics | Latency and status still recorded (recovery runs inside the metrics middleware) |
| Concurrent requests | Counts are exact under `-race` |
| `Authorization` header present | Absent from every log line |

## Acceptance Criteria
- [ ] Request ID generation, adoption, validation, and echo are tested.
- [ ] Metrics snapshot returns accurate counts after a known sequence of requests.
- [ ] Latency is recorded for panicking requests too.
- [ ] Log output is asserted against the standard field set using a test handler.
- [ ] A test asserts that a request carrying an `Authorization` header produces no log line
      containing its value.
- [ ] Middleware ordering guidance is documented: request ID, then logging, then metrics,
      then recovery, then the handler.

## Test Plan
Record in `test/plans/phase7.md`. Capture log output with an in-memory `slog.Handler` rather
than parsing stdout.
