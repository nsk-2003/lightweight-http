// Purpose: statusRecorder wraps http.ResponseWriter to capture the response status code and byte count.

package observability

import "net/http"

// statusRecorder wraps an http.ResponseWriter and records the first status code written
// and the total number of bytes written.
type statusRecorder struct {
	http.ResponseWriter
	status      int
	bytes       int
	wroteHeader bool
}

// newStatusRecorder returns a statusRecorder that defaults to status 200 (matching net/http
// behavior when WriteHeader is never called explicitly).
func newStatusRecorder(w http.ResponseWriter) *statusRecorder {
	return &statusRecorder{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the first status code and delegates to the underlying writer.
// Subsequent calls are ignored, matching the behavior of net/http's response writer.
func (r *statusRecorder) WriteHeader(code int) {
	if r.wroteHeader {
		return
	}
	r.wroteHeader = true
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write delegates to the underlying writer and accumulates the byte count.
func (r *statusRecorder) Write(b []byte) (int, error) {
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

// Unwrap returns the underlying ResponseWriter, enabling http.ResponseController to function correctly.
func (r *statusRecorder) Unwrap() http.ResponseWriter {
	return r.ResponseWriter
}
