package callgraph

import (
	"fmt"
	"strings"

	"arcdesk/internal/realmid"
)

// NodeID uniquely identifies a call graph node.
type NodeID string

const (
	realmUI     = "ui"
	realmHook   = "hook"
	realmFn     = "fn"
	realmBridge = "bridge"
	realmGoBind = "gobind"
	realmGo     = "go"
	realmEmit   = "emit"
	realmListen = "listen"
)

var callgraphRealms = map[string]struct{}{
	realmUI: {}, realmHook: {}, realmFn: {}, realmBridge: {},
	realmGoBind: {}, realmGo: {}, realmEmit: {}, realmListen: {},
}

// ParseNodeID validates and parses a NodeID string.
func ParseNodeID(s string) (NodeID, error) {
	r, err := realmid.Parse(s)
	if err != nil {
		return "", err
	}
	if _, ok := callgraphRealms[r.Realm]; !ok {
		return "", fmt.Errorf("invalid node id %q: unknown realm %q", s, r.Realm)
	}
	return NodeID(r.String()), nil
}

func (n NodeID) String() string { return string(n) }

func (n NodeID) Realm() string {
	r, err := realmid.Parse(string(n))
	if err != nil {
		return ""
	}
	return r.Realm
}

func (n NodeID) Path() string {
	r, err := realmid.Parse(string(n))
	if err != nil {
		return string(n)
	}
	return r.Path
}

func (n NodeID) Symbol() string {
	r, err := realmid.Parse(string(n))
	if err != nil {
		return ""
	}
	return r.Symbol
}

// NewUIID returns a UI component/handler node id.
func NewUIID(file, symbol string) NodeID {
	return NodeID(realmid.New(realmUI, normalizeSlash(file), symbol))
}

// NewUIIDAtLine returns a UI node with call-site line.
func NewUIIDAtLine(file string, line int, symbol string) NodeID {
	return NodeID(realmid.NewWithLine(realmUI, normalizeSlash(file), line, symbol))
}

// NewHookID returns a hook node id.
func NewHookID(file, symbol string) NodeID {
	return NodeID(realmid.New(realmHook, normalizeSlash(file), symbol))
}

// NewFnID returns a TS function node id.
func NewFnID(file, symbol string) NodeID {
	return NodeID(realmid.New(realmFn, normalizeSlash(file), symbol))
}

// NewBridgeCallID returns a bridge invocation site node id.
func NewBridgeCallID(file string, line int, method string) NodeID {
	return NodeID(realmid.NewWithLine(realmBridge, normalizeSlash(file), line, "app."+strings.TrimSpace(method)))
}

// NewGoBindID returns a Go bind method node id.
func NewGoBindID(file, receiverMethod string) NodeID {
	return NodeID(realmid.New(realmGoBind, normalizeSlash(file), receiverMethod))
}

// NewGoInternalID returns a Go internal symbol node id.
func NewGoInternalID(file, symbol string) NodeID {
	return NodeID(realmid.New(realmGo, normalizeSlash(file), symbol))
}

// NewEventEmitID returns an event emit site node id.
func NewEventEmitID(file string, line int, channel string) NodeID {
	return NodeID(realmid.NewWithLine(realmEmit, normalizeSlash(file), line, channel))
}

// NewEventListenID returns an event listen site node id.
func NewEventListenID(file string, line int, channel string) NodeID {
	return NodeID(realmid.NewWithLine(realmListen, normalizeSlash(file), line, channel))
}

// NewAnonymousHandlerID returns an anonymous UI handler node id.
func NewAnonymousHandlerID(file string, line int) NodeID {
	return NodeID(realmid.NewWithLine(realmUI, normalizeSlash(file), line, "anonymous:"+itoa(line)))
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func normalizeSlash(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "/")
	return strings.ReplaceAll(p, "\\", "/")
}
