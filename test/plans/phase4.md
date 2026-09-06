# Phase 4 — Request and Response Handling: Test Plan

## Scope

Tests for `pkg/errors` and the new request/response helpers in `pkg/router` covering all
behavioral requirements from `docs/specifications/phase4.md`. All tests use
`net/http/httptest`; no real ports are bound.

## Test files

- `pkg/errors/errors_test.go` (package `errors_test` — black-box)
- `pkg/router/request_test.go` (package `router_test` — black-box)
- `pkg/router/response_test.go` (package `router_test` — black-box)

## Test fixtures

Payloads are constructed inline in tests using `strings.NewReader`; no separate fixture
files are needed for Phase 4's scope.

## pkg/errors tests

| Test | Behavioral requirement covered |
|---|---|
| `TestHTTPError_Error_NoWrap` | `Error()` returns `Message` when no cause is wrapped |
| `TestHTTPError_Error_WithWrap` | `Error()` is non-empty and `Message` is accessible when cause is present |
| `TestHTTPError_Unwrap_NewHasNoCause` | `New` produces no wrapped cause |
| `TestHTTPError_Unwrap_WrapPreservesCause` | `Wrap` makes the cause reachable via `errors.Is` |
| `TestNew_SetsCodeAndMessage` | `New` sets `Code` and `Message` fields correctly |
| `TestWrap_SetsCodeMessageAndCause` | `Wrap` sets all three fields and cause is traversable |
| `TestCodeOf_HTTPError` | `CodeOf` returns the embedded status code for `*HTTPError` |
| `TestCodeOf_WrappedHTTPError` | `CodeOf` works on a directly-inspected `*HTTPError` |
| `TestCodeOf_PlainError_Returns500` | `CodeOf` returns 500 for plain errors |
| `TestCodeOf_Nil_Returns0` | `CodeOf` returns 0 for nil |
| `TestMessageOf_HTTPError` | `MessageOf` returns the public message |
| `TestMessageOf_PlainError_ReturnsGeneric` | `MessageOf` returns "Internal Server Error" for plain errors without leaking internals |
| `TestMessageOf_Nil_ReturnsEmpty` | `MessageOf` returns empty string for nil |

## pkg/router request parsing tests

| Test | Phase 4 behavior-table row covered |
|---|---|
| `TestBindJSON_ValidBody_ParsedSuccessfully` | Valid JSON body matching the target type → parsed successfully |
| `TestBindJSON_MalformedJSON_Returns400` | Malformed JSON → 400, message does not echo raw body |
| `TestBindJSON_UnknownField_StrictMode_Returns400` | Unknown field, strict mode → 400 naming the offending field |
| `TestBindJSON_UnknownField_LenientMode_Succeeds` | Unknown field, lenient mode → parsed successfully |
| `TestBindJSON_BodyExceedsLimit_Returns413` | Body exceeds limit → 413 |
| `TestBindJSON_WrongContentType_Returns415` | `Content-Type: application/xml` to JSON parser → 415 |
| `TestBindJSON_NoContentType_Succeeds` | Absent Content-Type → accepted (not treated as wrong type) |
| `TestBindJSON_CustomBodyLimit_Enforced` | Body just over custom limit → 413 |
| `TestBindJSON_DefaultBodyLimitFinite` | `DefaultBodyLimit` is a positive finite value |
| `TestBindForm_ValidBody_ParsedSuccessfully` | Form source parseable and testable |
| `TestBindForm_WrongContentType_Returns415` | Wrong Content-Type for form → 415 |
| `TestFormParam_Missing_Returns400NamingParam` | Missing form parameter → 400 naming the parameter |
| `TestFormParamInt_Valid` | Integer form parameter parsed correctly |
| `TestFormParamInt_NonNumeric_Returns400NamingParam` | Non-numeric form integer → 400 naming the parameter |
| `TestQueryParam_Present_ReturnsValue` | Query source parseable and testable |
| `TestQueryParam_Missing_Returns400NamingParam` | Missing required query parameter → 400 naming the parameter |
| `TestQueryParam_EmptyValue_Succeeds` | `?k=` (key present with empty value) is not treated as missing |
| `TestQueryParamInt_Valid_ReturnsInt` | Integer query parameter parsed correctly |
| `TestQueryParamInt_Missing_Returns400` | Missing integer query parameter → 400 |
| `TestQueryParamInt_NonNumeric_Returns400NamingParam` | Non-numeric value for integer parameter → 400 naming the parameter |
| `TestPathParam_Present_ReturnsValue` | Path parameter source parseable and testable |
| `TestPathParam_Absent_Returns400` | Absent path parameter (route/handler mismatch) → 400 |

