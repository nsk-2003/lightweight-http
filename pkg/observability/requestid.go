// Purpose: RequestID middleware that adopts or generates a per-request ID, validates it, and echoes it on the response.

package observability

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"

	httperr "github.com/example/lightweight-http/pkg/errors"
	"github.com/example/lightweight-http/pkg/middleware"
)

const (
	// requestIDHeader is the canonical HTTP header name for the per-request identifier.
	requestIDHeader = "X-Request-ID"

	// maxRequestIDLen is the maximum accepted length for an incoming request ID.
	// Values longer than this are replaced with a generated ID.
	maxRequestIDLen = 128
)

// isValidRequestID reports whether s is a well-formed request ID.
// Valid IDs are non-empty, at most maxRequestIDLen characters, and contain only
// alphanumeric characters, hyphens, or underscores.
func isValidRequestID(s string) bool {
	if s == "" || len(s) > maxRequestIDLen {
		return false
	}
	for _, c := range s {
		ok := (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_'
		if !ok {
			return false
		}
	}
	return true
}

// generateRequestID produces a cryptographically random 32-character hex string.
// In the extremely unlikely event that crypto/rand fails, a fixed fallback is returned
// so that the request can still proceed — the failure is not propagated to the caller.
func generateRequestID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "00000000000000000000000000000000"
	}
	return hex.EncodeToString(b)
}

// RequestID returns a Middleware that assigns a unique ID to every request.
//
// If the incoming request carries an X-Request-ID header with a valid value (non-empty,
// at most 128 characters, only alphanumeric/hyphen/underscore characters), that value is
// adopted. Otherwise, a cryptographically random 32-character hex ID is generated.
//
// The resulting ID is:
//   - stored in the request context via pkg/errors.WithRequestID (ADR-005);
//   - echoed on the response as X-Request-ID before the handler runs.
func RequestID() middleware.Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := r.Header.Get(requestIDHeader)
			if !isValidRequestID(id) {
				id = generateRequestID()
			}
			w.Header().Set(requestIDHeader, id)
			r = r.WithContext(httperr.WithRequestID(r.Context(), id))
			next.ServeHTTP(w, r)
		})
	}
}
