package callgraph

import (
	"context"
	"time"
)

// IndexVersion is bumped when index.json schema changes.
const IndexVersion = 2

// NodeKind classifies call graph vertices.
type NodeKind string

const (
	KindUIComponent NodeKind = "ui_component"
	KindUIHandler   NodeKind = "ui_handler"
	KindHook        NodeKind = "hook"
	KindTSFunction  NodeKind = "ts_function"
	KindBridgeCall  NodeKind = "bridge_call"
	KindGoBind      NodeKind = "go_bind"
	KindGoInternal  NodeKind = "go_internal"
	KindEventEmit   NodeKind = "event_emit"
	KindEventListen NodeKind = "event_listen"
)

// EdgeKind classifies call graph edges.
type EdgeKind string

const (
	EdgeCalls        EdgeKind = "calls"
	EdgeBridgeInvoke EdgeKind = "bridge_invoke"
	EdgeGoCalls       EdgeKind = "go_calls"
	EdgeHookUsedBy    EdgeKind = "hook_used_by"
	EdgeEmits         EdgeKind = "emits"
	EdgeListens       EdgeKind = "listens"
	EdgeEventDelivers EdgeKind = "event_delivers"
)

// Node is one vertex in the Wails call graph.
type Node struct {
	ID   NodeID   `json:"id"`
	Kind NodeKind `json:"kind"`
	Name string   `json:"name"`
	File string   `json:"file,omitempty"`
	Line int      `json:"line,omitempty"`
}

// Edge is one directed relationship.
type Edge struct {
	From NodeID   `json:"from"`
	To   NodeID   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// EdgeSnapshot is the persisted edge record (no files[]).
type EdgeSnapshot struct {
	From NodeID   `json:"from"`
	To   NodeID   `json:"to"`
	Kind EdgeKind `json:"kind"`
}

// NodeSnapshot is a query-facing node view.
type NodeSnapshot struct {
	ID   NodeID   `json:"id"`
	Kind NodeKind `json:"kind"`
	Name string   `json:"name"`
	File string   `json:"file,omitempty"`
	Line int      `json:"line,omitempty"`
}

// PathSegment is one hop in a CallPath.
type PathSegment struct {
	Node       NodeSnapshot `json:"node"`
	Edge       EdgeKind     `json:"edge,omitempty"`
	RealmCross bool         `json:"realmCross,omitempty"`
}

// CallPath is a traced cross-realm call chain.
type CallPath struct {
	ID        string        `json:"id"`
	Direction string        `json:"direction"` // forward | backward
	Segments  []PathSegment `json:"segments"`
	Truncated     bool          `json:"truncated,omitempty"`
	Hint          string        `json:"hint,omitempty"`
	EventChannel  string        `json:"eventChannel,omitempty"`
	PathKind      string        `json:"pathKind,omitempty"` // rpc | event
}

// ParseWarning records a non-fatal analysis issue.
type ParseWarning struct {
	File    string `json:"file,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

// Stats summarizes index health.
type Stats struct {
	NodeCount       int            `json:"nodeCount"`
	EdgeCount       int            `json:"edgeCount"`
	BridgeCallCount int            `json:"bridgeCallCount,omitempty"`
	GoBindCount       int            `json:"goBindCount,omitempty"`
	EventEmitCount    int            `json:"eventEmitCount,omitempty"`
	EventListenCount  int            `json:"eventListenCount,omitempty"`
	EventDeliverCount int            `json:"eventDeliverCount,omitempty"`
	WarningCount    int            `json:"warningCount,omitempty"`
	ParseErrorCount int            `json:"parseErrorCount,omitempty"`
	Warnings        []ParseWarning `json:"warnings,omitempty"`
	BuildDurationMs int64          `json:"buildDurationMs,omitempty"`
	BuiltAt         time.Time      `json:"builtAt,omitempty"`
	Stale           bool           `json:"stale,omitempty"`
}

// Meta records freshness metadata in meta.json.
type Meta struct {
	GeneratedAt  time.Time `json:"generatedAt"`
	GitHead      string    `json:"gitHead,omitempty"`
	Fingerprint  string    `json:"fingerprint"`
	IndexVersion int       `json:"indexVersion"`
}

// TraceOptions controls path tracing.
type TraceOptions struct {
	MaxDepth            int
	MaxPaths            int
	IncludeGoInternal   bool
	IncludeEvents       bool
	StopAtGoBindForward bool
	SymbolQuery         SymbolQuery
	SymbolContext       context.Context
}

// DefaultTraceOptions returns Phase 1 defaults.
func DefaultTraceOptions() TraceOptions {
	return TraceOptions{
		MaxDepth:            10,
		MaxPaths:            3,
		IncludeGoInternal:   false,
		IncludeEvents:       true,
		StopAtGoBindForward: true,
	}
}
