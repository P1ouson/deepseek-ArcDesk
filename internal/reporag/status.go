package reporag

import (
	"context"
	"fmt"

	"arcdesk/internal/codegraph"
	"arcdesk/internal/repomap"
)

// Status returns readiness of dependency, symbol graph, and navigation layers.
func (h *Host) Status(ctx context.Context) StatusReport {
	if h == nil {
		return StatusReport{}
	}
	var layers []LayerStatus

	depReady, depDetail, depStale := false, "disabled", false
	if h.Dep != nil {
		if st, err := h.Dep.Status(); err == nil {
			depReady = st.NodeCount > 0
			depDetail = fmt.Sprintf("%d modules, %d edges", st.NodeCount, st.EdgeCount)
			depStale = st.Stale
		} else if err := h.Dep.EnsureReady(ctx); err == nil {
			depReady = true
			depDetail = "index ready"
		} else {
			depDetail = err.Error()
		}
	}
	layers = append(layers, LayerStatus{
		Name: "dependency", Ready: depReady, Detail: depDetail, Stale: depStale, Enabled: h.Dep != nil,
	})

	cgReady, cgDetail, cgStale := false, "disabled", false
	if h.Callgraph != nil {
		if st, err := h.Callgraph.Status(); err == nil {
			cgReady = st.NodeCount > 0
			cgDetail = fmt.Sprintf("%d nodes, %d bridge calls", st.NodeCount, st.BridgeCallCount)
			cgStale = st.Stale
		}
	}
	layers = append(layers, LayerStatus{
		Name: "callgraph", Ready: cgReady, Detail: cgDetail, Stale: cgStale, Enabled: h.Callgraph != nil,
	})

	symReady, symDetail := false, "disabled"
	if h.CodegraphEnabled {
		if codegraph.Initialized(h.Root) {
			symReady = true
			symDetail = ".codegraph/ initialized"
		} else {
			symDetail = "run codegraph init or enable auto_install"
		}
		if h.Reg != nil {
			if _, ok := h.Reg.Get("mcp__codegraph__codegraph_search"); ok {
				symReady = true
				symDetail = "codegraph MCP connected"
			}
		}
	}
	layers = append(layers, LayerStatus{
		Name: "symbol_graph", Ready: symReady, Detail: symDetail, Enabled: h.CodegraphEnabled,
	})

	navReady, navDetail := false, "grep/glob always available"
	if h.LSP != nil {
		navReady = true
		navDetail = "LSP lazy servers + grep/glob fallback"
	}
	layers = append(layers, LayerStatus{
		Name: "code_navigation", Ready: navReady, Detail: navDetail, Enabled: h.LSP != nil,
	})

	mapReady := repomap.LoadBlock(h.Root) != ""
	mapDetail := "missing — warming on boot"
	if mapReady {
		mapDetail = "repo-map.md in system prefix"
	}
	layers = append(layers, LayerStatus{
		Name: "repomap", Ready: mapReady, Detail: mapDetail, Enabled: true,
	})

	return StatusReport{Layers: layers}
}
