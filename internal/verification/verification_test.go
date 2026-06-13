package verification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/instruction"
	"arcdesk/internal/memory"
	"arcdesk/internal/tool"
)

func TestResolveMergesSourcesAndDedupes(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg := config.VerificationConfig{
		AfterWrite: []string{"go test ./..."},
	}
	mem := []memory.Source{{
		Path: "AGENTS.md",
		Body: "## arcdesk host checks\n- verify: go test ./...\n- verify: go vet ./...\n",
	}}

	checks, policy, ok := Resolve(root, cfg, mem)
	if !ok {
		t.Fatal("expected verification enabled")
	}
	if policy.MaxRetries != defaultMaxRetries || policy.OnFailure != "retry" {
		t.Fatalf("policy = %+v, want max=%d on_failure=retry", policy, defaultMaxRetries)
	}

	seen := map[string]bool{}
	for _, c := range checks {
		seen[c.Command] = true
	}
	for _, want := range []string{"go test ./...", "go vet ./...", "go build ./..."} {
		if !seen[want] {
			t.Fatalf("missing command %q in %#v", want, checks)
		}
	}
}

func TestResolveDisabled(t *testing.T) {
	disabled := false
	cfg := config.VerificationConfig{Enabled: &disabled, AfterWrite: []string{"go test ./..."}}
	checks, _, ok := Resolve(t.TempDir(), cfg, nil)
	if ok || len(checks) != 0 {
		t.Fatalf("Resolve() = (%v, %v), want disabled with no checks", checks, ok)
	}
}

func TestDiscoverGoAndFrontend(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	front := filepath.Join(root, "desktop", "frontend")
	if err := os.MkdirAll(front, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "pnpm-lock.yaml"), []byte("lockfile"), 0o644); err != nil {
		t.Fatal(err)
	}

	checks := Discover(root, config.VerificationConfig{IncludeE2E: boolPtr(true)})
	commands := map[string]bool{}
	for _, c := range checks {
		commands[c.Command] = true
	}
	for _, want := range []string{"go build ./...", "go test ./...", "go vet ./...", "pnpm -C desktop/frontend build"} {
		if !commands[want] {
			t.Fatalf("missing %q in %#v", want, checks)
		}
	}
}

