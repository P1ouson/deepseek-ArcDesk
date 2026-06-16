package boot

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/codegraph"
	"arcdesk/internal/config"
	"arcdesk/internal/dependency"
	"arcdesk/internal/envaware"
	"arcdesk/internal/event"
	"arcdesk/internal/hook"
	"arcdesk/internal/plugin"
	"arcdesk/internal/repomap"
	"arcdesk/internal/tool"
	"arcdesk/internal/workspacerefresh"
)

// WorkspaceKit holds workspace-scoped resources shared across desktop tabs:
// config, code indexes, environment probe, and the MCP plugin host. Sessions
// still get their own tool registry and controller; kit supplies the heavy
// infrastructure once per workspace.
type WorkspaceKit struct {
	Root       string
	Cfg        *config.Config
	PluginHost *plugin.Host
	LazySpecs  []plugin.Spec
	BgSpecs    []plugin.Spec
	DepIdx     *dependency.Index
	CgIdx      *callgraph.Index
	RefreshHost *workspacerefresh.Host
	EnvSnap    envaware.Snapshot
}

// RegisterInto wires kit MCP tools into a per-session registry.
func (k *WorkspaceKit) RegisterInto(reg *tool.Registry, ctx context.Context) {
	if k == nil || reg == nil {
		return
	}
	for _, t := range k.PluginHost.ConnectedTools() {
		reg.Add(t)
	}
	registerDeferred := func(specs []plugin.Spec, kick bool) {
		for _, s := range specs {
			cs, _ := plugin.LoadCachedSchema(s.Name, plugin.SpecFingerprint(s))
			for _, t := range plugin.LazyToolset(s, cs, k.PluginHost, reg, ctx, kick) {
				reg.Add(t)
			}
		}
	}
	registerDeferred(k.LazySpecs, false)
	registerDeferred(k.BgSpecs, true)
}

// Close releases shared MCP connections and indexes.
func (k *WorkspaceKit) Close() {
	if k == nil {
		return
	}
	if k.PluginHost != nil {
		k.PluginHost.Close()
	}
}

