// Purpose: Typed request parsing from JSON bodies, URL-encoded form data, and query
// parameters, enforcing size limits and content-type checks (ADR-005, ADR-006).

package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	httperr "github.com/example/lightweight-http/pkg/errors"
)

// DefaultMaxBodyBytes is the default maximum size of a request body (1 MiB).
// Bodies exceeding this limit are rejected with a 413 status before the body is read
// into memory, preventing out-of-memory conditions.
const DefaultMaxBodyBytes int64 = 1 << 20

// ParseOptions configures the behavior of ParseJSON.
type ParseOptions struct {
	// MaxBodyBytes caps the body size. Zero uses DefaultMaxBodyBytes.
	MaxBodyBytes int64
	// DisallowUnknownFields rejects JSON with fields absent from the target type.
	// Defaults to true when omitted from a ParseJSON call (strict mode).
	DisallowUnknownFields bool
}

// noopResponseWriter is a minimal http.ResponseWriter passed to http.MaxBytesReader.
// MaxBytesReader stores the writer but only uses it for an internal type assertion to
// reach server-internal state; a concrete non-nil value prevents a nil-interface
// dereference if that assertion path is reached.
type noopResponseWriter struct{}

func (noopResponseWriter) Header() http.Header         { return http.Header{} }
func (noopResponseWriter) Write(b []byte) (int, error) { return len(b), nil }
func (noopResponseWriter) WriteHeader(int)             {}

// ParseJSON reads and decodes the request body as JSON into v.
//
// The default configuration is strict (DisallowUnknownFields=true) with a 1 MiB body
// limit. Pass a ParseOptions as the third argument to override either setting.
//
// The body is consumed by this call; do not call ParseJSON twice on the same request.
//
// Error statuses returned:
//   - 415 if Content-Type is not application/json (params like charset are ignored)
//   - 413 if the body exceeds MaxBodyBytes
//   - 400 for malformed JSON or an unknown field in strict mode
func ParseJSON(r *http.Request, v any, opts ...ParseOptions) error {
	ct := r.Header.Get("Content-Type")
	mediaType := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])
	if !strings.EqualFold(mediaType, "application/json") {
		return httperr.UnsupportedMediaType("Content-Type must be application/json")
	}

	opt := ParseOptions{MaxBodyBytes: DefaultMaxBodyBytes, DisallowUnknownFields: true}
	if len(opts) > 0 {
		o := opts[0]
		if o.MaxBodyBytes > 0 {
			opt.MaxBodyBytes = o.MaxBodyBytes
		}
		opt.DisallowUnknownFields = o.DisallowUnknownFields
	}

	limited := http.MaxBytesReader(noopResponseWriter{}, r.Body, opt.MaxBodyBytes)
	dec := json.NewDecoder(limited)
	if opt.DisallowUnknownFields {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(v); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return httperr.RequestEntityTooLarge("request body too large")
		}
		if strings.HasPrefix(err.Error(), "json: unknown field") {
			field := strings.TrimPrefix(err.Error(), `json: unknown field "`)
			field = strings.TrimSuffix(field, `"`)
			return httperr.BadRequest(fmt.Sprintf("unknown field: %s", field))
		}
		return httperr.BadRequest("malformed JSON request body")
	}
	return nil
}

// ParseForm parses the request body as URL-encoded or multipart form data,
// populating r.Form and r.PostForm for subsequent access via r.FormValue.
//
// Content-Type must be application/x-www-form-urlencoded or multipart/form-data.
// maxMemory controls the maximum bytes of multipart body held in memory; it defaults
// to 32 MiB when omitted.
//
// Returns a 415 error if the Content-Type does not match either form type.
func ParseForm(r *http.Request, maxMemory ...int64) error {
	ct := r.Header.Get("Content-Type")
	mediaType := strings.TrimSpace(strings.SplitN(ct, ";", 2)[0])

	switch {
	case strings.EqualFold(mediaType, "application/x-www-form-urlencoded"):
		if err := r.ParseForm(); err != nil {
			return httperr.BadRequest(fmt.Sprintf("failed to parse form body: %v", err))
		}
		return nil
	case strings.EqualFold(mediaType, "multipart/form-data"):
		mem := int64(32 << 20)
		if len(maxMemory) > 0 && maxMemory[0] > 0 {
			mem = maxMemory[0]
		}
		if err := r.ParseMultipartForm(mem); err != nil {
			return httperr.BadRequest(fmt.Sprintf("failed to parse multipart form: %v", err))
		}
		return nil
	default:
		return httperr.UnsupportedMediaType(
			"Content-Type must be application/x-www-form-urlencoded or multipart/form-data",
		)
	}
}

// RequireQuery returns the value of the named query parameter.
// A parameter that is present but has an empty value ("?key=") is returned as an empty
// string without error. Returns a 400 HTTPError naming the parameter when it is absent.
func RequireQuery(r *http.Request, name string) (string, error) {
	q := r.URL.Query()
	vals, ok := q[name]
	if !ok {
		return "", httperr.BadRequest(fmt.Sprintf("missing required query parameter: %s", name))
	}
	return vals[0], nil
}

// QueryInt returns the named query parameter as a signed integer.
// Returns a 400 HTTPError if the parameter is absent or its value cannot be parsed as int.
func QueryInt(r *http.Request, name string) (int, error) {
	s, err := RequireQuery(r, name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, httperr.BadRequest(fmt.Sprintf("query parameter %q must be an integer", name))
	}
	return n, nil
}
