// Purpose: Structured response writer with status and byte tracking, plus content-negotiated
// helpers for writing JSON and plain-text responses (ADR-006, ADR-011, ADR-012).

package router

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	httperr "github.com/example/lightweight-http/pkg/errors"
)

// ResponseWriter wraps an http.ResponseWriter to record the HTTP status code and the
// total bytes written for observability (Phase 7 metrics). It guarantees that the
// underlying WriteHeader is called exactly once; duplicate calls are logged and ignored.
type ResponseWriter struct {
	http.ResponseWriter
	status       int
	bytesWritten int64
	wroteHeader  bool
	logger       *slog.Logger
}

// WrapResponseWriter wraps w in a ResponseWriter. If logger is nil, slog.Default() is used.
func WrapResponseWriter(w http.ResponseWriter, logger *slog.Logger) *ResponseWriter {
	if logger == nil {
		logger = slog.Default()
	}
	return &ResponseWriter{ResponseWriter: w, logger: logger}
}

// Status returns the HTTP status code written to this response.
// Returns 200 if WriteHeader has never been explicitly called.
func (rw *ResponseWriter) Status() int {
	if !rw.wroteHeader {
		return http.StatusOK
	}
	return rw.status
}

// BytesWritten returns the total number of response body bytes written.
func (rw *ResponseWriter) BytesWritten() int64 {
	return rw.bytesWritten
}

// WriteHeader records the status code and forwards it to the underlying writer.
// Subsequent calls after the first are logged and ignored (status is written exactly once).
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		rw.logger.Warn("ResponseWriter.WriteHeader called after header already written",
			"attempted_status", code,
			"original_status", rw.status,
		)
		return
	}
	rw.status = code
	rw.wroteHeader = true
	rw.ResponseWriter.WriteHeader(code)
}

// Write forwards the body bytes to the underlying writer and accumulates the byte count.
// The first Write implicitly commits the header with status 200 per net/http semantics.
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.status = http.StatusOK
		rw.wroteHeader = true
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.bytesWritten += int64(n)
	return n, err
}

// WriteJSON writes v as a JSON-encoded body with the given status code.
//
// It performs Accept-header negotiation: if the client explicitly accepts application/json
// or has no preference (*/* or absent), JSON is sent. If the client cannot accept
// application/json, a 406 HTTPError is returned and nothing is written to w.
//
// Sets Content-Type: application/json on success.
func WriteJSON(w http.ResponseWriter, r *http.Request, status int, v any) error {
	ct, err := negotiate(r)
	if err != nil {
		return err
	}
	if ct != "application/json" {
		return httperr.NotAcceptable("client does not accept application/json")
	}
	b, merr := json.Marshal(v)
	if merr != nil {
		return fmt.Errorf("WriteJSON: marshal: %w", merr)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, werr := w.Write(b)
	return werr
}

// WriteText writes body as a plain-text response with the given status code.
//
// It performs Accept-header negotiation: if the client explicitly accepts text/plain
// or has no preference (*/* or absent), text is sent. If the client cannot accept
// text/plain, a 406 HTTPError is returned and nothing is written to w.
//
// Sets Content-Type: text/plain; charset=utf-8 on success.
func WriteText(w http.ResponseWriter, r *http.Request, status int, body string) error {
	ct, err := negotiate(r)
	if err != nil {
		return err
	}
	if ct != "text/plain" {
		return httperr.NotAcceptable("client does not accept text/plain")
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, werr := w.Write([]byte(body))
	return werr
}

// Respond negotiates the response content type from the Accept header and writes v.
//
// Selection rules (applied in order of media-type entries in Accept):
//   - absent or */* → application/json (default)
//   - application/json or application/* → JSON
//   - text/plain or text/* → plain text
//   - anything else → 406 HTTPError (nothing written to w)
//
// When writing as text/plain: if v is a string it is written verbatim; if it implements
// fmt.Stringer its String() result is used; otherwise v is JSON-marshalled into text.
func Respond(w http.ResponseWriter, r *http.Request, status int, v any) error {
	ct, err := negotiate(r)
	if err != nil {
		return err
	}
	switch ct {
	case "text/plain":
		var text string
		switch s := v.(type) {
		case string:
			text = s
		case fmt.Stringer:
			text = s.String()
		default:
			b, merr := json.Marshal(v)
			if merr != nil {
				return fmt.Errorf("Respond: marshal for text/plain: %w", merr)
			}
			text = string(b)
		}
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(status)
		_, werr := w.Write([]byte(text))
		return werr
	default: // application/json
		b, merr := json.Marshal(v)
		if merr != nil {
			return fmt.Errorf("Respond: marshal: %w", merr)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, werr := w.Write(b)
		return werr
	}
}

// NoContent writes a 204 No Content response with no body and no Content-Type header.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// negotiate parses the Accept header and returns the best supported content type.
// Returns ("application/json", nil) by default when Accept is absent or */*.
// Returns ("", *httperr.HTTPError{406}) when no supported type is acceptable.
func negotiate(r *http.Request) (string, error) {
	accept := r.Header.Get("Accept")
	if accept == "" {
		return "application/json", nil
	}

	for _, part := range strings.Split(accept, ",") {
		// Strip quality factor (e.g. ";q=0.9") and surrounding whitespace.
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch mt {
		case "*/*", "application/*":
			return "application/json", nil
		case "application/json":
			return "application/json", nil
		case "text/plain", "text/*":
			return "text/plain", nil
		}
	}

	return "", httperr.NotAcceptable(
		"none of the accepted media types are supported; supported: application/json, text/plain",
	)
}
