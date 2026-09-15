// Purpose: Defines the HTTPError type and constructor functions for framework-level errors
// that carry an HTTP status code, enabling centralized error rendering (ADR-006).

package errors

import "fmt"

// HTTPError is a framework-level error with an HTTP status code and a human-readable message.
// Handlers and parsing helpers return HTTPError values rather than writing error responses
// themselves; a centralized error handler (Phase 5) renders them uniformly.
type HTTPError struct {
	// Code is the HTTP status code to send to the client.
	Code int
	// Message is a short, safe-to-expose description of the error.
	Message string
}

// Error implements the error interface.
func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.Code, e.Message)
}

// StatusCode returns the HTTP status code associated with this error.
func (e *HTTPError) StatusCode() int {
	return e.Code
}

// New returns an HTTPError with the given status code and message.
func New(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// BadRequest returns an HTTPError with status 400 Bad Request.
func BadRequest(message string) *HTTPError {
	return New(400, message)
}

// RequestEntityTooLarge returns an HTTPError with status 413 Request Entity Too Large.
func RequestEntityTooLarge(message string) *HTTPError {
	return New(413, message)
}

// UnsupportedMediaType returns an HTTPError with status 415 Unsupported Media Type.
func UnsupportedMediaType(message string) *HTTPError {
	return New(415, message)
}

// NotAcceptable returns an HTTPError with status 406 Not Acceptable.
func NotAcceptable(message string) *HTTPError {
	return New(406, message)
}
