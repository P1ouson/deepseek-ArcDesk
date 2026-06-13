package callgraph

import (
	"slices"
	"strings"
	"time"
)

// CallGraph is an in-memory Wails call graph snapshot.
type CallGraph struct {
	Root            string
	Nodes           map[NodeID]*Node
	Out             map[NodeID][]NodeID
	In              map[NodeID][]NodeID
	BridgeByMethod  map[string][]NodeID
	MethodMap       map[string]NodeID
	Warnings        []ParseWarning
	BuiltAt         time.Time
	Stats           Stats
	edges           []Edge
}

// NewGraph returns an empty graph for workspace root.
func NewGraph(root string) *CallGraph {
	return &CallGraph{
		Root:           root,
		Nodes:          make(map[NodeID]*Node),
		Out:            make(map[NodeID][]NodeID),
		In:             make(map[NodeID][]NodeID),
		BridgeByMethod: make(map[string][]NodeID),
		MethodMap:      make(map[string]NodeID),
	}
}

// AddNode inserts or replaces a node.
func (g *CallGraph) AddNode(n *Node) {
	if g == nil || n == nil || n.ID == "" {
		return
	}
	if g.Nodes == nil {
		g.Nodes = make(map[NodeID]*Node)
	}
	g.Nodes[n.ID] = n
}

// AddEdge records a directed edge and updates adjacency.
func (g *CallGraph) AddEdge(from, to NodeID, kind EdgeKind) {
	if g == nil || from == "" || to == "" {
		return
	}
	for _, e := range g.edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return
		}
	}
	g.edges = append(g.edges, Edge{From: from, To: to, Kind: kind})
	if g.Out == nil {
		g.Out = make(map[NodeID][]NodeID)
	}
	if g.In == nil {
		g.In = make(map[NodeID][]NodeID)
	}
	g.Out[from] = appendUnique(g.Out[from], to)
	g.In[to] = appendUnique(g.In[to], from)
}

// Node returns the node for id.
func (g *CallGraph) Node(id NodeID) (*Node, bool) {
	if g == nil {
		return nil, false
	}
	n, ok := g.Nodes[id]
	return n, ok
}

// EdgeCount returns the number of edges.
func (g *CallGraph) EdgeCount() int {
	if g == nil {
		return 0
	}
	return len(g.edges)
}

// RebuildIndexes recomputes BridgeByMethod and MethodMap from nodes/edges.
func (g *CallGraph) RebuildIndexes() {
	if g == nil {
		return
	}
	if g.BridgeByMethod == nil {
		g.BridgeByMethod = make(map[string][]NodeID)
	} else {
		clear(g.BridgeByMethod)
	}
	if g.MethodMap == nil {
		g.MethodMap = make(map[string]NodeID)
	} else {
		clear(g.MethodMap)
	}

	for id, n := range g.Nodes {
		if n == nil {
			continue
		}
		switch n.Kind {
		case KindBridgeCall:
			method := bridgeMethodName(n.Name)
			if method != "" {
				g.BridgeByMethod[method] = appendUnique(g.BridgeByMethod[method], id)
			}
		case KindGoBind:
			method := goBindShortName(n.Name)
			full := n.Name
			if method != "" {
				if _, ok := g.MethodMap[method]; !ok {
					g.MethodMap[method] = id
				}
			}
			if full != "" {
				g.MethodMap[full] = id
			}
		}
	}
}

func bridgeMethodName(name string) string {
	return strings.TrimPrefix(name, "app.")
}

func goBindShortName(name string) string {
	if i := strings.LastIndex(name, "."); i >= 0 {
		return name[i+1:]
	}
	return name
}

func appendUnique(list []NodeID, id NodeID) []NodeID {
	if slices.Contains(list, id) {
		return list
	}
	return append(list, id)
}

func nodeSnapshot(n *Node) NodeSnapshot {
	if n == nil {
		return NodeSnapshot{}
	}
	return NodeSnapshot{
		ID:   n.ID,
		Kind: n.Kind,
		Name: n.Name,
		File: n.File,
		Line: n.Line,
	}
}
