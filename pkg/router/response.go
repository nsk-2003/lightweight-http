// Purpose: Structured response writer — tracks status code and bytes written
// for Phase 7 metrics, enforces once-only header writes, and provides JSON,
// text, no-content, and content-negotiated response helpers (ADR-006, ADR-012).
package router

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	httperrors "github.com/example/lightweight-http/pkg/errors"
)

// ResponseWriter wraps http.ResponseWriter to record the HTTP status code and
// the number of bytes written. WriteHeader may only be called once; a second
// call logs a warning and is otherwise ignored rather than panicking.
type ResponseWriter struct {
	http.ResponseWriter
	status int
	n      int64
	wrote  bool
	log    *slog.Logger // nil → use slog.Default() for double-write warnings
}

// WrapResponseWriter wraps w in a ResponseWriter. Pass a non-nil log to direct
// double-write warnings to a specific logger; nil uses slog.Default().
func WrapResponseWriter(w http.ResponseWriter, log *slog.Logger) *ResponseWriter {
	return &ResponseWriter{ResponseWriter: w, log: log}
}

// Status returns the HTTP status code sent to the client. Returns 0 if neither
// WriteHeader nor Write has been called yet.
func (rw *ResponseWriter) Status() int { return rw.status }

// BytesWritten returns the total number of body bytes written to the underlying
// ResponseWriter.
func (rw *ResponseWriter) BytesWritten() int64 { return rw.n }

// WriteHeader records code and delegates to the underlying writer. A second
// call logs a warning at the Warn level and returns without altering the
// already-sent response status (safe to call after the header is written).
func (rw *ResponseWriter) WriteHeader(code int) {
	if rw.wrote {
		logger := rw.log
		if logger == nil {
			logger = slog.Default()
		}
		logger.Warn("WriteHeader called after header already sent",
			slog.Int("ignored_code", code),
			slog.Int("sent_code", rw.status),
		)
		return
	}
	rw.wrote = true
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Write implicitly commits a 200 status if WriteHeader has not yet been called,
// accumulates the byte count, and delegates to the underlying writer.
func (rw *ResponseWriter) Write(b []byte) (int, error) {
	if !rw.wrote {
		rw.wrote = true
		rw.status = http.StatusOK
	}
	n, err := rw.ResponseWriter.Write(b)
	rw.n += int64(n)
	return n, err
}

// ─── Response helpers ─────────────────────────────────────────────────────────

// JSON encodes v as JSON and writes it to w with the given HTTP status code.
// It sets Content-Type: application/json; charset=utf-8.
func JSON(w http.ResponseWriter, status int, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("router.JSON: marshal: %w", err)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, err = w.Write(b)
	return err
}

// Text writes body as plain text to w with the given HTTP status code.
// It sets Content-Type: text/plain; charset=utf-8.
func Text(w http.ResponseWriter, status int, body string) error {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, err := w.Write([]byte(body))
	return err
}

// NoContent writes a 204 No Content response with no body and no Content-Type.
func NoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Respond performs content negotiation from r's Accept header and writes v
// as JSON (default) or plain text. If Accept is absent or "*/*", JSON is used.
// Returns a 406 HTTPError without writing anything when no supported type is found.
func Respond(w http.ResponseWriter, r *http.Request, status int, v any) error {
	accept := r.Header.Get("Accept")
	if accept == "" || accept == "*/*" {
		return JSON(w, status, v)
	}

	for _, part := range strings.Split(accept, ",") {
		mt := strings.TrimSpace(strings.SplitN(part, ";", 2)[0])
		switch mt {
		case "application/json", "*/*":
			return JSON(w, status, v)
		case "text/plain":
			return Text(w, status, fmt.Sprintf("%v", v))
		}
	}
	return httperrors.New(http.StatusNotAcceptable,
		fmt.Sprintf("no supported media type in Accept: %s", accept))
}
