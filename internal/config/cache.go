package config

import (
	"os"
	"path/filepath"
	"sync"
)

type configCacheEntry struct {
	cfg     *Config
	sources map[string]int64 // path → mod time (unix nano); 0 means absent
}

var (
	configCacheMu sync.Mutex
	configCache   = map[string]*configCacheEntry{}
)

func configCacheKey(root string) string {
	root = resolveRoot(root)
	if abs, err := filepath.Abs(root); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(root)
}

func configSourcePaths(root string) []string {
	root = resolveRoot(root)
	var paths []string
	if uc := userConfigPath(); uc != "" {
		paths = append(paths, uc)
	}
	paths = append(paths, projectConfigPathForRoot(root))
	mcpFile := mcpJSONFile
	if root != "." {
		mcpFile = filepath.Join(root, mcpJSONFile)
	}
	paths = append(paths, mcpFile)
	if p := legacyConfigPath(); p != "" {
		paths = append(paths, p)
	}
	dotEnv := ".env"
	if root != "" && root != "." {
		dotEnv = filepath.Join(root, ".env")
	}
	paths = append(paths, dotEnv)
	if p := UserCredentialsPath(); p != "" {
		paths = append(paths, p)
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".env"))
	}
	return paths
}

func sourceFingerprints(paths []string) map[string]int64 {
	out := make(map[string]int64, len(paths))
	for _, path := range paths {
		fi, err := os.Stat(path)
		if err != nil {
			out[path] = 0
			continue
		}
		out[path] = fi.ModTime().UnixNano()
	}
	return out
}

func fingerprintsMatch(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

// InvalidateConfigCache drops cached config for root. Pass "" to clear all.
func InvalidateConfigCache(root string) {
	configCacheMu.Lock()
	defer configCacheMu.Unlock()
	if root == "" {
		configCache = map[string]*configCacheEntry{}
		return
	}
	delete(configCache, configCacheKey(root))
}

func loadForRootUncached(root string) (*Config, error) {
	root = resolveRoot(root)
	loadDotEnvForRoot(root)
	cfg := Default()

	projectTOML := projectConfigPathForRoot(root)

	var tomlSources []string
	if uc := userConfigPath(); uc != "" {
		tomlSources = append(tomlSources, uc)
	}
	tomlSources = append(tomlSources, projectTOML)
	sawConfigFile := false
	for _, path := range tomlSources {
		if _, err := os.Stat(path); err == nil {
			sawConfigFile = true
		}
		if err := mergeFile(cfg, path); err != nil {
			return nil, err
		}
	}
	plugins, err := mergeTOMLPlugins(tomlSources)
	if err != nil {
		return nil, err
	}
	cfg.Plugins = plugins

	mcpFile := mcpJSONFile
	if root != "." {
		mcpFile = filepath.Join(root, mcpJSONFile)
	}
	entries, err := loadMCPJSON(mcpFile)
	if err != nil {
		return nil, err
	}
	cfg.mergeMCPJSON(entries)

	cfg.mergeMCPJSON(loadLegacyMCP(legacyConfigPath()))
	normalizeLegacyEffort(cfg)
	normalizeEffortConfig(cfg)
	backfillDeepSeekPro(cfg)
	dedupeRedundantProviders(cfg)
	pruneUnconfiguredProviders(cfg)
	if !sawConfigFile {
		cfg.Codegraph.Enabled = false
		disabled := false
		cfg.Dependency.Enabled = &disabled
	}
	return cfg, nil
}
