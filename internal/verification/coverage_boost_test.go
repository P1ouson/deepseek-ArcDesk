package verification

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/instruction"
	"arcdesk/internal/tool"
)

func TestDiscoverLegacyAlias(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "go.mod"), []byte("module x\n"), 0o644)
	checks := DiscoverLegacy(root)
	if len(checks) == 0 {
		t.Fatal("expected checks")
	}
}

func TestDiscoverEmptyRoot(t *testing.T) {
	if got := Discover("", config.VerificationConfig{}); got != nil {
		t.Fatalf("got %#v", got)
	}
}

func TestPackageBuildCommand(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "web")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "yarn.lock"), []byte("lock"), 0o644)
	got := packageBuildCommand(dir, "web")
	if got != "yarn --cwd web build" {
		t.Fatalf("got %q", got)
	}
}

func TestScriptCommandAllManagers(t *testing.T) {
	cases := []struct {
		prefix, rel, name, body, want string
	}{
		{"pnpm", ".", "build", "vite build", "pnpm build"},
		{"npm run", ".", "build", "vite build", "npm run build"},
		{"npm run", "web", "build", "vite build", "npm run build --prefix web"},
		{"yarn", "web", "build", "vite build", "yarn --cwd web build"},
		{"bun", "web", "build", "vite build", "bun --cwd web run build"},
		{"make", ".", "build", "all", "make build"},
		{"pnpm", "web", "build", "", ""},
	}
	for _, tc := range cases {
		if got := scriptCommand(tc.prefix, tc.rel, tc.name, tc.body); got != tc.want {
			t.Fatalf("%+v: got %q", tc, got)
		}
	}
}

func TestExecCommandAllManagers(t *testing.T) {
	cases := []struct {
		prefix, rel, args, want string
	}{
		{"pnpm", ".", "vitest run", "pnpm exec vitest run"},
		{"pnpm", "web", "vitest run", "pnpm -C web exec vitest run"},
		{"npm run", ".", "playwright test", "npx playwright test"},
		{"npm run", "web", "playwright test", "npm exec --prefix web playwright test"},
		{"yarn", "web", "playwright test", "yarn --cwd web exec playwright test"},
		{"bun", "web", "playwright test", "bun --cwd web x playwright test"},
		{"make", "web", "playwright test", "npx playwright test"},
	}
	for _, tc := range cases {
		if got := execCommand(tc.prefix, tc.rel, tc.args); got != tc.want {
			t.Fatalf("%+v: got %q", tc, got)
		}
	}
}

func TestPackageManagerPrefix(t *testing.T) {
	root := t.TempDir()
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("package.json", `{}`)
	if got := packageManagerPrefix(root); got != "npm run" {
		t.Fatalf("default = %q", got)
	}
	write("pnpm-lock.yaml", "lock")
	if got := packageManagerPrefix(root); got != "pnpm" {
		t.Fatalf("pnpm = %q", got)
	}
	os.Remove(filepath.Join(root, "pnpm-lock.yaml"))
	write("yarn.lock", "lock")
	if got := packageManagerPrefix(root); got != "yarn" {
		t.Fatalf("yarn = %q", got)
	}
	os.Remove(filepath.Join(root, "yarn.lock"))
	write("bun.lockb", "lock")
	if got := packageManagerPrefix(root); got != "bun" {
		t.Fatalf("bun = %q", got)
	}
}

func TestHasVitestConfigVariants(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"vitest.config.ts", "vitest.config.mjs"} {
		_ = os.WriteFile(filepath.Join(root, name), []byte("export default {}"), 0o644)
		if !hasVitestConfig(root) {
			t.Fatalf("missing %s", name)
		}
		os.Remove(filepath.Join(root, name))
	}
}

func TestDiscoverInvalidPackageJSON(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte("{bad"), 0o644)
	if got := Discover(root, config.VerificationConfig{}); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestDiscoverVitestConfigFallback(t *testing.T) {
	root := t.TempDir()
	_ = os.WriteFile(filepath.Join(root, "package.json"), []byte(`{"scripts":{"build":"vite build"}}`), 0o644)
	_ = os.WriteFile(filepath.Join(root, "vitest.config.ts"), []byte("export default {}"), 0o644)
	_ = os.WriteFile(filepath.Join(root, "pnpm-lock.yaml"), []byte("lock"), 0o644)
	checks := Discover(root, config.VerificationConfig{})
	found := false
	for _, c := range checks {
		if strings.Contains(c.Command, "vitest run") {
			found = true
		}
	}
	if !found {
		t.Fatalf("checks = %#v", checks)
	}
}

