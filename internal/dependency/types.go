package dependency

import "time"

// IndexVersion is bumped when the on-disk index schema changes.
const IndexVersion = 1

// Kind classifies dependency graph nodes.
type Kind string

const (
	KindInternalGo    Kind = "internal_go"
	KindExternalGo    Kind = "external_go"
	KindStdlib        Kind = "stdlib"
	KindInternalJS    Kind = "internal_js"
	KindExternalNPM   Kind = "external_npm"
	KindWorkspaceNPM  Kind = "workspace_npm"
	KindBridge        Kind = "bridge" // Phase 2
)

// EdgeKind classifies dependency graph edges.
type EdgeKind string

const (
	EdgeSourceImport    EdgeKind = "source_import"
	EdgeManifestRequire EdgeKind = "manifest_require"
	EdgeWorkspaceRef    EdgeKind = "workspace_ref"
	EdgeDynamicImport   EdgeKind = "dynamic_import"
	EdgeBridge          EdgeKind = "bridge" // Phase 2
)

// BuildMethod records how the index was produced.
type BuildMethod string

const (
	BuildGoList         BuildMethod = "go_list"
	BuildParserFallback BuildMethod = "parser_fallback"
	BuildMerged         BuildMethod = "merged"
)

// ManifestRef points to the manifest file that declared a dependency.
type ManifestRef struct {
	Path    string `json:"path,omitempty"`
	Section string `json:"section,omitempty"` // module, require, dependencies, ...
}

// NodeMeta holds optional metadata for a node.
type NodeMeta struct {
	Version     string   `json:"version,omitempty"`
	Warnings    []string `json:"warnings,omitempty"`
	BuildMethod string   `json:"buildMethod,omitempty"`
	BridgeMethod string  `json:"bridgeMethod,omitempty"` // Phase 2
}

// Node is one vertex in the dependency graph.
type Node struct {
	ID        NodeID        `json:"id"`
	Kind      Kind          `json:"kind"`
	Name      string        `json:"name"`
	Dir       string        `json:"dir,omitempty"`
	Manifest  ManifestRef   `json:"manifest,omitempty"`
	Files     []string      `json:"files,omitempty"`
	Meta      NodeMeta      `json:"meta,omitempty"`
}

// Edge is one directed dependency between nodes.
type Edge struct {
	From  NodeID   `json:"from"`
	To    NodeID   `json:"to"`
	Kind  EdgeKind `json:"kind"`
	Files []string `json:"files,omitempty"`
}

// EdgeSnapshot is the persisted edge record in index.json.
type EdgeSnapshot struct {
	From  NodeID   `json:"from"`
	To    NodeID   `json:"to"`
	Kind  EdgeKind `json:"kind"`
	Files []string `json:"files,omitempty"`
}

// ParseError records a non-fatal parse failure during indexing.
type ParseError struct {
	File    string `json:"file"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
}

// ImpactLayers holds precomputed reverse-impact layers for a node.
type ImpactLayers struct {
	Direct      []NodeID   `json:"direct,omitempty"`
	Transitive  [][]NodeID `json:"transitive,omitempty"` // index 0 = distance 2, etc.
	External    []NodeID   `json:"external,omitempty"`
}

// ImpactEntry is one node in a formatted impact result.
type ImpactEntry struct {
	ID       NodeID `json:"id"`
	Name     string `json:"name"`
	Kind     Kind   `json:"kind"`
	Distance int    `json:"distance,omitempty"`
	EdgeVia  NodeID `json:"edgeVia,omitempty"`
}

// ImpactResult is the query-facing impact view for a source node.
type ImpactResult struct {
	Source     NodeID         `json:"source"`
	Layers     ImpactLayersView `json:"layers"`
	CrossRealm []ImpactEntry  `json:"crossRealm"`
	Hint       string         `json:"hint,omitempty"`
}

// ImpactLayersView is the formatted impact layers returned by queries.
type ImpactLayersView struct {
	Direct     []ImpactEntry `json:"direct,omitempty"`
	Transitive []ImpactEntry `json:"transitive,omitempty"`
	External   []ImpactEntry `json:"external,omitempty"`
}

// Cycle describes a source-import cycle within one language realm.
type Cycle struct {
	Ring     []NodeID `json:"ring"`
	Lang     string   `json:"lang"`               // go | js
	Severity string   `json:"severity"`           // error | warning
	Hub      NodeID   `json:"hub,omitempty"`      // barrel aggregate node, when detected
	Hint     string   `json:"hint,omitempty"`     // human-readable note for warnings
}

// VersionConflict records a manifest-level version mismatch.
type VersionConflict struct {
	Module   string   `json:"module"`
	Versions []string `json:"versions"`
	Paths    []string `json:"paths,omitempty"`
	Message  string   `json:"message,omitempty"`
}

// Stats summarizes index size and build health.
type Stats struct {
	NodeCount        int           `json:"nodeCount"`
	EdgeCount        int           `json:"edgeCount"`
	GoPackages       int           `json:"goPackages,omitempty"`
	JSPackages       int           `json:"jsPackages,omitempty"`
	ParseErrorCount  int           `json:"parseErrorCount"`
	ParseErrorSample []ParseError  `json:"parseErrorSample,omitempty"`
	BuildMethod      BuildMethod   `json:"buildMethod,omitempty"`
	BuildDurationMs  int64         `json:"buildDurationMs,omitempty"`
	BuiltAt          time.Time     `json:"builtAt,omitempty"`
	Stale            bool          `json:"stale,omitempty"`
	OrphanCount      int           `json:"orphanCount,omitempty"`
}

// Meta records index freshness metadata stored in meta.json.
type Meta struct {
	GeneratedAt  time.Time `json:"generatedAt"`
	GitHead      string    `json:"gitHead,omitempty"`
	Fingerprint  string    `json:"fingerprint"`
	IndexVersion int       `json:"indexVersion"`
}
