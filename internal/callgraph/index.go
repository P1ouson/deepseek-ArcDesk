package callgraph

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/reporeuse"
)

// Index is the query entry point for the Wails call graph.
type Index struct {
	mu          sync.RWMutex
	root        string
	graph       *CallGraph
	meta        *Meta
	catalog     ModuleCatalog
	symbolQuery SymbolQuery
	forceStale  bool
	loadOnce    sync.Once
	loadErr     error
}

// Open loads a persisted call graph when present.
func Open(root string, catalog ModuleCatalog) (*Index, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, errors.New("workspace root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	if catalog == nil {
		catalog = noopCatalog{}
	}
	idx := &Index{root: abs, catalog: catalog}
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
	return idx, idx.loadErr
}

// EnsureReady builds when missing or stale.
func (i *Index) EnsureReady(ctx context.Context) error {
	if i == nil {
		return errors.New("nil index")
	}
	_ = i.catalog.EnsureReady(ctx)
	i.mu.RLock()
	ready := i.graph != nil && !i.forceStale && !CheckStale(i.root, i.meta)
	i.mu.RUnlock()
	if ready {
		return nil
	}
	return i.RefreshIfStale(ctx)
}

// RefreshIfStale rebuilds the index when stale.
func (i *Index) RefreshIfStale(ctx context.Context) error {
	if i == nil {
		return errors.New("nil index")
	}
	mu := refreshLockFor(i.root)
	if !mu.TryLock() {
		return nil
	}
	defer mu.Unlock()

	i.mu.Lock()
	defer i.mu.Unlock()

	if i.graph != nil && i.meta != nil && !i.forceStale && !CheckStale(i.root, i.meta) {
		return nil
	}
	if ctx != nil {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}

	if i.graph != nil && i.meta != nil && !i.forceStale {
		newHead := gitHead(i.root)
		newFP := ComputeFingerprint(i.root)
		if reporeuse.HeadChangedFingerprintStable(i.meta.GitHead, newHead, i.meta.Fingerprint, newFP) {
			paths, err := reporeuse.ChangedFilesBetween(i.root, i.meta.GitHead, newHead)
			if err == nil && !reporeuse.PathsAffectCallgraph(paths) {
				return i.bumpGitHeadLocked()
			}
		}
	}

	g, meta, err := BuildGraph(BuildOptions{Root: i.root, Catalog: i.catalog})
	if err != nil {
		return err
	}
	dir, err := ProjectDir(i.root)
	if err != nil {
		return err
	}
	if err := SaveIndex(g, dir); err != nil {
		return err
	}
	if err := SaveMeta(meta, dir); err != nil {
		return err
	}
	i.graph = g
	i.meta = meta
	i.forceStale = false
	return nil
}

// InvalidateFiles marks the index stale.
func (i *Index) InvalidateFiles(paths []string) error {
	if i == nil {
		return errors.New("nil index")
	}
	_ = paths
	i.mu.Lock()
	i.forceStale = true
	i.mu.Unlock()
	return nil
}

func (i *Index) bumpGitHeadLocked() error {
	head := gitHead(i.root)
	if i.meta == nil {
		i.meta = &Meta{IndexVersion: IndexVersion}
	}
	i.meta.GitHead = head
	i.meta.GeneratedAt = time.Now().UTC()
	i.meta.Fingerprint = ComputeFingerprint(i.root)
	dir, err := ProjectDir(i.root)
	if err != nil {
		return err
	}
	if err := SaveMeta(i.meta, dir); err != nil {
		return err
	}
	i.forceStale = false
	return nil
}

// SetSymbolQuery injects a CodeGraph-backed symbol query for trace_forward extension.
func (i *Index) SetSymbolQuery(sq SymbolQuery) {
	if i == nil {
		return
	}
	i.mu.Lock()
	defer i.mu.Unlock()
	i.symbolQuery = sq
}

func (i *Index) traceOpts(opts TraceOptions) TraceOptions {
	i.mu.RLock()
	sq := i.symbolQuery
	i.mu.RUnlock()
	if opts.SymbolQuery == nil {
		opts.SymbolQuery = sq
	}
	return opts
}

// TraceForward traces from path#symbol forward.
func (i *Index) TraceForward(ctx context.Context, path, symbol string, opts TraceOptions) ([]CallPath, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	opts = i.traceOpts(opts)
	opts.SymbolContext = ctx
	id, err := ResolveNodeID(g, path, symbol)
	if err != nil {
		return nil, err
	}
	return TraceForward(g, id, opts), nil
}

// TraceBackward traces from Go method name backward.
func (i *Index) TraceBackward(ctx context.Context, goMethod string, opts TraceOptions) ([]CallPath, error) {
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	opts = i.traceOpts(opts)
	gobind, ok := g.MethodMap[goMethod]
	if !ok {
		gobind, ok = g.MethodMap["App."+goMethod]
	}
	if !ok {
		return nil, ErrNodeNotFound
	}
	return TraceBackward(g, gobind, opts), nil
}

// FindBridge finds paths between frontend node and go method.
func (i *Index) FindBridge(ctx context.Context, path string, line int, goMethod string) ([]CallPath, error) {
	_ = ctx
	g, err := i.graphForRead()
	if err != nil {
		return nil, err
	}
	path = normalizeSlash(path)
	var frontend NodeID
	if line > 0 {
		for id, n := range g.Nodes {
			if n != nil && n.File == path && n.Line == line {
				frontend = id
				break
			}
		}
	}
	if frontend == "" {
		frontend, err = ResolveNodeID(g, path, "")
		if err != nil {
			return nil, err
		}
	}
	return FindBridgePath(g, frontend, goMethod)
}

// Root returns the workspace root path.
func (i *Index) Root() string {
	if i == nil {
		return ""
	}
	return i.root
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

// Status returns index stats.
func (i *Index) Status() (Stats, error) {
	g, err := i.graphForRead()
	if err != nil {
		return Stats{}, err
	}
	stats := g.Stats
	stats.NodeCount = len(g.Nodes)
	stats.EdgeCount = g.EdgeCount()
	stats.Stale = i.forceStale || CheckStale(i.root, i.meta)
	return stats, nil
}

func (i *Index) graphForRead() (*CallGraph, error) {
	i.mu.RLock()
	defer i.mu.RUnlock()
	if i.graph == nil {
		return nil, ErrIndexNotReady
	}
	return i.graph, nil
}

var refreshLocks sync.Map

func refreshLockFor(root string) *sync.Mutex {
	v, _ := refreshLocks.LoadOrStore(root, &sync.Mutex{})
	return v.(*sync.Mutex)
}
