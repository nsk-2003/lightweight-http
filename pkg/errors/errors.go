// Purpose: Centralized HTTP error type, JSON wire format, sentinel errors,
// optional stack capture (debug mode only), and the central error handler
// that every layer routes through (ADR-006, ADR-008).
package errors

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
)

// ─── Core error type ──────────────────────────────────────────────────────────

// HTTPError is the framework's canonical error type. It carries an HTTP status
// code, a stable machine-readable code, and a client-safe public message. An
// optional wrapped cause is accessible via errors.Is/As but is never included
// in client responses (ADR-008).
type HTTPError struct {
	// Code is the HTTP status code (e.g. 404).
	Code int
	// ErrCode is the stable machine-readable identifier (e.g. "not_found").
	// If empty, the handler derives one from Code.
	ErrCode string
	// Message is the client-safe human-readable description.
	Message string
	cause   error
	details []Detail
}

// Detail is a single structured field-level item in an error's details list.
// It is included in the JSON envelope only when the error was constructed with
// WithDetails.
type Detail struct {
	Field  string `json:"field,omitempty"`
	Reason string `json:"reason"`
}

// Error implements the error interface. The cause is included for server-side
// logging but is never sent to clients.
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
// ErrCode defaults to "" and is derived from Code when the envelope is written.
func New(code int, message string) *HTTPError {
	return &HTTPError{Code: code, Message: message}
}

// NewCoded creates an HTTPError with an explicit machine-readable errCode.
func NewCoded(status int, errCode string, message string) *HTTPError {
	return &HTTPError{Code: status, ErrCode: errCode, Message: message}
}

// Wrap creates an HTTPError that wraps cause. The cause is available for
// server-side inspection via errors.Is/As but is never sent to clients.
func Wrap(code int, message string, cause error) *HTTPError {
	return &HTTPError{Code: code, Message: message, cause: cause}
}

// WithDetails returns a shallow copy of e with the provided details attached.
// The original is not modified, so sentinels remain safe to reuse.
func (e *HTTPError) WithDetails(details ...Detail) *HTTPError {
	cp := *e
	cp.details = make([]Detail, len(details))
	copy(cp.details, details)
	return &cp
}

// CodeOf returns the HTTP status code for err. Returns http.StatusInternalServerError
// for any error that is not (or does not wrap) an *HTTPError, and 0 for nil.
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
// returns the generic "Internal Server Error" text. Returns "" for nil.
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

// ─── Sentinel errors ──────────────────────────────────────────────────────────

// Pre-built sentinel errors for the most common HTTP failure cases.
// Use errors.Is to test whether a wrapped error is one of these sentinels.
var (
	ErrBadRequest       = NewCoded(http.StatusBadRequest, "bad_request", "Bad Request")
	ErrUnauthorized     = NewCoded(http.StatusUnauthorized, "unauthorized", "Unauthorized")
	ErrForbidden        = NewCoded(http.StatusForbidden, "forbidden", "Forbidden")
	ErrNotFound         = NewCoded(http.StatusNotFound, "not_found", "Not Found")
	ErrConflict         = NewCoded(http.StatusConflict, "conflict", "Conflict")
	ErrUnsupportedMedia = NewCoded(http.StatusUnsupportedMediaType, "unsupported_media_type", "Unsupported Media Type")
	ErrUnprocessable    = NewCoded(http.StatusUnprocessableEntity, "unprocessable_entity", "Unprocessable Entity")
	ErrTooManyRequests  = NewCoded(http.StatusTooManyRequests, "too_many_requests", "Too Many Requests")
	ErrInternal         = NewCoded(http.StatusInternalServerError, "internal_error", "Internal Server Error")
)

// ─── Request ID context helpers ───────────────────────────────────────────────

// requestIDKey is the unexported context key for request IDs (ADR-005).
type requestIDKey struct{}

// WithRequestID returns a new context carrying id as the request identifier.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, id)
}

