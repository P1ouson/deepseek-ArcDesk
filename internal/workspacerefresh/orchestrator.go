package workspacerefresh

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/codegraph"
	"arcdesk/internal/config"
	"arcdesk/internal/dependency"
	"arcdesk/internal/repomap"
)

// Host orchestrates repo index refresh for one workspace.
type Host struct {
	Root string
	Cfg  *config.Config
	Dep  *dependency.Index
	Cg   *callgraph.Index

	mu         sync.Mutex
	lastReport Report
}

// Report captures the latest orchestrated refresh run.
type Report struct {
	Plan      Plan      `json:"plan"`
	AfterPlan Plan      `json:"afterPlan,omitempty"`
	RanAt     time.Time `json:"ranAt,omitempty"`
	Errors    []string  `json:"errors,omitempty"`
}

// LastReport returns the most recent refresh report.
func (h *Host) LastReport() Report {
	if h == nil {
		return Report{}
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.lastReport
}

// PlanInput builds dry-run refresh decisions.
func (h *Host) PlanInput() PlanInput {
	if h == nil {
		return PlanInput{}
	}
	codegraphOn := h.Cfg != nil && h.Cfg.Codegraph.Enabled
	return PlanInput{
		Root:             h.Root,
		Cfg:              h.Cfg,
		Dep:              h.Dep,
		Cg:               h.Cg,
		CodegraphEnabled: codegraphOn,
	}
}

// Refresh runs repomap → dependency → callgraph refresh in order.
func (h *Host) Refresh(ctx context.Context) Report {
	if h == nil {
		return Report{}
	}
	in := h.PlanInput()
	before := BuildPlan(in)
	var errs []string

	if h.Cfg != nil && h.Cfg.Reporag.ShouldEnable() {
		if err := repomap.EnsureReady(h.Root); err != nil {
			errs = append(errs, "repomap ensure: "+err.Error())
		}
		if err := repomap.RefreshIfStale(h.Root); err != nil {
			errs = append(errs, "repomap refresh: "+err.Error())
		}
	}

	if h.Dep != nil {
		if err := h.Dep.RefreshIfStale(ctx); err != nil {
			errs = append(errs, "dependency refresh: "+err.Error())
		}
	}

	if h.Cg != nil {
		if err := h.Cg.RefreshIfStale(ctx); err != nil {
			errs = append(errs, "callgraph refresh: "+err.Error())
		}
	}

	if h.Cfg != nil && h.Cfg.Codegraph.Enabled {
		if bin, ok := codegraph.Resolve(h.Cfg.Codegraph.Path); ok {
			if err := codegraph.EnsureInit(ctx, bin, h.Root); err != nil {
				slog.Debug("workspacerefresh: codegraph init", "err", err)
			}
		}
	}

	after := BuildPlan(in)
	rep := Report{
		Plan:      before,
		AfterPlan: after,
		RanAt:     time.Now().UTC(),
		Errors:    errs,
	}
	h.mu.Lock()
	h.lastReport = rep
	h.mu.Unlock()
	return rep
}

// RefreshBackground runs Refresh without blocking the caller.
func (h *Host) RefreshBackground(ctx context.Context) {
	if h == nil {
		return
	}
	go func() {
		_ = h.Refresh(context.WithoutCancel(ctx))
	}()
}

// NewHost builds a refresh host from workspace kit fields.
func NewHost(root string, cfg *config.Config, dep *dependency.Index, cg *callgraph.Index) *Host {
	return &Host{Root: root, Cfg: cfg, Dep: dep, Cg: cg}
}
