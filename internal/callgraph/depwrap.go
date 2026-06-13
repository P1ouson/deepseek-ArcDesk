package callgraph

import (
	"context"

	"arcdesk/internal/dependency"
)

// NewDependencyCatalog wraps a dependency Index as ModuleCatalog.
func NewDependencyCatalog(idx *dependency.Index) ModuleCatalog {
	if idx == nil {
		return noopCatalog{}
	}
	return &depCatalogWrapper{inner: idx.ModuleCatalog()}
}

type depCatalogWrapper struct {
	inner dependency.ModuleCatalog
}

func (w *depCatalogWrapper) ResolveFile(relPath string) (string, bool) {
	return w.inner.ResolveFile(relPath)
}

func (w *depCatalogWrapper) ModuleKind(moduleID string) (string, bool) {
	return w.inner.ModuleKind(moduleID)
}

func (w *depCatalogWrapper) EnsureReady(ctx context.Context) error {
	return w.inner.EnsureReady(ctx)
}

func (w *depCatalogWrapper) Status() (int, int, string) {
	return w.inner.Status()
}
