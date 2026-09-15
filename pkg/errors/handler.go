// Purpose: Central error handler that converts any error into the standard JSON envelope
// and writes it to the response; the single site that formats error responses (ADR-006).

package errors

import (
	"encoding/json"
	stderrors "errors"
	"log/slog"
	"net/http"
	runtimedebug "runtime/debug"
)

// envelope is the top-level JSON structure for every error response.
// No keys other than "error" are permitted at this level (ADR-006).
type envelope struct {
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

// Handle converts err into the standard error envelope and writes it to w.
//
// If err wraps an *HTTPError (found via errors.As), its Status, Code, Message, and Details
// are used. Any other error type becomes a 500 with a generic client message; the original
// error is logged so it can be correlated with a request ID.
//
// The Cause field is always logged and never written to the client body (ADR-008).
// When debugMode is true, a goroutine stack snapshot is also written to the log; the
// client-facing body is identical regardless of debugMode.
//
// If logger is nil, it is resolved to slog.Default() only when a log entry is needed.
func Handle(w http.ResponseWriter, r *http.Request, err error, logger *slog.Logger, debugMode bool) {
	if err == nil {
		return
	}

	var he *HTTPError
	if !stderrors.As(err, &he) {
		he = Internal("an internal error occurred", err)
	}

	reqID := RequestIDFromContext(r.Context())

	if he.Status >= 500 || debugMode {
		if logger == nil {
			logger = slog.Default()
		}
		attrs := []any{
			"status", he.Status,
			"code", he.Code,
			"method", r.Method,
			"path", r.URL.Path,
		}
		if reqID != "" {
			attrs = append(attrs, "request_id", reqID)
		}
		if he.Cause != nil {
			attrs = append(attrs, "cause", he.Cause.Error())
		}
		if debugMode {
			attrs = append(attrs, "stack", string(runtimedebug.Stack()))
		}
		logger.Error("request error", attrs...)
	}

	body := errorBody{
		Code:      he.Code,
		Message:   he.Message,
		Status:    he.Status,
		RequestID: reqID,
		Details:   he.Details,
	}
	env := envelope{Error: body}
	b, _ := json.Marshal(env)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(he.Status)
	_, _ = w.Write(b)
}