## pkg/router response writer tests

| Test | Phase 4 behavior-table row covered |
|---|---|
| `TestResponseWriter_StatusTrackedAfterWriteHeader` | Response writer records status code |
| `TestResponseWriter_StatusImplicit200OnWrite` | Implicit 200 status recorded when body written without WriteHeader |
| `TestResponseWriter_BytesWrittenTracked` | Response writer records byte count for Phase 7 metrics |
| `TestResponseWriter_DoubleWriteHeader_SecondIsNoOp` | Second WriteHeader call is no-op (safe after header written) |
| `TestResponseWriter_ZeroStatus_BeforeAnyWrite` | Status is 0 before any write |
| `TestResponseWriter_DelegatesUnderlyingWriter` | ResponseWriter delegates to the underlying writer correctly |
| `TestJSON_SetsContentType` | JSON response sets Content-Type: application/json |
| `TestJSON_WritesEncodedBody` | JSON response body is valid JSON |
| `TestJSON_WritesStatus` | JSON response writes the given status code |
| `TestText_SetsContentType` | Text response sets Content-Type: text/plain |
| `TestText_WritesBody` | Text response writes the body string |
| `TestNoContent_Status204_NoBody_NoContentType` | 204 response → no body, no Content-Type |
| `TestRespond_AcceptApplicationJSON_WritesJSON` | `Accept: application/json` → JSON response |
| `TestRespond_AcceptTextPlain_WritesText` | `Accept: text/plain` → text response |
| `TestRespond_AcceptStar_DefaultsToJSON` | `Accept: */*` → JSON default |
| `TestRespond_NoAcceptHeader_DefaultsToJSON` | Absent Accept header → JSON default |
| `TestRespond_UnsupportedAccept_Returns406_WritesNothing` | Unsupported Accept → 406 HTTPError, no body written |
| `TestRespond_AcceptMultiple_PicksFirstSupported` | Multi-value Accept picks first supported type |

## Key design decisions

### Error representation
Parsing failures return `*errors.HTTPError` (from the new `pkg/errors` package) carrying
the HTTP status code and a safe public message. The router helpers never write error
responses themselves (ADR-006); handlers return the error and Phase 5 renders it.

### Body size limit
`http.MaxBytesReader` wraps `r.Body` before decoding. A `*http.MaxBytesError` from the
decoder is mapped to a 413 response error. The limit is configurable via `WithBodyLimit`.
The default (`DefaultBodyLimit = 1 MiB`) is always finite.

### Content-Type enforcement
`BindJSON` rejects non-`application/json` Content-Type with 415. An absent Content-Type
is accepted (many clients omit it). `BindForm` similarly rejects non-form content types.

### Strict JSON mode
`json.Decoder.DisallowUnknownFields` is enabled by default. Unknown fields produce a
400 error whose message names the offending field (from the decoder's own error string,
which is safe to forward). Opt out with `WithStrictJSON(false)`.

### Safe error messages
`sanitizeJSONError` in `request.go` ensures that decoder error messages forwarded to
clients name fields or types, never raw body content.

### Content negotiation
`Respond` parses `Accept` comma-separated tokens and picks the first supported type.
`application/json` and `*/*` map to JSON; `text/plain` maps to plain text. Unrecognised
types return a 406 HTTPError without writing to the response.

### ResponseWriter
`WrapResponseWriter` wraps any `http.ResponseWriter`. It tracks status (0 until first
write, 200 implied by `Write` without `WriteHeader`) and cumulative bytes for Phase 7
metrics. A second `WriteHeader` call logs a Warn-level message and is otherwise ignored.

## Coverage

- `pkg/errors`: 100% statement coverage
- `pkg/router`: 88.1% statement coverage (includes Phase 2 and Phase 3 trie/context code)

## Commands run to verify

```
source ./environment.sh && go build ./... && go vet ./... && gofmt -l . && go test ./... -race -cover
```

All checks clean; all tests passed with no race conditions detected.
