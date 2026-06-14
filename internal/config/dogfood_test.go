package config

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestDogfoodArcdeskTOML(t *testing.T) {
	root := repoRoot(t)
	t.Chdir(root)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Verification.Disabled() {
		t.Fatal("verification should be enabled in dogfood arcdesk.toml")
	}
	if !cfg.Verification.EnforcesFinalAnswer() {
		t.Fatal("dogfood arcdesk.toml should opt into enforce_final_answer for ArcDesk repo development")
	}
	maxRetries, onFailure := cfg.Verification.ResolvedPolicy()
	if maxRetries != 3 || onFailure != "rollback" {
		t.Fatalf("policy max=%d on_failure=%q", maxRetries, onFailure)
	}
	if len(cfg.Verification.AfterWrite) == 0 {
		t.Fatal("expected after_write checks")
	}
	if !strings.Contains(cfg.Verification.AfterWrite[0], "go build") {
		t.Fatalf("after_write = %v", cfg.Verification.AfterWrite)
	}
	if cfg.Dependency.Disabled() {
		t.Fatal("dependency should be enabled")
	}
	if !cfg.Reporag.ShouldEnable() {
		t.Fatal("reporag should be enabled in dogfood config")
	}
	if !cfg.Verification.AutoDiscoverEnabled() {
		t.Fatal("verification auto_discover should be enabled")
	}
	if cfg.Verification.IncludeE2E != nil && *cfg.Verification.IncludeE2E {
		t.Fatal("include_e2e should be false in dogfood config")
	}
	if !cfg.Runtime.ShouldEnable() {
		t.Fatal("runtime should be enabled for P0 dogfood")
	}
	if !cfg.Callgraph.ShouldIndex(callgraphDiscoverable(root)) {
		t.Fatal("callgraph should be enabled for P0 dogfood")
	}
	if !cfg.Selfdebug.ShouldEnable() {
		t.Fatal("selfdebug should be enabled for P0 dogfood")
	}
	if !cfg.Constraint.ShouldEnable() {
		t.Fatal("constraint should be enabled for P0 dogfood")
	}
	if !cfg.Codegraph.Enabled || !cfg.Codegraph.AutoInstall {
		t.Fatal("codegraph should be enabled with auto_install for P0 dogfood")
	}
}

func callgraphDiscoverable(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "desktop", "wails.json")); err == nil {
		return true
	}
	b, err := os.ReadFile(filepath.Join(root, "go.mod"))
	return err == nil && strings.Contains(string(b), "wails.io")
}

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		t.Fatalf("repo root %q: %v", root, err)
	}
	if _, err := os.Stat(filepath.Join(root, ProjectConfigFile)); err != nil {
		t.Fatalf("missing %s: %v", ProjectConfigFile, err)
	}
	return root
}