// RequestIDFromContext returns the request ID stored in ctx, or "" if none.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey{}).(string)
	return id
}

// ─── JSON envelope types ──────────────────────────────────────────────────────

// errorEnvelope is the top-level JSON wrapper: {"error": {...}}.
// No other top-level keys are ever written (ADR-006).
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// errorBody is the inner object of the error envelope.
type errorBody struct {
	Code      string   `json:"code"`
	Message   string   `json:"message"`
	Status    int      `json:"status"`
	RequestID string   `json:"request_id,omitempty"`
	Details   []Detail `json:"details,omitempty"`
}

// ─── Handler ──────────────────────────────────────────────────────────────────

// Handler serializes any error to the standard JSON envelope and writes it to
// the response. It is the single point of error-to-wire translation (ADR-006).
type Handler struct {
	log   *slog.Logger
	debug bool
}

// NewHandler creates a Handler. When debug is true, wrapped causes and runtime
// stack traces are written to the log; they are never sent to the client body
// (ADR-008). debug is set at construction time, never read from a request.
func NewHandler(log *slog.Logger, debug bool) *Handler {
	return &Handler{log: log, debug: debug}
}

// ServeError writes the JSON error envelope for err to w. The request ID is
// extracted from r's context when present. In production mode (debug=false),
// the cause and stack frames are never included in the client body.
func (h *Handler) ServeError(w http.ResponseWriter, r *http.Request, err error) {
	var he *HTTPError
	if !stderrors.As(err, &he) {
		// Unknown error — log it and respond with a generic 500.
		h.log.ErrorContext(r.Context(), "unhandled error",
			slog.Any("error", err),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		)
		he = &HTTPError{
			Code:    http.StatusInternalServerError,
			ErrCode: "internal_error",
			Message: http.StatusText(http.StatusInternalServerError),
		}
	} else if he.cause != nil {
		// HTTPError with a cause — log cause server-side; keep it off the wire.
		attrs := []any{
			slog.Any("cause", he.cause),
			slog.Int("status", he.Code),
			slog.String("method", r.Method),
			slog.String("path", r.URL.Path),
		}
		if h.debug {
			attrs = append(attrs, slog.String("stack", string(debug.Stack())))
		}
		h.log.ErrorContext(r.Context(), "error with cause", attrs...)
	}

	errCode := he.ErrCode
	if errCode == "" {
		errCode = deriveErrCode(he.Code)
	}

	body := errorBody{
		Code:      errCode,
		Message:   he.Message,
		Status:    he.Code,
		RequestID: RequestIDFromContext(r.Context()),
		Details:   he.details, // nil → omitempty removes it from JSON
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(he.Code)
	_ = json.NewEncoder(w).Encode(errorEnvelope{Error: body})
}

// WriteError writes the standard JSON error envelope using slog.Default() and
// production mode (no debug). It is a convenience wrapper for callers that do
// not need a pre-configured Handler (e.g. the router's built-in 404/405).
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	NewHandler(slog.Default(), false).ServeError(w, r, err)
}

// ─── helpers ──────────────────────────────────────────────────────────────────

// deriveErrCode returns a stable machine-readable code for a status when the
// HTTPError was not constructed with an explicit ErrCode.
func deriveErrCode(status int) string {
	switch status {
	case http.StatusBadRequest:
		return "bad_request"
	case http.StatusUnauthorized:
		return "unauthorized"
	case http.StatusForbidden:
		return "forbidden"
	case http.StatusNotFound:
		return "not_found"
	case http.StatusMethodNotAllowed:
		return "method_not_allowed"
	case http.StatusConflict:
		return "conflict"
	case http.StatusUnsupportedMediaType:
		return "unsupported_media_type"
	case http.StatusUnprocessableEntity:
		return "unprocessable_entity"
	case http.StatusTooManyRequests:
		return "too_many_requests"
	default:
		return "internal_error"
	}
}