func TestDiscoverVitestAndPlaywright(t *testing.T) {
	root := t.TempDir()
	front := filepath.Join(root, "desktop", "frontend")
	if err := os.MkdirAll(front, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), []byte(`{
		"scripts": {
			"build": "vite build",
			"test": "vitest run",
			"test:e2e": "playwright test"
		}
	}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "pnpm-lock.yaml"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedPlaywrightReady(t, front)

	checks := Discover(root, config.VerificationConfig{IncludeE2E: boolPtr(true)})
	byCat := map[Category][]string{}
	for _, c := range checks {
		byCat[categoryOf(c.Category)] = append(byCat[categoryOf(c.Category)], c.Command)
	}
	if len(byCat[CategoryUnit]) == 0 {
		t.Fatalf("missing unit checks: %#v", checks)
	}
	if len(byCat[CategoryE2E]) == 0 {
		t.Fatalf("missing e2e checks: %#v", checks)
	}
}

func TestDiscoverPlaywrightConfigOnly(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "playwright.config.ts"), []byte("export default {}"), 0o644); err != nil {
		t.Fatal(err)
	}
	seedPlaywrightReady(t, root)
	checks := Discover(root, config.VerificationConfig{IncludeE2E: boolPtr(true)})
	found := false
	for _, c := range checks {
		if categoryOf(c.Category) == CategoryE2E && strings.Contains(c.Command, "playwright test") {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestDiscoverPlaywrightSkippedWithoutBrowsers(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"test:e2e":"playwright test"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "node_modules", "@playwright", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
	emptyCache := filepath.Join(t.TempDir(), "empty")
	if err := os.Mkdir(emptyCache, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", emptyCache)
	checks := Discover(root, config.VerificationConfig{IncludeE2E: boolPtr(true)})
	for _, c := range checks {
		if categoryOf(c.Category) == CategoryE2E {
			t.Fatalf("expected no e2e without browsers, got %#v", checks)
		}
	}
}

func seedPlaywrightReady(t *testing.T, dir string) {
	t.Helper()
	cache := filepath.Join(t.TempDir(), "pw-browsers")
	if err := os.MkdirAll(filepath.Join(cache, "chromium-1124"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PLAYWRIGHT_BROWSERS_PATH", cache)
	if err := os.MkdirAll(filepath.Join(dir, "node_modules", "@playwright", "test"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverE2EDefaultOff(t *testing.T) {
	root := t.TempDir()
	front := filepath.Join(root, "desktop", "frontend")
	if err := os.MkdirAll(front, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), []byte(`{"scripts":{"test:e2e":"playwright test"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := Discover(root, config.VerificationConfig{})
	for _, c := range checks {
		if categoryOf(c.Category) == CategoryE2E {
			t.Fatalf("E2E should be opt-in by default, got %#v", checks)
		}
	}
}

func TestDiscoverTypecheckScript(t *testing.T) {
	root := t.TempDir()
	front := filepath.Join(root, "desktop", "frontend")
	if err := os.MkdirAll(front, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "package.json"), []byte(`{"scripts":{"typecheck":"tsc --noEmit"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(front, "pnpm-lock.yaml"), []byte("lock"), 0o644); err != nil {
		t.Fatal(err)
	}
	checks := Discover(root, config.VerificationConfig{})
	found := false
	for _, c := range checks {
		if c.Command == "pnpm -C desktop/frontend typecheck" {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestDiscoverRespectsIncludeFlags(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	off := false
	checks := Discover(root, config.VerificationConfig{IncludeUnit: &off, IncludeE2E: &off})
	for _, c := range checks {
		cat := categoryOf(c.Category)
		if cat == CategoryUnit || cat == CategoryE2E {
			t.Fatalf("unexpected %s: %#v", cat, checks)
		}
	}
}

func TestBuildRetryContext(t *testing.T) {
	checks := []instruction.VerifyCheck{
		{Command: "go test ./...", Category: string(CategoryUnit)},
		{Command: "pnpm exec playwright test --reporter=line", Category: string(CategoryE2E)},
	}
	got := BuildRetryContext(checks, "go test ./...", "FAIL pkg")
	if !strings.Contains(got, "## Verification Engine") || !strings.Contains(got, "Unit tests") {
		t.Fatalf("got %q", got)
	}
	if BuildRetryContext(checks, "git status", "") != "" {
		t.Fatal("non-verify")
	}
}

func TestVerificationTools(t *testing.T) {
	plan := NewPlan([]instruction.VerifyCheck{
		{Command: "go build ./...", Category: string(CategoryBuild), SourcePath: "auto-discovery"},
		{Command: "go test ./...", Category: string(CategoryUnit), SourcePath: "auto-discovery"},
	}, Policy{MaxRetries: 3, OnFailure: "retry"})
	reg := tool.NewRegistry()
	RegisterTools(reg, plan)
	status, ok := reg.Get("verification_status")
	if !ok {
		t.Fatal("missing status tool")
	}
	out, err := status.Execute(t.Context(), nil)
	if err != nil || !strings.Contains(out, "Verification:") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	planTool, _ := reg.Get("verification_plan")
	planOut, err := planTool.Execute(t.Context(), nil)
	if err != nil || !strings.Contains(planOut, "## build") {
		t.Fatalf("planOut=%q err=%v", planOut, err)
	}
}

func TestIsVerifyCommand(t *testing.T) {
	if !IsVerifyCommand("vitest run") || !IsVerifyCommand("playwright test") {
		t.Fatal("expected verify")
	}
}

func TestResolvePlan(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644)
	plan, ok := ResolvePlan(root, config.VerificationConfig{}, nil)
	if !ok || len(plan.Checks) == 0 {
		t.Fatalf("plan = %+v ok=%v", plan, ok)
	}
}

func TestVerificationConfigResolvedPolicy(t *testing.T) {
	cfg := config.VerificationConfig{MaxRetries: 2, OnFailure: "rollback"}
	max, onFailure := cfg.ResolvedPolicy()
	if max != 2 || onFailure != "rollback" {
		t.Fatalf("ResolvedPolicy() = (%d, %q)", max, onFailure)
	}

	cfg = config.VerificationConfig{}
	max, onFailure = cfg.ResolvedPolicy()
	if max != defaultMaxRetries || onFailure != "retry" {
		t.Fatalf("defaults = (%d, %q)", max, onFailure)
	}
}

func TestDefaultDiscoverOptions(t *testing.T) {
	opts := DefaultDiscoverOptions(config.VerificationConfig{})
	if !opts.IncludeUnit || opts.IncludeE2E {
		t.Fatalf("opts = %+v", opts)
	}
	off := false
	opts = DefaultDiscoverOptions(config.VerificationConfig{IncludeUnit: &off})
	if opts.IncludeUnit {
		t.Fatal("unit should be off")
	}
}

func TestScriptAndExecCommands(t *testing.T) {
	if got := scriptCommand("pnpm", "desktop/frontend", "test", "vitest run"); got != "pnpm -C desktop/frontend test" {
		t.Fatalf("got %q", got)
	}
	if got := execCommand("pnpm", ".", "playwright test --reporter=line"); got != "pnpm exec playwright test --reporter=line" {
		t.Fatalf("got %q", got)
	}
}
