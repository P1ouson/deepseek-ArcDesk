package dependency

import (
	"slices"
	"time"
)

// Graph is an in-memory module dependency graph. It is safe to read from multiple
// goroutines only when the owning Index holds RLock; mutations require Index Lock.
type Graph struct {
	Root        string                    `json:"root"`
	Nodes       map[NodeID]*Node          `json:"nodes"`
	Out         map[NodeID][]NodeID       `json:"out"`
	In          map[NodeID][]NodeID       `json:"in"`
	Impact      map[NodeID]ImpactLayers   `json:"impact,omitempty"`
	Cycles      []Cycle                   `json:"cycles,omitempty"`
	Conflicts   []VersionConflict         `json:"conflicts,omitempty"`
	Files       map[string]NodeID         `json:"files,omitempty"`
	Orphans     []NodeID                  `json:"orphans,omitempty"`
	ParseErrors []ParseError              `json:"parseErrors,omitempty"`
	BuildMethod BuildMethod               `json:"buildMethod,omitempty"`
	BuiltAt     time.Time                 `json:"builtAt"`
	Stats       Stats                     `json:"stats"`
	edges       []Edge                    // dedup aid; not persisted separately
}

// NewGraph returns an empty graph rooted at workspace abs path.
func NewGraph(root string) *Graph {
	return &Graph{
		Root:        root,
		Nodes:       make(map[NodeID]*Node),
		Out:         make(map[NodeID][]NodeID),
		In:          make(map[NodeID][]NodeID),
		Impact:      make(map[NodeID]ImpactLayers),
		Files:       make(map[string]NodeID),
		ParseErrors: nil,
	}
}

// AddNode inserts or replaces a node.
func (g *Graph) AddNode(n *Node) {
	if g == nil || n == nil {
		return
	}
	if g.Nodes == nil {
		g.Nodes = make(map[NodeID]*Node)
	}
	g.Nodes[n.ID] = n
}

// AddEdge records a directed edge and updates Out/In adjacency lists.
func (g *Graph) AddEdge(from, to NodeID, kind EdgeKind, file string) {
	if g == nil || from == "" || to == "" {
		return
	}
	for _, e := range g.edges {
		if e.From == from && e.To == to && e.Kind == kind {
			if file != "" && !slices.Contains(e.Files, file) {
				for i := range g.edges {
					if g.edges[i].From == from && g.edges[i].To == to && g.edges[i].Kind == kind {
						g.edges[i].Files = append(g.edges[i].Files, file)
						break
					}
				}
			}
			return
		}
	}
	e := Edge{From: from, To: to, Kind: kind}
	if file != "" {
		e.Files = []string{file}
	}
	g.edges = append(g.edges, e)
	g.Out[from] = appendUnique(g.Out[from], to)
	g.In[to] = appendUnique(g.In[to], from)
}

// RemoveNode deletes a node and all adjacency references to it.
func (g *Graph) RemoveNode(id NodeID) {
	if g == nil || id == "" {
		return
	}
	delete(g.Nodes, id)
	delete(g.Out, id)
	delete(g.In, id)
	delete(g.Impact, id)

	for from, outs := range g.Out {
		g.Out[from] = removeID(outs, id)
	}
	for to, ins := range g.In {
		g.In[to] = removeID(ins, id)
	}

	filtered := g.edges[:0]
	for _, e := range g.edges {
		if e.From != id && e.To != id {
			filtered = append(filtered, e)
		}
	}
	g.edges = filtered

	for path, owner := range g.Files {
		if owner == id {
			delete(g.Files, path)
		}
	}
	g.Orphans = removeID(g.Orphans, id)
}

// ImportsOf returns direct dependencies of id (O(1) map + slice copy).
func (g *Graph) ImportsOf(id NodeID) []NodeID {
	if g == nil {
		return nil
	}
	return slices.Clone(g.Out[id])
}

// ImportedBy returns direct importers of id (O(1) map + slice copy).
func (g *Graph) ImportedBy(id NodeID) []NodeID {
	if g == nil {
		return nil
	}
	return slices.Clone(g.In[id])
}

// Node returns the node for id.
func (g *Graph) Node(id NodeID) (*Node, bool) {
	if g == nil {
		return nil, false
	}
	n, ok := g.Nodes[id]
	return n, ok
}

func appendUnique(list []NodeID, id NodeID) []NodeID {
	if slices.Contains(list, id) {
		return list
	}
	return append(list, id)
}

func removeID(list []NodeID, id NodeID) []NodeID {
	if len(list) == 0 {
		return list
	}
	out := list[:0]
	for _, v := range list {
		if v != id {
			out = append(out, v)
		}
	}
	return out
}
