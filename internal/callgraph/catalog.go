package callgraph

import "context"

// ModuleCatalog supplies module/file classification from the dependency index.
// Mirrors dependency.ModuleCatalog without importing that package.
type ModuleCatalog interface {
	ResolveFile(relPath string) (moduleID string, ok bool)
	ModuleKind(moduleID string) (kind string, ok bool)
	EnsureReady(ctx context.Context) error
	Status() (nodeCount, edgeCount int, buildMethod string)
}

// noopCatalog is used when dependency index is unavailable.
type noopCatalog struct{}

func (noopCatalog) ResolveFile(string) (string, bool) { return "", false }
func (noopCatalog) ModuleKind(string) (string, bool)  { return "", false }
func (noopCatalog) EnsureReady(context.Context) error { return nil }
func (noopCatalog) Status() (int, int, string)        { return 0, 0, "" }
