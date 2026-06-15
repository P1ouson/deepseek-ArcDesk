package harness

// Layer identifies one of the seven Harness layers.
type Layer string

const (
	LayerControlPlane Layer = "control_plane"
	LayerMemory       Layer = "memory"
	LayerContext      Layer = "context"
	LayerSubAgent     Layer = "sub_agent"
	LayerRAG          Layer = "rag"
	LayerSkills       Layer = "skills"
	LayerMCP          Layer = "mcp"
)

// AllLayers lists the canonical layer order for status output.
var AllLayers = []Layer{
	LayerControlPlane,
	LayerMemory,
	LayerContext,
	LayerSubAgent,
	LayerRAG,
	LayerSkills,
	LayerMCP,
}

// RAGCapabilities reports which retrieval indexes are wired this session.
type RAGCapabilities struct {
	GitRag      bool `json:"git_rag"`
	ArchRag     bool `json:"arch_rag"`
	UIRag       bool `json:"ui_rag"`
	RepoMap     bool `json:"repo_map"`
	Dependency  bool `json:"dependency"`
	Callgraph   bool `json:"callgraph"`
	Runtime     bool `json:"runtime"`
	FailureMem  bool `json:"failure_memory"`
	Codegraph   bool `json:"codegraph"`
}

// LayerFlags records which layers are active for the current session.
type LayerFlags struct {
	ControlPlane bool            `json:"control_plane"`
	Memory       bool            `json:"memory"`
	Context      bool            `json:"context"`
	SubAgent     bool            `json:"sub_agent"`
	RAG          RAGCapabilities `json:"rag"`
	Skills       int             `json:"skills"` // count of indexed skills
	MCP          int             `json:"mcp"`    // connected server count
}

// LayerEntry is one row in harness status output.
type LayerEntry struct {
	Layer       Layer  `json:"layer"`
	Active      bool   `json:"active"`
	Description string `json:"description"`
	Entry       string `json:"entry,omitempty"`
}

// DescribeLayers turns flags into human-readable layer rows for tools/UI.
func DescribeLayers(f LayerFlags) []LayerEntry {
	ragActive := f.RAG.GitRag || f.RAG.ArchRag || f.RAG.UIRag || f.RAG.RepoMap ||
		f.RAG.Dependency || f.RAG.Callgraph || f.RAG.Runtime || f.RAG.FailureMem || f.RAG.Codegraph
	out := []LayerEntry{
		{
			Layer:       LayerControlPlane,
			Active:      f.ControlPlane,
			Description: "PLAN-EXECUTE-VERIFY FSM, gates, transcript boundaries",
			Entry:       "internal/harness (Plane, FSM)",
		},
		{
			Layer:       LayerMemory,
			Active:      f.Memory,
			Description: "Working / Episodic / Semantic / Procedural + Consolidate",
			Entry:       "internal/harness (FourLayer, Consolidate)",
		},
		{
			Layer:       LayerContext,
			Active:      f.Context,
			Description: "Cache-stable prefix + turn-tail injection + compaction",
			Entry:       "internal/harness/context + agent.compact + ctxcompress",
		},
		{
			Layer:       LayerSubAgent,
			Active:      f.SubAgent,
			Description: "Isolated task/skill loops with tool whitelist",
			Entry:       "agent.task, skill subagent runner",
		},
		{
			Layer:       LayerRAG,
			Active:      ragActive,
			Description: "Structured retrieval tools (no embedding index by design)",
			Entry:       "gitrag, archrag, uirag, dependency, callgraph, failuremem, …",
		},
		{
			Layer:       LayerSkills,
			Active:      f.Skills > 0,
			Description: "Deterministic playbooks; bodies load on demand",
			Entry:       "internal/skill, run_skill, /<name>",
		},
		{
			Layer:       LayerMCP,
			Active:      f.MCP > 0,
			Description: "External MCP tool servers",
			Entry:       "internal/plugin, mcp__<server>__<tool>",
		},
	}
	return out
}
