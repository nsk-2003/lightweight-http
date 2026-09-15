// Purpose: HTTPError type, sentinel errors, constructor functions, and request-ID context
// helpers; implements the one-error-one-wire-format constraint (ADR-006).

package errors

import (
	"context"
	"fmt"
	"net/http"
)

// Detail is a single structured entry in an error's optional field-level details list.
// It is serialized into the "details" array of the error envelope.
type Detail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// HTTPError is the single error type used throughout the framework.
// Handlers return HTTPError values; Handle renders them as a JSON envelope.
// Cause is never serialized to the client (ADR-008).
type HTTPError struct {
	// Status is the HTTP status code sent to the client.
	Status int
	// Code is the stable machine-readable error identifier, e.g. "not_found".
	Code string
	// Message is a client-safe human-readable description.
	Message string
	// Cause wraps the underlying error; it is logged but never sent to the client.
	Cause error
	// Details holds optional field-level structured information.
	Details []Detail
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("HTTP %d %s: %s: %v", e.Status, e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("HTTP %d %s: %s", e.Status, e.Code, e.Message)
}

// Unwrap returns the wrapped Cause, enabling errors.Is and errors.As to traverse the chain.
func (e *HTTPError) Unwrap() error {
	return e.Cause
}

// StatusCode returns the HTTP status code associated with this error.
func (e *HTTPError) StatusCode() int {
	return e.Status
}

// Sentinel errors for the most common HTTP failure cases.
// Use them directly or as targets for errors.Is through a Cause chain.
var (
	ErrBadRequest           = &HTTPError{Status: http.StatusBadRequest, Code: "bad_request", Message: "bad request"}
	ErrUnauthorized         = &HTTPError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "unauthorized"}
	ErrForbidden            = &HTTPError{Status: http.StatusForbidden, Code: "forbidden", Message: "forbidden"}
	ErrNotFound             = &HTTPError{Status: http.StatusNotFound, Code: "not_found", Message: "not found"}
	ErrConflict             = &HTTPError{Status: http.StatusConflict, Code: "conflict", Message: "conflict"}
	ErrUnsupportedMediaType = &HTTPError{Status: http.StatusUnsupportedMediaType, Code: "unsupported_media_type", Message: "unsupported media type"}
	ErrUnprocessableEntity  = &HTTPError{Status: http.StatusUnprocessableEntity, Code: "unprocessable_entity", Message: "unprocessable entity"}
	ErrTooManyRequests      = &HTTPError{Status: http.StatusTooManyRequests, Code: "too_many_requests", Message: "too many requests"}
	ErrInternal             = &HTTPError{Status: http.StatusInternalServerError, Code: "internal_error", Message: "an internal error occurred"}
)

// New returns an HTTPError with the given HTTP status, machine-readable code, and message.
func New(status int, code, message string) *HTTPError {
	return &HTTPError{Status: status, Code: code, Message: message}
}

// BadRequest returns a 400 HTTPError with code "bad_request".
func BadRequest(message string) *HTTPError {
	return New(http.StatusBadRequest, "bad_request", message)
}

// Unauthorized returns a 401 HTTPError with code "unauthorized".
func Unauthorized(message string) *HTTPError {
	return New(http.StatusUnauthorized, "unauthorized", message)
}

// Forbidden returns a 403 HTTPError with code "forbidden".
func Forbidden(message string) *HTTPError {
	return New(http.StatusForbidden, "forbidden", message)
}

// NotFound returns a 404 HTTPError with code "not_found".
func NotFound(message string) *HTTPError {
	return New(http.StatusNotFound, "not_found", message)
}

// Conflict returns a 409 HTTPError with code "conflict".
func Conflict(message string) *HTTPError {
	return New(http.StatusConflict, "conflict", message)
}

// UnsupportedMediaType returns a 415 HTTPError with code "unsupported_media_type".
func UnsupportedMediaType(message string) *HTTPError {
	return New(http.StatusUnsupportedMediaType, "unsupported_media_type", message)
}

// UnprocessableEntity returns a 422 HTTPError with code "unprocessable_entity" and optional field Details.
func UnprocessableEntity(message string, details ...Detail) *HTTPError {
	e := New(http.StatusUnprocessableEntity, "unprocessable_entity", message)
	if len(details) > 0 {
		e.Details = details
	}
	return e
}

// TooManyRequests returns a 429 HTTPError with code "too_many_requests".
func TooManyRequests(message string) *HTTPError {
	return New(http.StatusTooManyRequests, "too_many_requests", message)
}

// Internal returns a 500 HTTPError with code "internal_error" and a wrapped cause.
// The cause is logged by Handle but is never serialized to the client (ADR-008).
func Internal(message string, cause error) *HTTPError {
	return &HTTPError{
		Status:  http.StatusInternalServerError,
		Code:    "internal_error",
		Message: message,
		Cause:   cause,
	}
}

// RequestEntityTooLarge returns a 413 HTTPError with code "request_too_large".
func RequestEntityTooLarge(message string) *HTTPError {
	return New(http.StatusRequestEntityTooLarge, "request_too_large", message)
}

// NotAcceptable returns a 406 HTTPError with code "not_acceptable".
func NotAcceptable(message string) *HTTPError {
	return New(http.StatusNotAcceptable, "not_acceptable", message)
}

// requestIDKey is the unexported context key for the per-request identifier.
type requestIDKey struct{}

// WithRequestID returns a new context carrying id as the per-request identifier.
// The request-ID middleware uses this to attach a unique ID to every request (ADR-005).
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID stored by WithRequestID, or an empty
// string if none is present in ctx.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}
