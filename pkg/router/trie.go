// Purpose: Implements the segment trie that provides O(segments) path matching without backtracking (ADR-004).

package router

import (
	"fmt"
	"net/http"
)

// trieNode is one node in the routing trie. Each node corresponds to one path-segment position.
type trieNode struct {
	// static maps a literal segment string to its child node.
	static map[string]*trieNode

	// param is the child reached when no static child matches and a :name wildcard was registered
	// at this position. At most one wildcard is allowed per level.
	param *trieNode

	// paramName is the name of the wildcard that leads to param (e.g. "id" for ":id").
	// Set when the first wildcard route at this level is registered; subsequent registrations
	// at the same level must use the same name.
	paramName string

	// handlers maps HTTP method strings (e.g. "GET") to the registered handler.
	handlers map[string]http.HandlerFunc

	// pattern is the full route pattern string stored at the terminal node.
	pattern string
}

func newTrieNode() *trieNode {
	return &trieNode{static: make(map[string]*trieNode)}
}

// register inserts a handler into the trie at the position described by segments.
// paramsSeen tracks parameter names already encountered in this single pattern to detect duplicates.
// method is the uppercase HTTP method string. pattern is the full route pattern for storage at
// the terminal node. Returns an error for duplicate routes or invalid patterns.
func (n *trieNode) register(method string, segments []string, paramsSeen map[string]bool, h http.HandlerFunc, pattern string) error {
	if len(segments) == 0 {
		if n.handlers == nil {
			n.handlers = make(map[string]http.HandlerFunc)
			n.pattern = pattern
		}
		if _, dup := n.handlers[method]; dup {
			return fmt.Errorf("router: duplicate route: %s (method+pattern already registered)", method)
		}
		n.handlers[method] = h
		return nil
	}

	seg := segments[0]
	rest := segments[1:]

	if len(seg) > 0 && seg[0] == ':' {
		name := seg[1:]
		if name == "" {
			return fmt.Errorf("router: empty parameter name in pattern segment %q", seg)
		}
		if paramsSeen[name] {
			return fmt.Errorf("router: duplicate path parameter %q in pattern", name)
		}
		paramsSeen[name] = true

		if n.param == nil {
			n.param = newTrieNode()
			n.paramName = name
		} else if n.paramName != name {
			return fmt.Errorf("router: conflicting path parameter at this segment: already registered as %q, cannot use %q", n.paramName, name)
		}
		return n.param.register(method, rest, paramsSeen, h, pattern)
	}

	child, ok := n.static[seg]
	if !ok {
		child = newTrieNode()
		n.static[seg] = child
	}
	return child.register(method, rest, paramsSeen, h, pattern)
}

// matchResult holds the outcome of a successful trie traversal.
type matchResult struct {
	node       *trieNode
	captured   []string // captured param values, in traversal order
	paramNames []string // param names corresponding to captured values, in traversal order
	pattern    string   // matched route pattern
}

// match traverses the trie for the given path segments and returns the matched node with
// any captured parameter values. Returns nil when no route matches.
//
// Static children are always preferred over the wildcard child (ADR-004). Once a static
// match is taken, the traversal commits to that subtree — there is no backtracking.
func (n *trieNode) match(segments []string, captured, paramNames []string) *matchResult {
	if len(segments) == 0 {
		if n.handlers != nil {
			return &matchResult{node: n, captured: captured, paramNames: paramNames, pattern: n.pattern}
		}
		return nil
	}

	seg := segments[0]
	rest := segments[1:]

	// Static match takes unconditional priority; no fallback to wildcard if this fails.
	if child, ok := n.static[seg]; ok {
		return child.match(rest, captured, paramNames)
	}

	// No static match: try the wildcard child.
	if n.param != nil {
		return n.param.match(rest,
			append(captured, seg),
			append(paramNames, n.paramName),
		)
	}

	return nil
}

// allowedMethods returns a slice of HTTP methods that have handlers at this node.
func (n *trieNode) allowedMethods() []string {
	methods := make([]string, 0, len(n.handlers))
	for m := range n.handlers {
		methods = append(methods, m)
	}
	return methods
}
