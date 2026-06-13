package dependency

import (
	"context"
	"strings"
)

// ModuleCatalog is the subset of Index that cross-realm indexes (e.g. callgraph) need.
// Defined in dependency to avoid circular imports; callgraph declares a compatible interface.
type ModuleCatalog interface {
	// ResolveFile returns the module NodeID for a repo-relative file path.
	ResolveFile(relPath string) (moduleID string, ok bool)
	// ModuleKind returns the catalog kind of a module ("go", "js", "gomod", "npm", "bridge").
	ModuleKind(moduleID string) (kind string, ok bool)
	// EnsureReady blocks until the dependency index is loaded.
	EnsureReady(ctx context.Context) error
	// Status returns brief index stats.
	Status() (nodeCount, edgeCount int, buildMethod string)
}

// ModuleCatalog returns idx as a ModuleCatalog adapter.
func (idx *Index) ModuleCatalog() ModuleCatalog {
	if idx == nil {
		return nil
	}
	return &catalogAdapter{idx: idx}
}

type catalogAdapter struct {
	idx *Index
}

func (a *catalogAdapter) ResolveFile(relPath string) (string, bool) {
	if a == nil || a.idx == nil {
		return "", false
	}
	relPath = normalizeSlash(relPath)
	a.idx.mu.RLock()
	defer a.idx.mu.RUnlock()
	if a.idx.graph == nil {
		return "", false
	}
	id, err := resolveID(a.idx.graph, relPath)
	if err != nil {
		return "", false
	}
	return string(id), true
}

func (a *catalogAdapter) ModuleKind(moduleID string) (string, bool) {
	if a == nil || a.idx == nil {
		return "", false
	}
	a.idx.mu.RLock()
	defer a.idx.mu.RUnlock()
	if a.idx.graph == nil {
		return "", false
	}
	n, ok := a.idx.graph.Node(NodeID(strings.TrimSpace(moduleID)))
	if !ok || n == nil {
		return "", false
	}
	return moduleKindFromNode(n)
}

func (a *catalogAdapter) EnsureReady(ctx context.Context) error {
	if a == nil || a.idx == nil {
		return ErrIndexNotReady
	}
	return a.idx.EnsureReady(ctx)
}

func (a *catalogAdapter) Status() (int, int, string) {
	if a == nil || a.idx == nil {
		return 0, 0, ""
	}
	a.idx.mu.RLock()
	defer a.idx.mu.RUnlock()
	if a.idx.graph == nil {
		return 0, 0, ""
	}
	g := a.idx.graph
	return len(g.Nodes), len(g.edges), string(g.BuildMethod)
}

func moduleKindFromNode(n *Node) (string, bool) {
	if n == nil {
		return "", false
	}
	switch n.Kind {
	case KindInternalGo:
		return "go", true
	case KindInternalJS:
		return "js", true
	case KindExternalGo, KindStdlib:
		return "gomod", true
	case KindExternalNPM, KindWorkspaceNPM:
		return "npm", true
	case KindBridge:
		return "bridge", true
	default:
		return "", false
	}
}
