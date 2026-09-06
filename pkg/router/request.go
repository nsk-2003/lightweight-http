// Purpose: Typed request parsing helpers — BindJSON, BindForm, FormParam,
// FormParamInt, QueryParam, QueryParamInt, and PathParam. Bodies are bounded
// by http.MaxBytesReader; Content-Type is verified before parsing begins.
package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	httperrors "github.com/example/lightweight-http/pkg/errors"
)

// DefaultBodyLimit is the default maximum request body size in bytes (1 MiB).
const DefaultBodyLimit int64 = 1 << 20

// BindOption configures body-parsing behavior for BindJSON and BindForm.
type BindOption func(*bindConfig)

type bindConfig struct {
	bodyLimit  int64
	strictJSON bool
}

func defaultBindConfig() *bindConfig {
	return &bindConfig{bodyLimit: DefaultBodyLimit, strictJSON: true}
}

// WithBodyLimit sets the maximum body size in bytes. Bodies exceeding the
// limit produce a 413 Request Entity Too Large error.
func WithBodyLimit(n int64) BindOption {
	return func(c *bindConfig) { c.bodyLimit = n }
}

// WithStrictJSON controls whether unknown JSON fields are rejected (default:
// true). Pass false to silently ignore unknown fields.
func WithStrictJSON(strict bool) BindOption {
	return func(c *bindConfig) { c.strictJSON = strict }
}

// noopRW is a minimal http.ResponseWriter used as the required first argument
// to http.MaxBytesReader when no real ResponseWriter is in scope. MaxBytesReader
// stores the value but does not write to it in Go 1.19+.
type noopRW struct{ h http.Header }

func (n *noopRW) Header() http.Header         { return n.h }
func (n *noopRW) Write(b []byte) (int, error) { return len(b), nil }
func (n *noopRW) WriteHeader(int)             {}

func newNoopRW() *noopRW { return &noopRW{h: make(http.Header)} }

// BindJSON decodes the JSON request body into dst. It requires the request's
// Content-Type to be application/json (or absent), enforces the body size limit
// (default 1 MiB), and in strict mode (default true) rejects unknown fields.
// The body is consumed; calling BindJSON twice on the same request will fail.
func BindJSON(r *http.Request, dst any, opts ...BindOption) error {
	cfg := defaultBindConfig()
	for _, o := range opts {
		o(cfg)
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" && !strings.HasPrefix(ct, "application/json") {
		return httperrors.New(http.StatusUnsupportedMediaType,
			fmt.Sprintf("content-type must be application/json, got %q", ct))
	}

	r.Body = http.MaxBytesReader(newNoopRW(), r.Body, cfg.bodyLimit)
	dec := json.NewDecoder(r.Body)
	if cfg.strictJSON {
		dec.DisallowUnknownFields()
	}

	if err := dec.Decode(dst); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return httperrors.New(http.StatusRequestEntityTooLarge, "request body too large")
		}
		return httperrors.Wrap(http.StatusBadRequest, sanitizeJSONError(err), err)
	}
	return nil
}

// sanitizeJSONError converts a json decode error into a safe client message
// that does not echo raw request-body content.
func sanitizeJSONError(err error) string {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return "malformed JSON"
	}
	msg := err.Error()
	// Unknown-field messages name the field, not the value — safe to forward.
	if strings.HasPrefix(msg, "json: unknown field") {
		return msg
	}
	// Type-mismatch messages describe the type mismatch, not the value.
	if strings.HasPrefix(msg, "json: cannot unmarshal") {
		return msg
	}
	return "malformed JSON"
}

// BindForm parses an application/x-www-form-urlencoded or multipart/form-data
// request body, enforcing the body size limit. After BindForm returns nil, use
// FormParam or FormParamInt to extract typed values from r.Form.
// The body is consumed; do not call BindForm twice on the same request.
func BindForm(r *http.Request, opts ...BindOption) error {
	cfg := defaultBindConfig()
	for _, o := range opts {
		o(cfg)
	}

	ct := r.Header.Get("Content-Type")
	if ct != "" &&
		!strings.HasPrefix(ct, "application/x-www-form-urlencoded") &&
		!strings.HasPrefix(ct, "multipart/form-data") {
		return httperrors.New(http.StatusUnsupportedMediaType,
			fmt.Sprintf("content-type must be application/x-www-form-urlencoded or multipart/form-data, got %q", ct))
	}

	r.Body = http.MaxBytesReader(newNoopRW(), r.Body, cfg.bodyLimit)
	if err := r.ParseForm(); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			return httperrors.New(http.StatusRequestEntityTooLarge, "request body too large")
		}
		return httperrors.Wrap(http.StatusBadRequest, "could not parse form body", err)
	}
	return nil
}

// FormParam returns the named form field value from r.Form. Call BindForm first
// to apply the body limit and populate r.Form. Returns a 400 error naming the
// parameter if it is absent.
func FormParam(r *http.Request, name string) (string, error) {
	if r.Form == nil {
		if err := r.ParseForm(); err != nil {
			return "", httperrors.Wrap(http.StatusBadRequest, "could not parse form", err)
		}
	}
	vals, ok := r.Form[name]
	if !ok || len(vals) == 0 {
		return "", httperrors.New(http.StatusBadRequest,
			fmt.Sprintf("missing form parameter %q", name))
	}
	return vals[0], nil
}

// FormParamInt returns the named form field as an integer. Returns a 400 error
// if the parameter is absent or cannot be parsed as an integer.
func FormParamInt(r *http.Request, name string) (int, error) {
	s, err := FormParam(r, name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, httperrors.New(http.StatusBadRequest,
			fmt.Sprintf("form parameter %q must be an integer, got %q", name, s))
	}
	return n, nil
}

// QueryParam returns the named query-string parameter value. Returns a 400
// error naming the parameter when the key is not present in the URL query.
func QueryParam(r *http.Request, name string) (string, error) {
	q := r.URL.Query()
	if !q.Has(name) {
		return "", httperrors.New(http.StatusBadRequest,
			fmt.Sprintf("missing query parameter %q", name))
	}
	return q.Get(name), nil
}

// QueryParamInt returns the named query-string parameter as an integer.
// Returns a 400 error if the parameter is absent or not parseable as int.
func QueryParamInt(r *http.Request, name string) (int, error) {
	s, err := QueryParam(r, name)
	if err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, httperrors.New(http.StatusBadRequest,
			fmt.Sprintf("query parameter %q must be an integer, got %q", name, s))
	}
	return n, nil
}

// PathParam returns the named path parameter from the request context. Returns
// a 400 error if the parameter is absent, which indicates a route/handler
// mismatch. The router always populates parameters for matched routes.
func PathParam(r *http.Request, name string) (string, error) {
	params := Params(r.Context())
	v, ok := params[name]
	if !ok {
		return "", httperrors.New(http.StatusBadRequest,
			fmt.Sprintf("path parameter %q not found", name))
	}
	return v, nil
}
