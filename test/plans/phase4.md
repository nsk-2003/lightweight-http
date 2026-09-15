# Phase 4 Test Plan — Request and Response Handling

## Scope
Typed request parsing (JSON body, form data, query parameters, path parameters) and a
structured response writer with content negotiation.

## Test Files
- `pkg/errors/errors_test.go` — unit tests for the HTTPError type and constructors.
- `pkg/router/request_test.go` — unit tests for ParseJSON, ParseForm, RequireQuery, QueryInt.
- `pkg/router/response_test.go` — unit tests for ResponseWriter tracking and write helpers.

## Test Fixtures
- `test/testdata/valid.json` — valid `{"name":"Alice","age":30}` payload.
- `test/testdata/malformed.json` — syntactically invalid JSON (`{invalid json`).
- `test/testdata/unknown_field.json` — valid JSON with an extra `unknown` field.
- Oversized body: generated inline in `TestParseJSONBodyTooLarge` using a 10-byte limit.

## Behavioral Requirements Coverage

| Situation | Test | Status |
|---|---|---|
| Valid JSON body matching the target type | `TestParseJSONSuccess` | ✓ |
| Malformed JSON → 400, no body echo | `TestParseJSONMalformed` | ✓ |
| Unknown field, strict mode → 400 naming the field | `TestParseJSONUnknownFieldStrict` | ✓ |
| Unknown field, permissive mode → success | `TestParseJSONUnknownFieldPermissive` | ✓ |
| Body exceeds limit → 413 | `TestParseJSONBodyTooLarge` | ✓ |
| Content-Type: application/xml → 415 | `TestParseJSONWrongContentType` | ✓ |
| Content-Type with charset param → accepted | `TestParseJSONContentTypeWithCharset` | ✓ |
| Form body URL-encoded → parsed | `TestParseFormURLEncoded` | ✓ |
| Form with wrong Content-Type → 415 | `TestParseFormWrongContentType` | ✓ |
| Missing required query parameter → 400 naming it | `TestRequireQueryMissing` | ✓ |
| Present-but-empty query value → success | `TestRequireQueryEmptyValue` | ✓ |
| Non-numeric integer parameter → 400 naming it | `TestQueryIntNonNumeric` | ✓ |
| Valid integer query parameter | `TestQueryIntSuccess` | ✓ |
| Missing integer parameter → 400 | `TestQueryIntMissing` | ✓ |
| Accept: application/json → JSON response | `TestRespondJSONAccept`, `TestWriteJSONAcceptsJSON` | ✓ |
| Accept: text/plain → Text response | `TestRespondTextAccept`, `TestWriteTextAcceptsPlain` | ✓ |
| Accept: */* → JSON default | `TestRespondWildcardDefaultsToJSON`, `TestWriteJSONWildcardAccept` | ✓ |
| Accept absent → JSON default | `TestRespondAbsentAcceptDefaultsToJSON`, `TestWriteJSONAbsentAccept` | ✓ |
| Unsupported Accept → 406, nothing written | `TestRespondUnsupportedAccept` | ✓ |
| 204 response → no body, no Content-Type | `TestNoContent` | ✓ |
| ResponseWriter records status code | `TestResponseWriterStatus`, `TestResponseWriterStatusDefault` | ✓ |
| ResponseWriter records byte count | `TestResponseWriterBytesWritten` | ✓ |
| WriteHeader called twice → no-op, status unchanged | `TestResponseWriterWriteHeaderOnlyOnce` | ✓ |
| Write without WriteHeader → implicit 200 | `TestResponseWriterWriteImpliesOK` | ✓ |

## Acceptance Criteria Verification
- [x] All four input sources (JSON body, form body, query string, path parameters) are
      parseable and tested. (Path param tests exist in router_test.go from Phase 2.)
- [x] Every row of the behavioral requirements table has a corresponding test.
- [x] ResponseWriter records status and byte count for Phase 7 metrics.
- [x] Body size limit is configurable (ParseOptions.MaxBodyBytes) and enforced by default
      (DefaultMaxBodyBytes = 1 MiB via http.MaxBytesReader).
- [x] No handler in the codebase writes error responses directly; all parse errors are
      returned as *httperr.HTTPError values.

## Notes
- `DisallowUnknownFields` defaults to true in ParseJSON (strict mode by spec).
- `negotiate` is unexported; it is tested indirectly through WriteJSON, WriteText, and Respond.
- The oversized-body test uses a synthetic 10-byte limit rather than a fixture file to keep
  the test fast and the repository free of large binary assets.
