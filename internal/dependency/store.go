package dependency

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

const (
	dependencySubdir = "dependency"
	indexFileName    = "index.json"
	metaFileName     = "meta.json"
)

// ErrIndexNotFound is returned when index.json does not exist.
var ErrIndexNotFound = errors.New("dependency index not found")

// ErrIndexCorrupt is returned when index.json cannot be decoded.
var ErrIndexCorrupt = errors.New("dependency index corrupt")

// ErrIndexNotReady is returned when the index has not been built yet.
var ErrIndexNotReady = errors.New("dependency index not ready")

// ErrNodeNotFound is returned when a graph node does not exist.
var ErrNodeNotFound = errors.New("dependency node not found")

// ProjectDir returns <workspace>/.arcdesk/dependency, creating it when needed.
func ProjectDir(workspace string) (string, error) {
	root := strings.TrimSpace(workspace)
	if root == "" {
		return "", fmt.Errorf("workspace required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(abs, ".arcdesk", dependencySubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func indexPath(dir string) string  { return filepath.Join(dir, indexFileName) }
func metaPath(dir string) string   { return filepath.Join(dir, metaFileName) }

// indexSnapshot is the on-disk JSON shape for index.json.
type indexSnapshot struct {
	Version     int                      `json:"version"`
	Root        string                   `json:"root"`
	BuiltAt     time.Time                `json:"builtAt"`
	BuildMethod BuildMethod              `json:"buildMethod,omitempty"`
	Nodes       map[string]*Node         `json:"nodes"`
	Out         map[string][]string      `json:"out"`
	In          map[string][]string      `json:"in"`
	Edges       []EdgeSnapshot           `json:"edges,omitempty"`
	Impact      map[string]ImpactLayers  `json:"impact,omitempty"`
	Cycles      []Cycle                  `json:"cycles,omitempty"`
	Conflicts   []VersionConflict        `json:"conflicts,omitempty"`
	Files       map[string]string        `json:"files,omitempty"`
	Orphans     []string                 `json:"orphans,omitempty"`
	ParseErrors []ParseError             `json:"parseErrors,omitempty"`
	Stats       Stats                    `json:"stats"`
}

// SaveIndex atomically writes index.json for g into dir.
func SaveIndex(g *Graph, dir string) error {
	if g == nil {
		return fmt.Errorf("graph is nil")
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("directory required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	snap := graphToSnapshot(g)
	b, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal index: %w", err)
	}
	return atomicWrite(filepath.Join(dir, indexFileName), b)
}

// LoadIndex reads index.json from dir and reconstructs a Graph.
func LoadIndex(dir string) (*Graph, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("directory required")
	}
	path := indexPath(dir)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrIndexNotFound
		}
		return nil, err
	}
	var snap indexSnapshot
	if err := json.Unmarshal(b, &snap); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIndexCorrupt, err)
	}
	if snap.Version <= 0 {
		snap.Version = IndexVersion
	}
	return snapshotToGraph(&snap), nil
}

// SaveMeta atomically writes meta.json into dir.
func SaveMeta(m *Meta, dir string) error {
	if m == nil {
		return fmt.Errorf("meta is nil")
	}
	if strings.TrimSpace(dir) == "" {
		return fmt.Errorf("directory required")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal meta: %w", err)
	}
	return atomicWrite(filepath.Join(dir, metaFileName), b)
}

// LoadMeta reads meta.json from dir.
func LoadMeta(dir string) (*Meta, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("directory required")
	}
	path := metaPath(dir)
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrIndexNotFound
		}
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrIndexCorrupt, err)
	}
	return &m, nil
}

