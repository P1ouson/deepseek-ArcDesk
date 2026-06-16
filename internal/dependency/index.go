package dependency

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
)

// Index is the query entry point for a workspace dependency graph.
type Index struct {
	mu             sync.RWMutex
	root           string
	graph          *Graph
	meta           *Meta
	bridgeAnalyzer BridgeImpactAnalyzer
	forceStale     bool
	loadOnce       sync.Once
	loadErr        error
}

// Open loads a persisted index when present. Call EnsureReady or RefreshIfStale
// to build when missing or stale.
func Open(root string) (*Index, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("workspace root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	idx := &Index{root: abs}
	idx.loadOnce.Do(func() {
		dir, err := ProjectDir(abs)
		if err != nil {
			idx.loadErr = err
			return
		}
		g, err := LoadIndex(dir)
		if err != nil {
			if !errors.Is(err, ErrIndexNotFound) {
				idx.loadErr = err
			}
			return
		}
		meta, err := LoadMeta(dir)
		if err != nil && !errors.Is(err, ErrIndexNotFound) {
			idx.loadErr = err
			return
		}
		idx.graph = g
		idx.meta = meta
	})
	if idx.loadErr != nil {
		return nil, idx.loadErr
	}
	return idx, nil
}

// EnsureReady builds the index when it is missing or stale.
func (i *Index) EnsureReady(ctx context.Context) error {
	if i == nil {
		return errors.New("nil index")
	}
	i.mu.RLock()
	ready := i.graph != nil && !i.forceStale && !CheckStale(i.root, i.meta)
	i.mu.RUnlock()
	if ready {
		return nil
	}
	return i.RefreshIfStale(ctx)
}

// SetBridgeImpactAnalyzer wires callgraph cross-realm impact analysis.
func (i *Index) SetBridgeImpactAnalyzer(a BridgeImpactAnalyzer) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.bridgeAnalyzer = a
}

// AffectedBy returns precomputed impact for a node id.
func (i *Index) AffectedBy(id NodeID) (ImpactResult, error) {
	g, err := i.graphForRead()
	if err != nil {
		return ImpactResult{}, err
	}
	i.mu.RLock()
	analyzer := i.bridgeAnalyzer
	i.mu.RUnlock()
	return AffectedByWithAnalyzer(g, id, analyzer)
}

// AffectedByPath resolves a repo-relative file or import path to impact.
func (i *Index) AffectedByPath(relPath string) (ImpactResult, error) {
	g, err := i.graphForRead()
	if err != nil {
		return ImpactResult{}, err
	}
	id, err := resolveID(g, relPath)
	if err != nil {
		return ImpactResult{}, err
	}
	return AffectedBy(g, id)
}

// ImportsOf returns direct dependencies of id.
func (i *Index) ImportsOf(id NodeID) ([]NodeID, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	if _, ok := g.Nodes[id]; !ok {
		return nil, ErrNodeNotFound
	}
	return g.ImportsOf(id), nil
}

// ImportedBy returns direct importers of id.
func (i *Index) ImportedBy(id NodeID) ([]NodeID, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	if _, ok := g.Nodes[id]; !ok {
		return nil, ErrNodeNotFound
	}
	return g.ImportedBy(id), nil
}

// FindCycles returns precomputed source-import cycles.
func (i *Index) FindCycles() ([]Cycle, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	if len(g.Cycles) == 0 {
		return nil, nil
	}
	out := make([]Cycle, len(g.Cycles))
	copy(out, g.Cycles)
	return out, nil
}

// VersionConflicts returns manifest version conflicts.
func (i *Index) VersionConflicts() ([]VersionConflict, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	if len(g.Conflicts) == 0 {
		return nil, nil
	}
	out := make([]VersionConflict, len(g.Conflicts))
	copy(out, g.Conflicts)
	return out, nil
}

// ResolveID maps a repo-relative path or import/package name to a node id.
func (i *Index) ResolveID(path string) (NodeID, error) {
	g, err := i.graphForRead()
	if err != nil {
		return "", err
	}
	return resolveID(g, path)
}

// MetaSnapshot returns a copy of index freshness metadata (may be nil).
func (i *Index) MetaSnapshot() *Meta {
	if i == nil {
		return nil
	}
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.meta == nil {
		return nil
	}
	cp := *i.meta
	return &cp
}

// Status returns index statistics and health.
func (i *Index) Status() (Stats, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.graph == nil {
		return Stats{}, ErrIndexNotReady
	}
	stats := i.graph.Stats
	stats.OrphanCount = len(i.graph.Orphans)
	stats.ParseErrorCount = len(i.graph.ParseErrors)
	stats.ParseErrorSample = sampleParseErrors(i.graph.ParseErrors, 10)
	stats.Stale = i.forceStale || CheckStale(i.root, i.meta)
	return stats, nil
}

// Root returns the workspace root path.
func (i *Index) Root() string {
	if i == nil {
		return ""
	}
	return i.root
}

// NodeName returns the display name for id, or string(id) when unknown.
func (i *Index) NodeName(id NodeID) string {
	g, err := i.graphForRead()
	if err != nil {
		return string(id)
	}
	if n := g.Nodes[id]; n != nil && n.Name != "" {
		return n.Name
	}
	return string(id)
}

// NodeKind returns the kind for id, or empty when unknown.
func (i *Index) NodeKind(id NodeID) Kind {
	g, err := i.graphForRead()
	if err != nil {
		return ""
	}
	if n := g.Nodes[id]; n != nil {
		return n.Kind
	}
	return ""
}

func (i *Index) graphForRead() (*Graph, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.graph == nil {
		return nil, ErrIndexNotReady
	}
	return i.graph, nil
}

func resolveID(g *Graph, path string) (NodeID, error) {
	path = normalizeSlash(strings.TrimSpace(path))
	if path == "" {
		return "", errors.New("path required")
	}
	if id, ok := g.Files[path]; ok {
		return id, nil
	}
	candidate := NewGoID(path)
	if _, ok := g.Nodes[candidate]; ok {
		return candidate, nil
	}
	candidate = NewJSID(path)
	if _, ok := g.Nodes[candidate]; ok {
		return candidate, nil
	}
	var nameMatches []NodeID
	var prefixMatches []NodeID
	for id, n := range g.Nodes {
		if n.Name == path || string(id) == path {
			nameMatches = append(nameMatches, id)
		}
		if n.Dir != "" && (n.Dir == path || strings.HasPrefix(path, n.Dir+"/")) {
			prefixMatches = append(prefixMatches, id)
		}
	}
	if len(nameMatches) == 1 {
		return nameMatches[0], nil
	}
	if len(nameMatches) > 1 {
		return "", errors.New("ambiguous path: multiple name matches")
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], nil
	}
	if len(prefixMatches) > 1 {
		return "", errors.New("ambiguous path: multiple prefix matches")
	}
	return "", ErrNodeNotFound
}
