package callgraph

import (
	"fmt"
	"time"
)

// BuildOptions configures a call graph build.
type BuildOptions struct {
	Root    string
	Catalog ModuleCatalog
}

// BuildGraph constructs a Wails call graph for the workspace.
// Sub-step failures degrade to warnings; only missing root returns an error.
func BuildGraph(opts BuildOptions) (*CallGraph, *Meta, error) {
	root := opts.Root
	if root == "" {
		return nil, nil, fmt.Errorf("root required")
	}
	start := time.Now()
	g := NewGraph(root)

	catalog := opts.Catalog
	if catalog == nil {
		catalog = NewFallbackCatalog(root)
	}
	_ = catalog

	binds, internals, emits, goWarn, _ := ScanGoBinds(root)

	cat := BuildMethodCatalog(root, binds)
	tsSyms, tsCalls, tsListens, tsWarn, _ := ScanTSFiles(root, cat.Methods)

	for _, b := range binds {
		g.AddNode(&Node{
			ID:   b.ID,
			Kind: KindGoBind,
			Name: "App." + b.Method,
			File: b.File,
			Line: b.Line,
		})
	}
	for _, in := range internals {
		g.AddNode(&Node{
			ID:   in.ID,
			Kind: KindGoInternal,
			Name: in.Name,
			File: in.File,
			Line: in.Line,
		})
		g.AddEdge(in.FromBind, in.ID, EdgeGoCalls)
	}
	for _, s := range tsSyms {
		g.AddNode(&Node{
			ID:   s.ID,
			Kind: s.Kind,
			Name: s.Name,
			File: s.File,
			Line: s.Line,
		})
	}
	for _, c := range tsCalls {
		kind := c.Kind
		if kind == "" {
			kind = EdgeCalls
		}
		g.AddEdge(c.From, c.To, kind)
		toNode, okTo := g.Node(c.To)
		fromNode, okFrom := g.Node(c.From)
		if okTo && okFrom && toNode.Kind == KindHook && fromNode.Kind == KindUIComponent {
			g.AddEdge(c.To, c.From, EdgeHookUsedBy)
		}
	}

	bridgeWarn := LinkBridgeEdges(g, binds)

	emitWarn := AttachEventEmits(g, emits)
	for _, l := range tsListens {
		g.AddNode(&Node{
			ID:   l.ID,
			Kind: KindEventListen,
			Name: l.Channel,
			File: l.File,
			Line: l.Line,
		})
	}
	LinkListenEdges(g, tsListens)
	deliverCount := LinkEventDelivers(g)

	g.Warnings = append(g.Warnings, goWarn...)
	g.Warnings = append(g.Warnings, tsWarn...)
	g.Warnings = append(g.Warnings, cat.Warnings...)
	g.Warnings = append(g.Warnings, bridgeWarn...)
	g.Warnings = append(g.Warnings, emitWarn...)
	g.Warnings = append(g.Warnings, finalizeWarnings(g, binds, cat.Methods, cat.DTS)...)

	g.RebuildIndexes()
	g.BuiltAt = time.Now().UTC()
	g.Stats = computeStats(g, time.Since(start), deliverCount)
	meta := NewMeta(root)
	return g, meta, nil
}

func computeStats(g *CallGraph, elapsed time.Duration, eventDeliverCount int) Stats {
	stats := Stats{
		NodeCount:         len(g.Nodes),
		EdgeCount:         g.EdgeCount(),
		BuildDurationMs:   elapsed.Milliseconds(),
		BuiltAt:           g.BuiltAt,
		Warnings:          g.Warnings,
		WarningCount:      len(g.Warnings),
		ParseErrorCount:   countParseErrors(g.Warnings),
		EventDeliverCount: eventDeliverCount,
	}
	for _, n := range g.Nodes {
		if n == nil {
			continue
		}
		switch n.Kind {
		case KindBridgeCall:
			stats.BridgeCallCount++
		case KindGoBind:
			stats.GoBindCount++
		case KindEventEmit:
			stats.EventEmitCount++
		case KindEventListen:
			stats.EventListenCount++
		}
	}
	return stats
}