func graphToSnapshot(g *Graph) indexSnapshot {
	snap := indexSnapshot{
		Version:     IndexVersion,
		Root:        g.Root,
		BuiltAt:     g.BuiltAt,
		BuildMethod: g.BuildMethod,
		Nodes:       make(map[string]*Node, len(g.Nodes)),
		Out:         make(map[string][]string, len(g.Out)),
		In:          make(map[string][]string, len(g.In)),
		Impact:      make(map[string]ImpactLayers, len(g.Impact)),
		Cycles:      slicesCloneCycles(g.Cycles),
		Conflicts:   slicesCloneConflicts(g.Conflicts),
		Files:       make(map[string]string, len(g.Files)),
		Orphans:     nodeIDsToStrings(g.Orphans),
		ParseErrors: slices.Clone(g.ParseErrors),
		Stats:       g.Stats,
	}
	for id, n := range g.Nodes {
		snap.Nodes[string(id)] = n
	}
	for from, outs := range g.Out {
		snap.Out[string(from)] = nodeIDsToStrings(outs)
	}
	for to, ins := range g.In {
		snap.In[string(to)] = nodeIDsToStrings(ins)
	}
	for id, layers := range g.Impact {
		snap.Impact[string(id)] = layers
	}
	for path, id := range g.Files {
		snap.Files[normalizeSlash(path)] = string(id)
	}
	snap.Edges = edgeSnapshots(g.edges)
	if snap.BuiltAt.IsZero() {
		snap.BuiltAt = time.Now().UTC()
	}
	return snap
}

func snapshotToGraph(snap *indexSnapshot) *Graph {
	g := NewGraph(snap.Root)
	g.BuiltAt = snap.BuiltAt
	g.BuildMethod = snap.BuildMethod
	g.Cycles = slicesCloneCycles(snap.Cycles)
	g.Conflicts = slicesCloneConflicts(snap.Conflicts)
	g.ParseErrors = slices.Clone(snap.ParseErrors)
	g.Stats = snap.Stats
	g.Orphans = stringsToNodeIDs(snap.Orphans)

	for s, n := range snap.Nodes {
		id := NodeID(s)
		if n != nil {
			n.ID = id
			g.Nodes[id] = n
		}
	}
	for s, outs := range snap.Out {
		from := NodeID(s)
		for _, toS := range outs {
			g.Out[from] = append(g.Out[from], NodeID(toS))
		}
	}
	for s, ins := range snap.In {
		to := NodeID(s)
		for _, fromS := range ins {
			g.In[to] = append(g.In[to], NodeID(fromS))
		}
	}
	for s, layers := range snap.Impact {
		g.Impact[NodeID(s)] = layers
	}
	for path, idS := range snap.Files {
		g.Files[normalizeSlash(path)] = NodeID(idS)
	}

	if len(snap.Edges) > 0 {
		g.edges = snapshotsToEdges(snap.Edges)
	} else {
		// Legacy index: rebuild edge list from adjacency without kind/files metadata.
		for from, outs := range g.Out {
			for _, to := range outs {
				g.edges = append(g.edges, Edge{From: from, To: to, Kind: EdgeSourceImport})
			}
		}
	}
	return g
}

func edgeSnapshots(edges []Edge) []EdgeSnapshot {
	if len(edges) == 0 {
		return nil
	}
	out := make([]EdgeSnapshot, len(edges))
	for i, e := range edges {
		out[i] = EdgeSnapshot{
			From:  e.From,
			To:    e.To,
			Kind:  e.Kind,
			Files: slices.Clone(e.Files),
		}
	}
	return out
}

func snapshotsToEdges(snaps []EdgeSnapshot) []Edge {
	out := make([]Edge, len(snaps))
	for i, s := range snaps {
		out[i] = Edge{
			From:  s.From,
			To:    s.To,
			Kind:  s.Kind,
			Files: slices.Clone(s.Files),
		}
	}
	return out
}

func findEdge(edges []Edge, from, to NodeID, kind EdgeKind) (Edge, bool) {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Kind == kind {
			return e, true
		}
	}
	return Edge{}, false
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

func nodeIDsToStrings(ids []NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}

func stringsToNodeIDs(ss []string) []NodeID {
	out := make([]NodeID, len(ss))
	for i, s := range ss {
		out[i] = NodeID(s)
	}
	return out
}

func slicesCloneCycles(in []Cycle) []Cycle {
	if len(in) == 0 {
		return nil
	}
	out := make([]Cycle, len(in))
	copy(out, in)
	return out
}

func slicesCloneConflicts(in []VersionConflict) []VersionConflict {
	if len(in) == 0 {
		return nil
	}
	out := make([]VersionConflict, len(in))
	copy(out, in)
	return out
}
