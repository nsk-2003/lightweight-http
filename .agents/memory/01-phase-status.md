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
