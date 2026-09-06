// Purpose: Framework HTTP error type — pairs an HTTP status code with a
// human-readable public message. Phase 5 adds structured JSON rendering;
// this file provides the type, constructors, and status/message helpers.
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
)

// HTTPError is the framework's canonical error type. It carries an HTTP status
// code and a public message safe to send to clients. An optional wrapped cause
// is accessible via errors.Is/As but is never included in client responses.
type HTTPError struct {
	Code    int
	Message string
	cause   error
}

// Error implements the error interface. It includes the cause when present to
// aid server-side logging, but the cause is never sent to clients.
func (e *HTTPError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap returns the wrapped cause so errors.Is and errors.As can traverse the
// chain. Returns nil when no cause was provided.
func (e *HTTPError) Unwrap() error { return e.cause }

// New creates an HTTPError with the given HTTP status code and public message.
func New(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// Wrap creates an HTTPError that wraps cause. The cause is available for
// server-side inspection via errors.Is/As but is never sent to clients.
func Wrap(code int, message string, cause error) *HTTPError {
	return &HTTPError{Code: code, Message: message, cause: cause}
}

// CodeOf returns the HTTP status code for err. Returns http.StatusInternalServerError
// (500) for any error that is not (or does not wrap) an *HTTPError, and 0 for nil.
func CodeOf(err error) int {
	if err == nil {
		return 0
	}
	var he *HTTPError
	if stderrors.As(err, &he) {
		return he.Code
	}
	return http.StatusInternalServerError
}

// MessageOf returns the public-safe message for err. For non-HTTPErrors it
// returns the generic "Internal Server Error" text to avoid leaking internals.
// Returns an empty string for nil.
func MessageOf(err error) string {
	if err == nil {
		return ""
	}
	var he *HTTPError
	if stderrors.As(err, &he) {
		return he.Message
	}
	return http.StatusText(http.StatusInternalServerError)
}
