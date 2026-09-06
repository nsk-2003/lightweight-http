# Phase 2 — Core Router

**Goal:** a router in `pkg/router/router.go` that matches method and path, extracts path
parameters, exposes query values, and supports route groups.

## Scope
1. Register handlers for GET, POST, PUT, DELETE, and PATCH.
2. Match static path segments and `:name` path parameters (for example `/users/:id`).
3. Expose parsed path parameters and query string values to the handler.
4. Support route grouping with a shared path prefix (for example `/api/v1`), including
   nested groups.
5. Implement `http.Handler` so the router can be served by any `net/http` server.

## Out of Scope
- Middleware (Phase 3). The router must not import `pkg/middleware`.
- Typed body parsing and the structured response writer (Phase 4).
- Error response formatting beyond plain 404/405 defaults (Phase 5 replaces these).

## Design Constraints
- Use `net/http` and `net/url` only.
- Matching uses an explicit segment/trie matcher, not regular expressions (ADR-004).
- Matching is O(number of path segments); it must not backtrack.
- Path parameters are carried on the request `context.Context` with an unexported key type
  (ADR-005).
- Static segments win over parameter segments when both could match the same path.
- Registering the same method and pattern twice is a programming error: report it clearly
  at registration time rather than silently overwriting.
- The zero value of the router, or a documented constructor, must be usable without
  configuration.

## Behavioral Requirements
| Situation | Expected result |
|---|---|
| Path matches, method matches | Handler runs |
| Path matches, method does not | 405 with an `Allow` header listing registered methods |
| No path match | 404 |
| Trailing slash mismatch | Documented, consistent behavior; pick one and test it |
| Duplicate parameter names in one pattern | Registration error |
| Percent-encoded path segment | Decoded before parameter extraction |
| Repeated query key (`?a=1&a=2`) | Both values retrievable |

## Acceptance Criteria
- [ ] All five methods are registerable and dispatch correctly.
- [ ] Path parameters are extracted for single and multiple parameters in one pattern.
- [ ] Query values are readable, including repeated and empty values.
- [ ] Nested groups compose their prefixes correctly.
- [ ] 404 and 405 behave as specified, with the `Allow` header on 405.
- [ ] Table-driven tests cover every row in the table above.
- [ ] The router satisfies `http.Handler`, verified by a compile-time assertion.

## Test Plan
Record in `test/plans/phase2.md`. Use `net/http/httptest` for all handler tests. Include at
least one benchmark of a match against a routing table with several dozen routes.
