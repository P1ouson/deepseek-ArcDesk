package dependency

import "strconv"

// computeImpact precomputes reverse importer layers and forward external deps.
func computeImpact(g *Graph) {
	if g == nil {
		return
	}
	if g.Impact == nil {
		g.Impact = make(map[NodeID]ImpactLayers)
	}
	for id, n := range g.Nodes {
		if !isInternalKind(n.Kind) {
			continue
		}
		g.Impact[id] = computeImpactFor(g, id)
	}
}

func computeImpactFor(g *Graph, source NodeID) ImpactLayers {
	var layers ImpactLayers
	layers.Direct, layers.Transitive = reverseInternalImporters(g, source)
	layers.External = forwardExternalImports(g, source)
	return layers
}

func reverseInternalImporters(g *Graph, source NodeID) (direct []NodeID, transitive [][]NodeID) {
	type item struct {
		id  NodeID
		dist int
	}
	seen := map[NodeID]int{source: 0}
	queue := []item{}
	for _, imp := range sourceImportImporters(g, source) {
		n := g.Nodes[imp]
		if n == nil || !isInternalKind(n.Kind) {
			continue
		}
		if _, ok := seen[imp]; ok {
			continue
		}
		seen[imp] = 1
		queue = append(queue, item{id: imp, dist: 1})
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.dist == 1 {
			direct = appendUniqueNodeID(direct, cur.id)
		} else {
			idx := cur.dist - 2
			for len(transitive) <= idx {
				transitive = append(transitive, nil)
			}
			transitive[idx] = appendUniqueNodeID(transitive[idx], cur.id)
		}
		for _, next := range sourceImportImporters(g, cur.id) {
			n := g.Nodes[next]
			if n == nil || !isInternalKind(n.Kind) {
				continue
			}
			if _, ok := seen[next]; ok {
				continue
			}
			seen[next] = cur.dist + 1
			queue = append(queue, item{id: next, dist: cur.dist + 1})
		}
	}
	return direct, transitive
}

func forwardExternalImports(g *Graph, source NodeID) []NodeID {
	var external []NodeID
	seen := map[NodeID]bool{source: true}
	queue := []NodeID{source}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, dep := range sourceImportImports(g, cur) {
			n := g.Nodes[dep]
			if n == nil {
				continue
			}
			if isExternalKind(n.Kind) {
				external = appendUniqueNodeID(external, dep)
				continue
			}
			if isInternalKind(n.Kind) && !seen[dep] {
				seen[dep] = true
				queue = append(queue, dep)
			}
		}
	}
	return external
}

// AffectedBy returns the precomputed impact view for id.
func AffectedBy(g *Graph, id NodeID) (ImpactResult, error) {
	return AffectedByWithAnalyzer(g, id, nil)
}

// AffectedByWithAnalyzer returns impact including optional cross-realm UI nodes.
func AffectedByWithAnalyzer(g *Graph, id NodeID, analyzer BridgeImpactAnalyzer) (ImpactResult, error) {
	if g == nil {
		return ImpactResult{}, ErrIndexNotReady
	}
	if _, ok := g.Nodes[id]; !ok {
		return ImpactResult{}, ErrNodeNotFound
	}
	layers, ok := g.Impact[id]
	if !ok {
		layers = computeImpactFor(g, id)
	}
	return formatImpactResult(g, id, layers, analyzer), nil
}

func formatImpactResult(g *Graph, source NodeID, layers ImpactLayers, analyzer BridgeImpactAnalyzer) ImpactResult {
	res := ImpactResult{
		Source:     source,
		CrossRealm: []ImpactEntry{},
	}
	res.Layers.Direct = entriesFor(g, layers.Direct, 1)
	for i, ring := range layers.Transitive {
		res.Layers.Transitive = append(res.Layers.Transitive, entriesFor(g, ring, i+2)...)
	}
	res.Layers.External = entriesFor(g, layers.External, 0)
	if analyzer != nil && analyzer.Available() {
		res.CrossRealm = crossRealmEntries(g, source, analyzer)
	}
	res.Hint = impactHint(g, source, layers)
	return res
}

func crossRealmEntries(g *Graph, source NodeID, analyzer BridgeImpactAnalyzer) []ImpactEntry {
	methods := bridgeMethodsForNode(g, source)
	if len(methods) == 0 {
		return []ImpactEntry{}
	}
	seen := map[string]bool{}
	var out []ImpactEntry
	for _, method := range methods {
		uiNodes, err := analyzer.AffectedUI(method)
		if err != nil {
			continue
		}
		for _, ui := range uiNodes {
			key := ui.ID + "|" + ui.Name
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, ImpactEntry{
				ID:   NodeID(ui.ID),
				Name: ui.Name,
				Kind: Kind(ui.Kind),
			})
		}
	}
	return out
}

func bridgeMethodsForNode(g *Graph, source NodeID) []string {
	n := g.Nodes[source]
	if n == nil {
		return nil
	}
	if n.Meta.BridgeMethod != "" {
		return []string{n.Meta.BridgeMethod}
	}
	if n.Kind == KindBridge {
		return []string{n.Name}
	}
	return nil
}

func entriesFor(g *Graph, ids []NodeID, distance int) []ImpactEntry {
	out := make([]ImpactEntry, 0, len(ids))
	for _, id := range ids {
		n := g.Nodes[id]
		if n == nil {
			continue
		}
		out = append(out, ImpactEntry{
			ID:       id,
			Name:     n.Name,
			Kind:     n.Kind,
			Distance: distance,
		})
	}
	return out
}

func impactHint(g *Graph, source NodeID, layers ImpactLayers) string {
	n := g.Nodes[source]
	name := source.String()
	if n != nil && n.Name != "" {
		name = n.Name
	}
	total := len(layers.Direct)
	for _, ring := range layers.Transitive {
		total += len(ring)
	}
	if total == 0 && len(layers.External) == 0 {
		return "leaf module: " + name
	}
	return fmtHint(name, len(layers.Direct), total, len(layers.External))
}

func fmtHint(name string, direct, totalInternal, external int) string {
	if external > 0 {
		return name + ": " + strconv.Itoa(direct) + " direct importer(s), " + strconv.Itoa(totalInternal) + " internal total, " + strconv.Itoa(external) + " external dep(s)"
	}
	return name + ": " + strconv.Itoa(direct) + " direct importer(s), " + strconv.Itoa(totalInternal) + " internal total"
}
