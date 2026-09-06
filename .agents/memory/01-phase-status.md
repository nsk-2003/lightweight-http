## 2026-09-07 — Phase 7 complete

Phase 7 (Observability) is complete and all DoD criteria verified.
Key decisions:
- `pkg/observability` at layer 3. Imports `pkg/errors` (request-ID helpers) and `pkg/router` (RoutePatternHolder). Layer 3 → Layer 2 is allowed. Package diagram updated.
- `RoutePatternHolder`: mutable `*struct{Pattern string}` installed in context by Logger (or Metrics standalone). Router calls `setRoutePattern(ctx, pattern)` which mutates it through the context chain. Both Logger and Metrics read the final value after `ServeHTTP` returns — no extra context write needed.
- `trieNode.pattern` field: set once at registration to the canonical pattern string (e.g. `/users/:id`). `matchResult` carries it; 404/405 paths leave the holder at `""`.
- `RequestID` middleware: validates incoming X-Request-ID (non-empty, ≤128 bytes, 0x21–0x7E only). Generates 32-char hex (16 bytes crypto/rand). Stores via `pkg/errors.WithRequestID`; echoes on `X-Request-ID` response header.
- `Logger` middleware: installs RoutePatternHolder, wraps ResponseWriter via `router.WrapResponseWriter`, logs after downstream returns. Standard fields: request_id, method, route, path, status, duration_ms, bytes, remote_addr. Authorization/Cookie never logged (structural: no code path reads headers).
- `Recorder` (Metrics): `sync.Mutex`-guarded `map[MetricKey]*routeEntry`. Thin `metricsRW` wrapper captures first status code. `Snapshot()` returns deep copy. Metrics wraps Recovery to see the 500 written by Recovery on panic.
- Recommended ordering: RequestID → Logger → Metrics → Recovery → Handler. Documented in test plan and observability.go godoc.
- Zero-value `Recorder` is ready to use (no constructor required).
Pointers: `pkg/observability/{observability,observability_test}.go`, `pkg/router/{context,trie,router}.go`, `test/plans/phase7.md`.

## 2026-09-07 — Phase 6 complete

Phase 6 (Dependency Management) is complete and all DoD criteria verified.
Key decisions:
- `pkg/di` at layer 2 (core). Imports only stdlib: `context`, `sync`, `log/slog`, `fmt`, `strings`. Does NOT import `pkg/errors` (DI errors are not HTTP errors).
- Key type: `string`. Duplicate registration → error (no silent last-wins).
- `Constructor func(r Resolver) (any, error)` — Resolver interface lets constructors resolve dependencies without holding the Container.
- Singleton lifecycle: `sync.Once` + cached `value`/`err`. Constructor called at most once even under 100-goroutine concurrent resolution. Failed constructor caches the error permanently.
- Scoped lifecycle: per-key `*scopedEntry` (with its own `sync.Once`) stored in `sync.Map`. `LoadOrStore` races safely; prevents scope-wide lock during construction; prevents deadlock when a scoped service's constructor resolves another scoped service.
- Cycle detection: `[]string` path threaded through `internalResolver.Resolve` → `container.resolve`. Cycle yields `"A -> B -> C -> A"` via `strings.Join`.
- Disposal: `Scope` maintains `[]disposableEntry` appended in construction order. `Dispose()` snapshots under `dispMu` then iterates in reverse. Errors logged via `slog.Default().Error`, loop continues.
- Context integration (ADR-005): `scopeKey{}` unexported type; `ContextWithScope` + `ScopeFromContext`.
- `Scope.Resolve(key)` delegates to `container.resolve(s, key, nil)`, so singletons are served from the parent container.
- Scoped service resolved from root → descriptive error naming the key.
Pointers: `pkg/di/{container,container_test}.go`, `test/plans/phase6.md`.

## 2026-09-07 — Phase 5 complete