func TestChecksFromInstructions(t *testing.T) {
	checks := ChecksFromInstructions([]instruction.VerifyCheck{
		{Command: "go build ./...", Category: string(CategoryBuild), SourcePath: discoverSource},
		{Command: "custom", SourcePath: "AGENTS.md", Line: 3},
		{Command: "no-cat", SourcePath: discoverSource},
	})
	if len(checks) != 3 || checks[0].Category != CategoryBuild {
		t.Fatalf("got %#v", checks)
	}
}

func TestRegisterToolsEdgeCases(t *testing.T) {
	RegisterTools(nil, Plan{})
	reg := tool.NewRegistry()
	RegisterTools(reg, Plan{})
	if _, ok := reg.Get("verification_status"); ok {
		t.Fatal("empty plan should not register")
	}
}

func TestVerificationToolMetadata(t *testing.T) {
	plan := NewPlan([]instruction.VerifyCheck{{Command: "go test ./...", Category: string(CategoryUnit)}}, Policy{MaxRetries: 1, OnFailure: "retry"})
	st := verifyStatusTool{plan: plan}
	pt := verifyPlanTool{plan: plan}
	if st.Description() == "" || pt.Description() == "" {
		t.Fatal("descriptions required")
	}
	if !st.ReadOnly() || !pt.ReadOnly() {
		t.Fatal("tools should be read-only")
	}
}

func TestVerifyPlanToolEmpty(t *testing.T) {
	pt := verifyPlanTool{plan: Plan{}}
	out, err := pt.Execute(t.Context(), nil)
	if err != nil || out != "No verification checks configured." {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestBuildRetryContextCategories(t *testing.T) {
	checks := []instruction.VerifyCheck{
		{Command: "go build ./...", Category: string(CategoryBuild)},
		{Command: "go test ./...", Category: string(CategoryUnit)},
		{Command: "pnpm exec playwright test", Category: string(CategoryE2E)},
		{Command: "make lint", Category: string(CategoryCustom)},
	}
	for _, tc := range []struct {
		cmd, want string
	}{
		{"go build ./...", "Compile/build failed"},
		{"go test ./...", "Unit tests failed"},
		{"pnpm exec playwright test", "Behavioral/E2E"},
		{"make lint", "Project verification failed"},
	} {
		got := BuildRetryContext(checks, tc.cmd, strings.Repeat("x", 300))
		if !strings.Contains(got, tc.want) {
			t.Fatalf("cmd=%q got %q", tc.cmd, got)
		}
	}
}

func TestBuildRetryContextPendingCap(t *testing.T) {
	var checks []instruction.VerifyCheck
	for i := 0; i < 10; i++ {
		checks = append(checks, instruction.VerifyCheck{
			Command:  "cmd" + string(rune('a'+i)),
			Category: string(CategoryUnit),
		})
	}
	got := BuildRetryContext(checks, "cmda", "err")
	if strings.Count(got, "[unit]") > 6 {
		t.Fatalf("pending list not capped: %q", got)
	}
}

func TestTruncateLines(t *testing.T) {
	long := strings.Repeat("a", 250)
	many := strings.Join(make([]string, 20), "\n")
	if got := truncateLines(long, 5, 200); !strings.HasSuffix(got, "…") {
		t.Fatalf("got %q", got)
	}
	if got := truncateLines(many, 3, 200); strings.Count(got, "\n") >= 20 {
		t.Fatalf("got %q", got)
	}
}

func TestIsVerifyCommandNegative(t *testing.T) {
	if IsVerifyCommand("") || IsVerifyCommand("echo hello") {
		t.Fatal("expected false")
	}
}

func TestCategoryOfDefault(t *testing.T) {
	if got := categoryOf("unknown"); got != CategoryCustom {
		t.Fatalf("got %q", got)
	}
}

func TestResolvePlanDisabled(t *testing.T) {
	off := false
	plan, ok := ResolvePlan(t.TempDir(), config.VerificationConfig{Enabled: &off}, nil)
	if ok || len(plan.Checks) != 0 {
		t.Fatalf("plan=%+v ok=%v", plan, ok)
	}
}

func TestResolvePlanNoChecks(t *testing.T) {
	_, ok := ResolvePlan(t.TempDir(), config.VerificationConfig{AutoDiscover: boolPtr(false)}, nil)
	if ok {
		t.Fatal("expected disabled when no checks")
	}
}

func boolPtr(v bool) *bool { return &v }
