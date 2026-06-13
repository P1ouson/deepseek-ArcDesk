package callgraph

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
	callgraphSubdir = "callgraph"
	indexFileName   = "index.json"
	metaFileName    = "meta.json"
)

// ErrIndexNotFound is returned when index.json does not exist.
var ErrIndexNotFound = errors.New("callgraph index not found")

// ErrIndexCorrupt is returned when index.json cannot be decoded.
var ErrIndexCorrupt = errors.New("callgraph index corrupt")

// ErrIndexNotReady is returned when the index has not been built yet.
var ErrIndexNotReady = errors.New("callgraph index not ready")

// ErrNodeNotFound is returned when a graph node does not exist.
var ErrNodeNotFound = errors.New("callgraph node not found")

// ProjectDir returns <workspace>/.arcdesk/callgraph, creating it when needed.
func ProjectDir(workspace string) (string, error) {
	root := strings.TrimSpace(workspace)
	if root == "" {
		return "", fmt.Errorf("workspace required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	dir := filepath.Join(abs, ".arcdesk", callgraphSubdir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func indexPath(dir string) string { return filepath.Join(dir, indexFileName) }
func metaPath(dir string) string  { return filepath.Join(dir, metaFileName) }

type indexSnapshot struct {
	Version        int                 `json:"version"`
	Root           string              `json:"root"`
	BuiltAt        time.Time           `json:"builtAt"`
	Nodes          map[string]*Node    `json:"nodes"`
	Out            map[string][]string `json:"out"`
	In             map[string][]string `json:"in"`
	Edges          []EdgeSnapshot      `json:"edges,omitempty"`
	BridgeByMethod map[string][]string `json:"bridgeByMethod,omitempty"`
	MethodMap      map[string]string   `json:"methodMap,omitempty"`
	Warnings       []ParseWarning      `json:"warnings,omitempty"`
	Stats          Stats               `json:"stats"`
}

// SaveIndex atomically writes index.json.
func SaveIndex(g *CallGraph, dir string) error {
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

// LoadIndex reads index.json from dir.
func LoadIndex(dir string) (*CallGraph, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, fmt.Errorf("directory required")
	}
	b, err := os.ReadFile(indexPath(dir))
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

// SaveMeta atomically writes meta.json.
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
	b, err := os.ReadFile(metaPath(dir))
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

func graphToSnapshot(g *CallGraph) indexSnapshot {
	snap := indexSnapshot{
		Version:        IndexVersion,
		Root:           g.Root,
		BuiltAt:        g.BuiltAt,
		Nodes:          make(map[string]*Node, len(g.Nodes)),
		Out:            make(map[string][]string, len(g.Out)),
		In:             make(map[string][]string, len(g.In)),
		BridgeByMethod: make(map[string][]string, len(g.BridgeByMethod)),
		MethodMap:      make(map[string]string, len(g.MethodMap)),
		Warnings:       slices.Clone(g.Warnings),
		Stats:          g.Stats,
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
	for method, ids := range g.BridgeByMethod {
		snap.BridgeByMethod[method] = nodeIDsToStrings(ids)
	}
	for k, id := range g.MethodMap {
		snap.MethodMap[k] = string(id)
	}
	snap.Edges = edgeSnapshots(g.edges)
	if snap.BuiltAt.IsZero() {
		snap.BuiltAt = time.Now().UTC()
	}
	return snap
}

func snapshotToGraph(snap *indexSnapshot) *CallGraph {
	g := NewGraph(snap.Root)
	g.BuiltAt = snap.BuiltAt
	g.Warnings = slices.Clone(snap.Warnings)
	g.Stats = snap.Stats

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
	for method, ids := range snap.BridgeByMethod {
		for _, idS := range ids {
			g.BridgeByMethod[method] = append(g.BridgeByMethod[method], NodeID(idS))
		}
	}
	for k, idS := range snap.MethodMap {
		g.MethodMap[k] = NodeID(idS)
	}
	if len(snap.Edges) > 0 {
		g.edges = snapshotsToEdges(snap.Edges)
	} else {
		for from, outs := range g.Out {
			for _, to := range outs {
				g.edges = append(g.edges, Edge{From: from, To: to, Kind: EdgeCalls})
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
		out[i] = EdgeSnapshot{From: e.From, To: e.To, Kind: e.Kind}
	}
	return out
}

func snapshotsToEdges(snaps []EdgeSnapshot) []Edge {
	out := make([]Edge, len(snaps))
	for i, s := range snaps {
		out[i] = Edge{From: s.From, To: s.To, Kind: s.Kind}
	}
	return out
}

func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := atomicWriteFile(tmp, data); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// atomicWriteFile is swappable for tests (disk-full simulation).
var atomicWriteFile = func(path string, data []byte) error {
	return os.WriteFile(path, data, 0o644)
}

func nodeIDsToStrings(ids []NodeID) []string {
	out := make([]string, len(ids))
	for i, id := range ids {
		out[i] = string(id)
	}
	return out
}
