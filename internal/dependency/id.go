package dependency

import (
	"fmt"
	"strings"

	"arcdesk/internal/realmid"
)

// NodeID uniquely identifies a node in the dependency graph.
// Wire format: "<realm>:<path>" (e.g. "go:arcdesk/internal/agent").
type NodeID string

const (
	realmGo     = "go"
	realmGoMod  = "gomod"
	realmJS     = "js"
	realmNpm    = "npm"
	realmBridge = "bridge"
)

var dependencyRealms = map[string]struct{}{
	realmGo:     {},
	realmGoMod:  {},
	realmJS:     {},
	realmNpm:    {},
	realmBridge: {},
}

// ParseNodeID validates and returns a NodeID from its string form.
func ParseNodeID(s string) (NodeID, error) {
	r, err := realmid.Parse(s)
	if err != nil {
		return "", err
	}
	if _, ok := dependencyRealms[r.Realm]; !ok {
		return "", fmt.Errorf("invalid node id %q: unknown realm %q", s, r.Realm)
	}
	if r.Symbol != "" || r.Line != 0 {
		return "", fmt.Errorf("invalid node id %q: dependency ids must not include symbol or line", s)
	}
	return NodeID(r.String()), nil
}

// NewGoID returns an internal Go package node id.
func NewGoID(importPath string) NodeID {
	return NodeID(realmid.New(realmGo, strings.TrimSpace(importPath), ""))
}

// NewGoModID returns an external Go module node id.
func NewGoModID(modulePath string) NodeID {
	return NodeID(realmid.New(realmGoMod, strings.TrimSpace(modulePath), ""))
}

// NewStdlibID returns a standard library package node id (gomod:std:<pkg>).
func NewStdlibID(pkg string) NodeID {
	return NodeID(realmid.New(realmGoMod, "std:"+strings.TrimSpace(pkg), ""))
}

// NewJSID returns a frontend directory package node id (repo-relative, slash-separated).
func NewJSID(workspaceRelPath string) NodeID {
	return NodeID(realmid.New(realmJS, normalizeSlash(workspaceRelPath), ""))
}

// NewNpmID returns an npm package node id.
func NewNpmID(name string) NodeID {
	return NodeID(realmid.New(realmNpm, strings.TrimSpace(name), ""))
}

func (n NodeID) String() string { return string(n) }

// Realm returns the realm prefix (go, gomod, js, npm, bridge).
func (n NodeID) Realm() string {
	r, err := realmid.Parse(string(n))
	if err != nil {
		return ""
	}
	return r.Realm
}

// Path returns the path portion after the realm prefix.
func (n NodeID) Path() string {
	r, err := realmid.Parse(string(n))
	if err != nil {
		return string(n)
	}
	return r.Path
}

func normalizeSlash(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	return strings.ReplaceAll(p, "\\", "/")
}
