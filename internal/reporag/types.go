package reporag

import (
	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/lsp"
	"arcdesk/internal/tool"
)

// Host wires repo intelligence layers for status, symbol search, and navigation.
type Host struct {
	Root             string
	Dep              *dependency.Index
	Callgraph        *callgraph.Index
	LSP              *lsp.Manager
	Reg              *tool.Registry
	CodegraphEnabled bool
}

// LayerStatus summarizes one repo intelligence tier.
type LayerStatus struct {
	Name    string `json:"name"`
	Ready   bool   `json:"ready"`
	Detail  string `json:"detail,omitempty"`
	Stale   bool   `json:"stale,omitempty"`
	Enabled bool   `json:"enabled"`
}

// StatusReport is the unified repo RAG health snapshot.
type StatusReport struct {
	Layers []LayerStatus `json:"layers"`
}
