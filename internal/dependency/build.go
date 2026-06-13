package dependency

import (
	"fmt"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// BuildOptions configures a full dependency graph build.
type BuildOptions struct {
	Root string
}

// BuildGraph indexes Go (and optionally JS) dependencies, analyzes the graph,
// and returns a ready-to-query Graph with freshness metadata.
func BuildGraph(opts BuildOptions) (*Graph, *Meta, error) {
	root := strings.TrimSpace(opts.Root)
	if root == "" {
		return nil, nil, fmt.Errorf("workspace root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, nil, err
	}

	start := time.Now()
	g := NewGraph(abs)
	var parseErrors []ParseError
	buildMethod := BuildMethod("")

	if goNodes, goEdges, goErrs, method, err := buildGoSection(abs); err != nil {
		slog.Debug("dependency: skip go section", "root", abs, "err", err)
	} else {
		mergeBuildResult(g, goNodes, goEdges, goErrs)
		parseErrors = append(parseErrors, goErrs...)
		if method != "" {
			buildMethod = method
		}
	}

	if nodeNodes, nodeEdges, nodeErrs, err := buildNodeSection(abs); err != nil {
		slog.Debug("dependency: skip node section", "root", abs, "err", err)
	} else if nodeNodes != nil || nodeEdges != nil || nodeErrs != nil {
		mergeBuildResult(g, nodeNodes, nodeEdges, nodeErrs)
		parseErrors = append(parseErrors, nodeErrs...)
		if buildMethod == "" {
			buildMethod = BuildMerged
		} else if buildMethod != BuildMerged {
			buildMethod = BuildMerged
		}
	}

	if len(g.Nodes) == 0 {
		g.ParseErrors = parseErrors
		g.BuildMethod = buildMethod
		g.BuiltAt = time.Now().UTC()
		g.Stats = computeStats(g, time.Since(start))
		meta := &Meta{
			GeneratedAt:  g.BuiltAt,
			GitHead:      gitHead(abs),
			Fingerprint:  ComputeFingerprint(abs),
			IndexVersion: IndexVersion,
		}
		return g, meta, nil
	}

	g.ParseErrors = parseErrors
	g.BuildMethod = buildMethod
	g.BuiltAt = time.Now().UTC()
	g.Cycles = computeCycles(g)
	g.Conflicts = detectVersionConflicts(abs, g)
	computeImpact(g)
	g.Orphans = findOrphans(g)
	g.Stats = computeStats(g, time.Since(start))

	meta := &Meta{
		GeneratedAt:  g.BuiltAt,
		GitHead:      gitHead(abs),
		Fingerprint:  ComputeFingerprint(abs),
		IndexVersion: IndexVersion,
	}
	return g, meta, nil
}

func buildGoSection(root string) ([]*Node, []Edge, []ParseError, BuildMethod, error) {
	b, err := NewGoBuilder(root)
	if err != nil {
		return nil, nil, nil, "", err
	}
	return b.Build()
}

func buildNodeSection(root string) ([]*Node, []Edge, []ParseError, error) {
	b, err := NewNodeBuilder(root)
	if err != nil {
		return nil, nil, nil, err
	}
	nodes, edges, errs, err := b.Build()
	if err != nil {
		return nil, nil, nil, err
	}
	if nodes == nil && edges == nil && len(errs) == 0 {
		return nil, nil, nil, nil
	}
	return nodes, edges, errs, nil
}

func mergeBuildResult(g *Graph, nodes []*Node, edges []Edge, parseErrors []ParseError) {
	if g == nil {
		return
	}
	for _, n := range nodes {
		if n == nil {
			continue
		}
		if existing, ok := g.Nodes[n.ID]; ok {
			mergeNode(existing, n)
		} else {
			g.AddNode(cloneNode(n))
		}
		for _, f := range n.Files {
			g.Files[normalizeSlash(f)] = n.ID
		}
	}
	for _, e := range edges {
		file := ""
		if len(e.Files) > 0 {
			file = e.Files[0]
		}
		g.AddEdge(e.From, e.To, e.Kind, file)
	}
	_ = parseErrors
}

func mergeNode(dst, src *Node) {
	if dst == nil || src == nil {
		return
	}
	if src.Dir != "" {
		dst.Dir = src.Dir
	}
	if src.Name != "" {
		dst.Name = src.Name
	}
	dst.Files = appendUniqueNodeIDStrings(dst.Files, src.Files)
	for _, w := range src.Meta.Warnings {
		dst.Meta.Warnings = appendUniqueString(dst.Meta.Warnings, w)
	}
	if src.Meta.Version != "" {
		dst.Meta.Version = src.Meta.Version
	}
	if src.Meta.BuildMethod != "" {
		dst.Meta.BuildMethod = src.Meta.BuildMethod
	}
}

func cloneNode(n *Node) *Node {
	cp := *n
	if len(n.Files) > 0 {
		cp.Files = append([]string(nil), n.Files...)
	}
	if len(n.Meta.Warnings) > 0 {
		cp.Meta.Warnings = append([]string(nil), n.Meta.Warnings...)
	}
	return &cp
}

func appendUniqueNodeIDStrings(dst, add []string) []string {
	for _, s := range add {
		s = normalizeSlash(s)
		found := false
		for _, d := range dst {
			if d == s {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, s)
		}
	}
	return dst
}

func findOrphans(g *Graph) []NodeID {
	if g == nil {
		return nil
	}
	var orphans []NodeID
	for id, n := range g.Nodes {
		if len(g.In[id]) > 0 || len(g.Out[id]) > 0 {
			continue
		}
		switch n.Kind {
		case KindExternalGo, KindExternalNPM, KindStdlib:
			continue
		}
		orphans = append(orphans, id)
	}
	sort.Slice(orphans, func(i, j int) bool { return orphans[i] < orphans[j] })
	return orphans
}

func computeStats(g *Graph, elapsed time.Duration) Stats {
	stats := Stats{
		NodeCount:       len(g.Nodes),
		EdgeCount:       len(g.edges),
		OrphanCount:     len(g.Orphans),
		ParseErrorCount: len(g.ParseErrors),
		BuildMethod:     g.BuildMethod,
		BuildDurationMs: elapsed.Milliseconds(),
		BuiltAt:         g.BuiltAt,
	}
	for _, n := range g.Nodes {
		switch n.Kind {
		case KindInternalGo, KindExternalGo, KindStdlib:
			stats.GoPackages++
		case KindInternalJS, KindExternalNPM, KindWorkspaceNPM:
			stats.JSPackages++
		}
	}
	stats.ParseErrorSample = sampleParseErrors(g.ParseErrors, 10)
	return stats
}

func sampleParseErrors(in []ParseError, max int) []ParseError {
	if len(in) == 0 {
		return nil
	}
	if len(in) <= max {
		return append([]ParseError(nil), in...)
	}
	return append([]ParseError(nil), in[:max]...)
}

func isInternalKind(k Kind) bool {
	switch k {
	case KindInternalGo, KindInternalJS:
		return true
	default:
		return false
	}
}

func isExternalKind(k Kind) bool {
	switch k {
	case KindExternalGo, KindExternalNPM, KindStdlib:
		return true
	default:
		return false
	}
}

func sourceImportImporters(g *Graph, id NodeID) []NodeID {
	if g == nil {
		return nil
	}
	var out []NodeID
	for _, e := range g.edges {
		if e.To == id && e.Kind == EdgeSourceImport {
			out = appendUniqueNodeID(out, e.From)
		}
	}
	return out
}

func sourceImportImports(g *Graph, id NodeID) []NodeID {
	if g == nil {
		return nil
	}
	var out []NodeID
	for _, e := range g.edges {
		if e.From == id && e.Kind == EdgeSourceImport {
			out = appendUniqueNodeID(out, e.To)
		}
	}
	return out
}

func appendUniqueNodeID(list []NodeID, id NodeID) []NodeID {
	for _, v := range list {
		if v == id {
			return list
		}
	}
	return append(list, id)
}
