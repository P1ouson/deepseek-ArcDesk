package dependency

import (
	"fmt"
	"sort"
)

// computeCycles finds source-import cycles separately for Go and JS nodes.
func computeCycles(g *Graph) []Cycle {
	if g == nil {
		return nil
	}
	var out []Cycle
	for _, lang := range []string{"go", "js"} {
		ids := internalNodesForLang(g, lang)
		if len(ids) == 0 {
			continue
		}
		out = append(out, tarjanCycles(g, ids, lang)...)
	}
	return out
}

func internalNodesForLang(g *Graph, lang string) []NodeID {
	var ids []NodeID
	for id, n := range g.Nodes {
		if !isInternalKind(n.Kind) {
			continue
		}
		if nodeLang(id, n) != lang {
			continue
		}
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return ids
}

func nodeLang(id NodeID, n *Node) string {
	switch n.Kind {
	case KindInternalJS, KindExternalNPM, KindWorkspaceNPM:
		return "js"
	case KindInternalGo, KindExternalGo, KindStdlib:
		return "go"
	default:
		switch id.Realm() {
		case realmJS, realmNpm:
			return "js"
		default:
			return "go"
		}
	}
}

func tarjanCycles(g *Graph, nodes []NodeID, lang string) []Cycle {
	allowed := map[NodeID]bool{}
	for _, id := range nodes {
		allowed[id] = true
	}

	index := 0
	stack := []NodeID{}
	onStack := map[NodeID]bool{}
	indices := map[NodeID]int{}
	lowlink := map[NodeID]int{}
	var cycles []Cycle

	var strongConnect func(v NodeID)
	strongConnect = func(v NodeID) {
		indices[v] = index
		lowlink[v] = index
		index++
		stack = append(stack, v)
		onStack[v] = true

		for _, w := range sourceImportImports(g, v) {
			if w == v {
				continue // skip self-imports (barrel index artifacts)
			}
			if !allowed[w] {
				continue
			}
			if _, ok := indices[w]; !ok {
				strongConnect(w)
				if lowlink[w] < lowlink[v] {
					lowlink[v] = lowlink[w]
				}
			} else if onStack[w] && indices[w] < lowlink[v] {
				lowlink[v] = indices[w]
			}
		}

		if lowlink[v] == indices[v] {
			var scc []NodeID
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				onStack[w] = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			if len(scc) > 1 {
				sort.Slice(scc, func(i, j int) bool { return scc[i] < scc[j] })
				cycles = append(cycles, annotateCycle(g, scc, lang))
			}
		}
	}

	for _, v := range nodes {
		if _, ok := indices[v]; !ok {
			strongConnect(v)
		}
	}
	return cycles
}

func annotateCycle(g *Graph, scc []NodeID, lang string) Cycle {
	c := Cycle{
		Ring:     scc,
		Lang:     lang,
		Severity: "error",
	}
	if hub := detectBarrelHub(g, scc); hub != "" {
		c.Hub = hub
		c.Severity = "warning"
		c.Hint = fmt.Sprintf("barrel hub pattern detected: %s aggregates all sub-packages", hubDisplayName(g, hub))
	}
	return c
}

// detectBarrelHub finds a node imported by every other member of the SCC.
// When several qualify, prefer the one with the most in-SCC importers.
func detectBarrelHub(g *Graph, scc []NodeID) NodeID {
	if len(scc) < 2 {
		return ""
	}
	inSCC := map[NodeID]bool{}
	for _, id := range scc {
		inSCC[id] = true
	}

	var best NodeID
	bestIn := -1
	for _, hub := range scc {
		if !isBarrelHub(g, hub, scc, inSCC) {
			continue
		}
		in := inSCCImportCount(g, hub, inSCC)
		if in > bestIn || (in == bestIn && (best == "" || hub < best)) {
			best, bestIn = hub, in
		}
	}
	return best
}

func isBarrelHub(g *Graph, hub NodeID, scc []NodeID, inSCC map[NodeID]bool) bool {
	for _, v := range scc {
		if v == hub {
			continue
		}
		if !inSCC[v] || !hasSourceImportEdge(g, v, hub) {
			return false
		}
	}
	return true
}

func inSCCImportCount(g *Graph, hub NodeID, inSCC map[NodeID]bool) int {
	n := 0
	for from := range inSCC {
		if from != hub && hasSourceImportEdge(g, from, hub) {
			n++
		}
	}
	return n
}

func hasSourceImportEdge(g *Graph, from, to NodeID) bool {
	if g == nil {
		return false
	}
	for _, e := range g.edges {
		if e.From == from && e.To == to && e.Kind == EdgeSourceImport {
			return true
		}
	}
	return false
}

func hubDisplayName(g *Graph, hub NodeID) string {
	if g != nil {
		if n := g.Nodes[hub]; n != nil && n.Name != "" {
			return n.Name
		}
	}
	return string(hub)
}
