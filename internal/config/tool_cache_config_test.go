package config

import (
	"path/filepath"
	"runtime"
	"testing"
)

func repoRootForConfigTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func TestToolCacheNormalizeDefaultsOn(t *testing.T) {
	cfg, err := LoadForRoot(repoRootForConfigTest(t))
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ToolCache.ShouldNormalize() {
		t.Fatal("expected tool_cache normalize default true")
	}
}

func TestToolCacheEnabledFromProjectTOML(t *testing.T) {
	root := repoRootForConfigTest(t)
	InvalidateConfigCache(root)
	cfg, err := LoadForRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.ToolCache.ShouldEnable() {
		t.Fatalf("expected tool_cache enabled from repo arcdesk.toml, Enabled=%v", cfg.ToolCache.Enabled)
	}
}