Phase 5 (Centralized Error Handling) is complete and all DoD criteria verified.
Key decisions:
- `HTTPError` struct extended with `ErrCode string` (machine-readable) and `details []Detail` (unexported, accessed via `WithDetails`). `Code int` (HTTP status) retained from Phase 4 for backward compatibility.
- Nine sentinel vars: `ErrBadRequest…ErrInternal`. Each created with `NewCoded(status, errCode, message)`. Sentinels are pointers; `errors.Is` through two `fmt.Errorf` wraps works because `fmt.Errorf("%w")` chains `Unwrap()`.
- `WithDetails(details ...Detail)` copies the receiver (struct literal copy) so sentinels cannot be mutated.
- JSON wire format: `{"error":{"code","message","status","request_id","details"}}`. `details` and `request_id` are `omitempty`. Top-level has exactly one key.
- `Handler{log, debug bool}` + `ServeError(w, r, err)` — single translation point. Cause is logged but never in response body. In debug mode, `runtime/debug.Stack()` is logged too.
- `WriteError(w, r, err)` convenience wrapper using `slog.Default()` + production mode; used by router for 404/405.
- `requestIDKey{}` (unexported) + `WithRequestID` + `RequestIDFromContext` in `pkg/errors` (ADR-005).
- `pkg/middleware` now imports `pkg/errors`. `Recovery` signature changed to `Recovery(log, debug bool)`. Uses `ServeError` for panic responses; logs panic + optional stack in Recovery defer, passes bare 500 to ServeError (no double-logging).
- Router 404 → `WriteError(w, req, ErrNotFound)`. Router 405 → `WriteError(w, req, NewCoded(405, "method_not_allowed", …))` after setting Allow header.
- Security invariant: tests assert client body never contains "goroutine" or "github.com/example/lightweight-http", in both production and debug modes.
Pointers: `pkg/errors/{errors,errors_test}.go`, `pkg/middleware/{middleware,middleware_test}.go`, `pkg/router/{router,router_test}.go`, `test/plans/phase5.md`.

## 2026-09-07 — Phase 4 complete

Phase 4 (Request and Response Handling) is complete and all DoD criteria verified.
Key decisions:
- `pkg/errors` bootstrapped with `HTTPError` type (Code + public Message + optional wrapped cause), `New`, `Wrap`, `CodeOf`, `MessageOf`. Phase 5 adds JSON wire rendering.
- `BindJSON` uses `http.MaxBytesReader` + `json.Decoder.DisallowUnknownFields` (configurable). Body limit configurable via `WithBodyLimit`; default 1 MiB. Content-Type enforced (absent = accepted, wrong = 415). Oversized → 413. Malformed → 400 without echoing raw body.
- `BindForm`/`FormParam`/`FormParamInt` handle `application/x-www-form-urlencoded` with same body limit + content-type checks.
- `QueryParam`/`QueryParamInt` use `url.Values.Has` (Go 1.17+) so `?k=` (empty value) is accepted; missing key → 400 naming the param.
- `PathParam` reads from context via existing `Params(ctx)`; absent param → 400.
- `ResponseWriter` in `pkg/router` tracks status (0 until first write) and byte count for Phase 7. Second `WriteHeader` call logs Warn and no-ops.
- `JSON`, `Text`, `NoContent`, `Respond` helpers. `Respond` parses comma-separated Accept; */* and absent → JSON default; unsupported → 406 HTTPError (no body written).
- `pkg/router` now imports `pkg/errors` and `log/slog`. Package design diagram updated.
- `noopRW` is an unexported struct implementing `http.ResponseWriter` used as the required arg to `http.MaxBytesReader` when no real ResponseWriter is in scope.
Pointers: `pkg/errors/{errors,errors_test}.go`, `pkg/router/{request,request_test,response,response_test}.go`, `test/plans/phase4.md`.

## 2026-09-07 — Phase 3 complete

Phase 3 (Middleware System) is complete and all DoD criteria verified.
Key decisions:
- `Middleware` type = `func(http.Handler) http.Handler`; `Chain` iterates in reverse so the first arg is outermost (ADR-009).
- `Recovery` wraps `http.ResponseWriter` in a thin `responseWriter` struct to track whether the header was already committed before a panic; if so, it logs and returns without writing a 500.
- `Recovery` re-panics on `http.ErrAbortHandler` to preserve server abort behavior (ADR-007).
- `pkg/middleware` imports only `net/http` and `log/slog`; it does not import `pkg/router` or `pkg/errors` in Phase 3.
- Group chain composition is achieved by nesting: `global(group(handler))` → A→B→C→handler order.
Pointers: `pkg/middleware/middleware.go`, `pkg/middleware/middleware_test.go`, `test/plans/phase3.md`.

## 2026-09-06 — Phase 2 complete

Phase 2 (Core Router) is complete and all DoD criteria verified.
Key decisions:
- Trie-based O(segments) matcher in `pkg/router/trie.go`; static wins over param, no backtracking (ADR-004).
- Path parameters in context via unexported `paramsKey{}` (ADR-005).
- Trailing slash is a **distinct** pattern; `/users` and `/users/` are different routes.
- Conflicting param names at the same trie position across different routes are reported as a registration error.
- `Router` and `Group` both return errors from registration methods (no panics).
- `pkg/router` imports only standard library: `net/http`, `net/url`, `context`, `sync`.
Pointers: `pkg/router/{router,trie,context}.go`, `pkg/router/router_test.go`, `test/plans/phase2.md`.

## 2026-09-06 — Phase 1 complete

Phase 1 (Project Setup) is complete and all DoD criteria verified.
Key decision: `doc.go` added at module root (package declaration + doc comment only) so
`go vet ./...` has a package to target in the empty module. Listed in sourcemap.md.
Pointer: doc.go, test/plans/phase1.md.
