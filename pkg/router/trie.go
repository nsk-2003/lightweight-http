// Purpose: Trie-based segment matcher for HTTP path routing. Each node holds
// one path segment's worth of dispatch state. Static children are looked up in
// O(1) via a map; at most one wildcard child exists per node, giving O(segments)
// matching with no backtracking (ADR-004).
package router

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// trieNode represents one path segment in the routing trie.
type trieNode struct {
	staticKids map[string]*trieNode        // exact-segment children
	paramKid   *trieNode                   // single wildcard child, if any
	paramName  string                      // name of wildcard (without ':')
	handlers   map[string]http.HandlerFunc // HTTP method → handler
	pattern    string                      // full route pattern, e.g. "/users/:id"
}

func newNode() *trieNode {
	return &trieNode{
		staticKids: make(map[string]*trieNode),
		handlers:   make(map[string]http.HandlerFunc),
	}
}

// add registers handler h for method at the given pre-split segments path.
// It reports an error on duplicate parameter names within the pattern or a
// duplicate (method, pattern) pair.
func (n *trieNode) add(method string, segments []string, h http.HandlerFunc) error {
	return n.addAt(method, segments, 0, nil, h)
}

func (n *trieNode) addAt(method string, segs []string, idx int, seen map[string]struct{}, h http.HandlerFunc) error {
	if idx == len(segs) {
		pat := "/"
		if len(segs) > 0 {
			pat = "/" + strings.Join(segs, "/")
		}
		if _, dup := n.handlers[method]; dup {
			return fmt.Errorf("router: %s %s already registered", method, pat)
		}
		if n.pattern == "" {
			n.pattern = pat
		}
		n.handlers[method] = h
		return nil
	}

	seg := segs[idx]

	if strings.HasPrefix(seg, ":") {
		// Wildcard segment.
		name := seg[1:]
		if name == "" {
			return fmt.Errorf("router: empty parameter name in segment %q", seg)
		}
		if _, dup := seen[name]; dup {
			return fmt.Errorf("router: duplicate parameter name %q in pattern", name)
		}
		if n.paramKid == nil {
			n.paramKid = newNode()
			n.paramKid.paramName = name
		} else if n.paramKid.paramName != name {
			return fmt.Errorf("router: conflicting parameter names at same position: %q vs %q",
				n.paramKid.paramName, name)
		}
		newSeen := cloneSet(seen)
		newSeen[name] = struct{}{}
		return n.paramKid.addAt(method, segs, idx+1, newSeen, h)
	}

	// Static segment.
	child, ok := n.staticKids[seg]
	if !ok {
		child = newNode()
		n.staticKids[seg] = child
	}
	return child.addAt(method, segs, idx+1, seen, h)
}

// matchResult carries a successful trie match.
type matchResult struct {
	node    *trieNode
	params  map[string]string
	pattern string // matched route pattern, e.g. "/users/:id"
}

// match walks the trie for the given request segments.
// Static children take priority over the wildcard child (ADR-004, static wins).
// There is no backtracking: once a static child is chosen at a level we do not
// retry the wildcard even if the static path leads to a dead end deeper down.
// Returns nil on no match.
func (n *trieNode) match(segs []string, idx int, params map[string]string) *matchResult {
	if idx == len(segs) {
		if len(n.handlers) == 0 {
			return nil
		}
		return &matchResult{node: n, params: params, pattern: n.pattern}
	}

	raw := segs[idx]
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		decoded = raw
	}

	// Static wins. No backtracking into paramKid if static fails deeper.
	if child, ok := n.staticKids[decoded]; ok {
		return child.match(segs, idx+1, params)
	}

	// Fall through to wildcard only when no static child matched this segment.
	if n.paramKid != nil {
		p := cloneStringMap(params)
		p[n.paramKid.paramName] = decoded
		return n.paramKid.match(segs, idx+1, p)
	}

	return nil
}

// allowedMethods returns the HTTP methods registered at the node reached by
// following segs, using the same routing logic as match. Returns nil when the
// path is not in the trie at all.
func (n *trieNode) allowedMethods(segs []string, idx int) []string {
	if idx == len(segs) {
		if len(n.handlers) == 0 {
			return nil
		}
		methods := make([]string, 0, len(n.handlers))
		for m := range n.handlers {
			methods = append(methods, m)
		}
		return methods
	}

	raw := segs[idx]
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		decoded = raw
	}

	if child, ok := n.staticKids[decoded]; ok {
		return child.allowedMethods(segs, idx+1)
	}
	if n.paramKid != nil {
		return n.paramKid.allowedMethods(segs, idx+1)
	}
	return nil
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func cloneSet(m map[string]struct{}) map[string]struct{} {
	out := make(map[string]struct{}, len(m)+1)
	for k := range m {
		out[k] = struct{}{}
	}
	return out
}

func cloneStringMap(m map[string]string) map[string]string {
	out := make(map[string]string, len(m)+1)
	for k, v := range m {
		out[k] = v
	}
	return out
}