// PrepareWorkspaceKit warms workspace infrastructure for desktop tab reuse.
func PrepareWorkspaceKit(ctx context.Context, root string, stderr io.Writer, deferEagerMCP bool) (*WorkspaceKit, error) {
	if stderr == nil {
		stderr = os.Stderr
	}
	root = strings.TrimSpace(root)
	if root == "" {
		if wd, err := os.Getwd(); err == nil {
			root = wd
		}
	}
	cfg, err := config.LoadForRoot(root)
	if err != nil {
		return nil, err
	}

	kit := &WorkspaceKit{Root: root, Cfg: cfg}

	if cfg.Dependency.ShouldIndex(dependency.Discoverable(root)) {
		if idx, err := dependency.Open(root); err == nil {
			kit.DepIdx = idx
			if err := idx.EnsureReady(ctx); err != nil {
				slog.Debug("dependency: kit ensure", "err", err)
			}
		}
	}

	if cfg.Callgraph.ShouldIndex(callgraph.Discoverable(root)) {
		var catalog callgraph.ModuleCatalog
		if kit.DepIdx != nil {
			catalog = callgraph.NewDependencyCatalog(kit.DepIdx)
		}
		if idx, err := callgraph.Open(root, catalog); err == nil {
			kit.CgIdx = idx
			if err := idx.EnsureReady(ctx); err != nil {
				slog.Debug("callgraph: kit ensure", "err", err)
			}
			if kit.DepIdx != nil {
				kit.DepIdx.SetBridgeImpactAnalyzer(newBridgeImpactAdapter(idx))
			}
		}
	}

	kit.RefreshHost = workspacerefresh.NewHost(root, cfg, kit.DepIdx, kit.CgIdx)
	if cfg.WorkspaceRefresh.ShouldEnable() {
		kit.RefreshHost.RefreshBackground(ctx)
	} else {
		if kit.DepIdx != nil {
			go func() {
				if err := kit.DepIdx.RefreshIfStale(context.WithoutCancel(ctx)); err != nil {
					slog.Debug("dependency: kit background refresh", "err", err)
				}
			}()
		}
		if kit.CgIdx != nil {
			go func() {
				if err := kit.CgIdx.RefreshIfStale(context.WithoutCancel(ctx)); err != nil {
					slog.Debug("callgraph: kit background refresh", "err", err)
				}
			}()
		}
		if cfg.Reporag.ShouldEnable() {
			go func() {
				if err := repomap.EnsureReady(root); err != nil {
					slog.Debug("repomap kit ensure", "root", root, "err", err)
				}
				if err := repomap.RefreshIfStale(root); err != nil {
					slog.Debug("repomap kit refresh", "root", root, "err", err)
				}
			}()
		}
	}

	kit.EnvSnap = envaware.Probe(ctx, root)

	pluginHost := plugin.NewHost()
	plugins, _ := filterTrustedMCPPlugins(root, cfg.Plugins, "")
	eagerEntries, lazyEntries, bgEntries := partitionByTier(autoStartPlugins(plugins))

	budget := plugin.DefaultStartupBudget()
	kept := eagerEntries[:0]
	for _, e := range eagerEntries {
		rec := plugin.Recommend(e.Name, budget, 0)
		if rec.Demote {
			lazyEntries = append(lazyEntries, e)
			continue
		}
		kept = append(kept, e)
	}
	eagerEntries = kept

	eagerSpecs := PluginSpecs(eagerEntries)
	lazySpecs := PluginSpecs(lazyEntries)
	bgSpecs := PluginSpecs(bgEntries)

	if cfg.Codegraph.Enabled {
		bin, ok := codegraph.Resolve(cfg.Codegraph.Path)
		if ok {
			spec := plugin.Spec{
				Name:              "codegraph",
				Command:           bin,
				Args:              []string{"serve", "--mcp"},
				Dir:               root,
				ReadOnlyToolNames: codegraph.ReadOnlyToolNames(),
			}
			warm := codegraph.Initialized(root)
			if err := codegraph.EnsureInit(ctx, bin, root); err == nil {
				if strings.TrimSpace(cfg.Codegraph.Tier) == "" {
					if warm {
						eagerSpecs = append(eagerSpecs, spec)
					} else {
						bgSpecs = append(bgSpecs, spec)
					}
				} else {
					switch cfg.Codegraph.ResolvedTier() {
					case "eager":
						eagerSpecs = append(eagerSpecs, spec)
					case "background":
						bgSpecs = append(bgSpecs, spec)
					default:
						lazySpecs = append(lazySpecs, spec)
					}
				}
			}
		}
	}

	if deferEagerMCP && len(eagerSpecs) > 0 {
		lazySpecs = append(lazySpecs, eagerSpecs...)
		eagerSpecs = nil
	}

	if len(eagerSpecs) > 0 {
		host, ptools := plugin.StartAvailable(ctx, eagerSpecs)
		if host != nil {
			pluginHost = host
			_ = ptools
			go pluginHost.StartPhaseB(ctx, event.Discard)
		}
	}

	kit.PluginHost = pluginHost
	kit.LazySpecs = lazySpecs
	kit.BgSpecs = bgSpecs

	_ = hook.IsTrusted(root, "")
	return kit, nil
}

// WarmWorkspaceIndexes kicks dependency/callgraph background refresh without MCP.
func WarmWorkspaceIndexes(ctx context.Context, root string) {
	cfg, err := config.LoadForRoot(root)
	if err != nil {
		return
	}
	root = strings.TrimSpace(root)
	if root == "" {
		return
	}
	if cfg.WorkspaceRefresh.ShouldEnable() {
		var depIdx *dependency.Index
		var cgIdx *callgraph.Index
		if cfg.Dependency.ShouldIndex(dependency.Discoverable(root)) {
			depIdx, _ = dependency.Open(root)
		}
		if cfg.Callgraph.ShouldIndex(callgraph.Discoverable(root)) {
			var catalog callgraph.ModuleCatalog
			if depIdx != nil {
				catalog = callgraph.NewDependencyCatalog(depIdx)
			}
			cgIdx, _ = callgraph.Open(root, catalog)
		}
		workspacerefresh.NewHost(root, cfg, depIdx, cgIdx).RefreshBackground(ctx)
		return
	}
	if cfg.Dependency.ShouldIndex(dependency.Discoverable(root)) {
		if idx, err := dependency.Open(root); err == nil {
			go func() {
				_ = idx.EnsureReady(context.WithoutCancel(ctx))
				_ = idx.RefreshIfStale(context.WithoutCancel(ctx))
			}()
		}
	}
	if cfg.Callgraph.ShouldIndex(callgraph.Discoverable(root)) {
		var catalog callgraph.ModuleCatalog
		if idx, err := dependency.Open(root); err == nil {
			catalog = callgraph.NewDependencyCatalog(idx)
		}
		if idx, err := callgraph.Open(root, catalog); err == nil {
			go func() {
				_ = idx.EnsureReady(context.WithoutCancel(ctx))
				_ = idx.RefreshIfStale(context.WithoutCancel(ctx))
			}()
		}
	}
}
