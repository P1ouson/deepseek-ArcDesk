package workspacerefresh

import (
	"strings"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/codegraph"
	"arcdesk/internal/config"
	"arcdesk/internal/dependency"
	"arcdesk/internal/repomap"
	"arcdesk/internal/reporeuse"
)

// Action describes what refresh will do for one layer.
type Action string

const (
	ActionSkip     Action = "skip"
	ActionMetaBump Action = "meta_bump"
	ActionRefresh  Action = "refresh"
	ActionDisabled Action = "disabled"
)

// LayerPlan is a dry-run decision for one index layer.
type LayerPlan struct {
	Name   string `json:"name"`
	Action Action `json:"action"`
	Reason string `json:"reason,omitempty"`
	Detail string `json:"detail,omitempty"`
}

// Plan summarizes refresh decisions without mutating indexes.
type Plan struct {
	GitHead     string      `json:"gitHead,omitempty"`
	Changed     []string    `json:"changedFiles,omitempty"`
	Layers      []LayerPlan `json:"layers"`
	SkipCount   int         `json:"skipCount"`
	RefreshCount int        `json:"refreshCount"`
}

// PlanInput holds workspace indexes for planning.
type PlanInput struct {
	Root             string
	Cfg              *config.Config
	Dep              *dependency.Index
	Cg               *callgraph.Index
	CodegraphEnabled bool
}

// BuildPlan returns dry-run refresh actions for repomap, dependency, callgraph, codegraph.
func BuildPlan(in PlanInput) Plan {
	root := strings.TrimSpace(in.Root)
	head, _ := repomap.WorkspaceRevision(root)
	plan := Plan{GitHead: head}

	var oldDepHead, oldCgHead string
	var depFP, cgFP string
	if in.Dep != nil {
		if meta := in.Dep.MetaSnapshot(); meta != nil {
			oldDepHead = meta.GitHead
			depFP = meta.Fingerprint
		}
	}
	if in.Cg != nil {
		if meta := in.Cg.MetaSnapshot(); meta != nil {
			oldCgHead = meta.GitHead
			cgFP = meta.Fingerprint
		}
	}

	changed, _ := reporeuse.ChangedFilesBetween(root, oldDepHead, head)
	plan.Changed = changed

	plan.Layers = nil
	plan.Layers = append(plan.Layers, planRepomap(in, head))
	plan.Layers = append(plan.Layers, planDependency(in, head, oldDepHead, depFP, changed))
	plan.Layers = append(plan.Layers, planCallgraph(in, head, oldCgHead, cgFP, changed))
	plan.Layers = append(plan.Layers, planCodegraph(in))

	for _, l := range plan.Layers {
		switch l.Action {
		case ActionSkip, ActionDisabled, ActionMetaBump:
			plan.SkipCount++
		case ActionRefresh:
			plan.RefreshCount++
		}
	}
	return plan
}

func planRepomap(in PlanInput, head string) LayerPlan {
	if in.Cfg == nil || !in.Cfg.Reporag.ShouldEnable() {
		return LayerPlan{Name: "repomap", Action: ActionDisabled, Reason: "reporag disabled"}
	}
	stale, err := repomap.IsStale(in.Root)
	if err != nil {
		return LayerPlan{Name: "repomap", Action: ActionRefresh, Reason: "error", Detail: err.Error()}
	}
	if !stale {
		return LayerPlan{Name: "repomap", Action: ActionSkip, Reason: "fresh", Detail: "head=" + shortHead(head)}
	}
	return LayerPlan{Name: "repomap", Action: ActionRefresh, Reason: "stale"}
}

func planDependency(in PlanInput, head, oldHead, fp string, changed []string) LayerPlan {
	if in.Cfg == nil || !in.Cfg.Dependency.ShouldIndex(dependency.Discoverable(in.Root)) {
		return LayerPlan{Name: "dependency", Action: ActionDisabled, Reason: "dependency indexing off"}
	}
	if in.Dep == nil {
		return LayerPlan{Name: "dependency", Action: ActionRefresh, Reason: "not loaded"}
	}
	st, err := in.Dep.Status()
	if err != nil {
		return LayerPlan{Name: "dependency", Action: ActionRefresh, Reason: "not ready"}
	}
	if !st.Stale {
		return LayerPlan{Name: "dependency", Action: ActionSkip, Reason: "fresh"}
	}
	newFP := dependency.ComputeFingerprint(in.Root)
	if reporeuse.HeadChangedFingerprintStable(oldHead, head, fp, newFP) && !reporeuse.PathsAffectDependency(changed) {
		return LayerPlan{
			Name: "dependency", Action: ActionMetaBump,
			Reason: "head_only_non_module",
			Detail: shortHead(oldHead) + "→" + shortHead(head),
		}
	}
	return LayerPlan{Name: "dependency", Action: ActionRefresh, Reason: "stale"}
}

func planCallgraph(in PlanInput, head, oldHead, fp string, changed []string) LayerPlan {
	if in.Cfg == nil || !in.Cfg.Callgraph.ShouldIndex(callgraph.Discoverable(in.Root)) {
		return LayerPlan{Name: "callgraph", Action: ActionDisabled, Reason: "callgraph indexing off"}
	}
	if in.Cg == nil {
		return LayerPlan{Name: "callgraph", Action: ActionRefresh, Reason: "not loaded"}
	}
	st, err := in.Cg.Status()
	if err != nil {
		return LayerPlan{Name: "callgraph", Action: ActionRefresh, Reason: "not ready"}
	}
	if !st.Stale {
		return LayerPlan{Name: "callgraph", Action: ActionSkip, Reason: "fresh"}
	}
	newFP := callgraph.ComputeFingerprint(in.Root)
	if reporeuse.HeadChangedFingerprintStable(oldHead, head, fp, newFP) && !reporeuse.PathsAffectCallgraph(changed) {
		return LayerPlan{
			Name: "callgraph", Action: ActionMetaBump,
			Reason: "head_only_non_bridge",
			Detail: shortHead(oldHead) + "→" + shortHead(head),
		}
	}
	return LayerPlan{Name: "callgraph", Action: ActionRefresh, Reason: "stale"}
}

func planCodegraph(in PlanInput) LayerPlan {
	if in.Cfg == nil || !in.Cfg.Codegraph.Enabled {
		return LayerPlan{Name: "codegraph", Action: ActionDisabled, Reason: "codegraph disabled"}
	}
	if codegraph.Initialized(in.Root) {
		return LayerPlan{Name: "codegraph", Action: ActionSkip, Reason: "initialized", Detail: ".codegraph/"}
	}
	return LayerPlan{Name: "codegraph", Action: ActionRefresh, Reason: "not_initialized"}
}

func shortHead(h string) string {
	h = strings.TrimSpace(h)
	if len(h) <= 7 {
		return h
	}
	return h[:7]
}
