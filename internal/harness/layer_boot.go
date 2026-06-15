package harness

import (
	"arcdesk/internal/config"
	"arcdesk/internal/dependency"
	"arcdesk/internal/callgraph"
	"arcdesk/internal/gitrag"
	"arcdesk/internal/memory"
	"arcdesk/internal/uirag"
)

// BuildLayerFlags records which Harness layers are wired for a booted session.
func BuildLayerFlags(cfg *config.Config, root string, skillCount, mcpServerCount int, mem *memory.Set) LayerFlags {
	if cfg == nil {
		cfg = config.Default()
	}
	return LayerFlags{
		ControlPlane: true,
		Memory:       mem != nil,
		Context:      true,
		SubAgent:     true,
		Skills:       skillCount,
		MCP:          mcpServerCount,
		RAG: RAGCapabilities{
			GitRag:     cfg.GitRag.ShouldEnable(gitrag.Discoverable(root)),
			ArchRag:    cfg.ArchRag.ShouldEnable(),
			UIRag:      cfg.UIRag.ShouldEnable(uirag.Discoverable(root)),
			RepoMap:    cfg.Reporag.ShouldEnable(),
			Dependency: cfg.Dependency.ShouldIndex(dependency.Discoverable(root)),
			Callgraph:  cfg.Callgraph.ShouldIndex(callgraph.Discoverable(root)),
			Runtime:    cfg.Runtime.ShouldEnable(),
			FailureMem: cfg.FailureMemory.ShouldEnable(),
			Codegraph:  cfg.Codegraph.Enabled,
		},
	}
}

// SemanticLoaded reports whether hierarchical memory docs were discovered.
func SemanticLoaded(mem *memory.Set) bool {
	return mem != nil && !mem.Empty()
}
