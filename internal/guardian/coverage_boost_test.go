package guardian

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/archrag"
	"arcdesk/internal/constraint"
	"arcdesk/internal/diff"
	"arcdesk/internal/tool"
)

func TestGuardianMockUIAndPreferInternal(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n" +
		"- Prefer editing `internal/counter` instead of duplicating logic in `desktop/`.\n" +
		"- Do not fix UI with hardcoded mock data.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	idx := archrag.NewIndex(dir, nil)
	host := &constraint.Host{Root: dir}
	eng := constraint.NewEngine(host, constraint.DefaultSettings())
	g := New(idx, eng)

	mock := g.CheckPath("desktop/frontend/src/App.tsx", "", "const data = { mock: true }\n")
	if !mock.Blocked {
		t.Fatalf("mock UI should block: %+v", mock)
	}
	warn := g.CheckPath("desktop/app.go", "", "func Save() { return state }\n")
	if warn.Blocked || len(warn.Violations) == 0 {
		t.Fatalf("prefer internal warn: %+v", warn)
	}
}

func TestGuardianCheckEditAndStats(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n- Wails bind methods live under `desktop/` only.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(archrag.NewIndex(dir, nil), nil)
	res := g.CheckEdit("write", diff.Change{
		Path:    "internal/x.go",
		OldText: "",
		NewText: "func (a *App) Bind() {}\n",
	})
	if !res.Blocked {
		t.Fatalf("CheckEdit: %+v", res)
	}
	checks, blocks, warns := g.Stats()
	if checks == 0 || blocks == 0 {
		t.Fatalf("stats checks=%d blocks=%d warns=%d", checks, blocks, warns)
	}
	last := g.LastResult()
	if !last.Blocked {
		t.Fatal("last result")
	}
	if g.SummaryLine() == "" || g.Rules() == nil {
		t.Fatal("summary/rules")
	}
}

func TestGuardianToolsFull(t *testing.T) {
	dir := t.TempDir()
	rulesBody := "## Rules\n\n- Wails bind methods live under `desktop/` only.\n"
	for i := 0; i < 4; i++ {
		rulesBody += "- Rule item " + string(rune('A'+i)) + " about `internal/`.\n"
	}
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(rulesBody), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(archrag.NewIndex(dir, nil), nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, g)

	rules, _ := reg.Get("architecture_guardian_rules")
	out, err := rules.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "SPEC rule") {
		t.Fatalf("rules=%q err=%v", out, err)
	}
	check, _ := reg.Get("architecture_guardian_check")
	out, err = check.Execute(context.Background(), json.RawMessage(`{
		"path":"internal/x.go",
		"old_text":"",
		"new_text":"func x() {}\n"
	}`))
	if err != nil || !strings.Contains(out, "blocked=") {
		t.Fatalf("check=%q err=%v", out, err)
	}
	if _, err = check.Execute(context.Background(), json.RawMessage(`{"path":""}`)); err == nil {
		t.Fatal("path required")
	}
	status, _ := reg.Get("architecture_guardian_status")
	g.CheckPath("internal/x.go", "", "func (a *App) X() {}\n")
	out, err = status.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "Last violations") || !strings.Contains(out, "blocks=1") {
		t.Fatalf("status with violations=%q err=%v", out, err)
	}
}

func TestGuardianNilAndEmptyRules(t *testing.T) {
	if BuildRetryContext(nil, nil) != "" {
		t.Fatal("nil context")
	}
	g := New(nil, nil)
	if len(g.Rules()) != 0 || g.SummaryLine() == "" {
		t.Fatal("nil index guardian")
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, g)
	rules, _ := reg.Get("architecture_guardian_rules")
	out, err := rules.Execute(context.Background(), nil)
	if err != nil || !strings.Contains(out, "No SPEC rules") {
		t.Fatalf("empty rules=%q err=%v", out, err)
	}
}

func TestGuardianRuleHelpers(t *testing.T) {
	if !isRulesSection("Architecture Rules") || !isRulesSection("constraints") {
		t.Fatal("rules section")
	}
	if isRulesSection("intro") {
		t.Fatal("non rules section")
	}
	if classifyRule("must not do x") != "forbid" || classifyRule("required field") != "require" {
		t.Fatal("classify")
	}
	if classifyRule("maybe") != "guidance" {
		t.Fatal("guidance")
	}
	paths := extractPaths("edit `internal/foo` and `desktop/bar`")
	if len(paths) != 2 {
		t.Fatalf("paths=%v", paths)
	}
	if !prefersInternal(SpecRule{Paths: []string{"internal/x"}}) {
		t.Fatal("prefers internal path")
	}
	if !prefersInternal(SpecRule{Text: "keep logic in internal/"}) {
		t.Fatal("prefers internal text")
	}
	if !looksLikeGoBusinessLogic("func Save() int {\n") {
		t.Fatal("business logic func")
	}
	if !looksLikeMockUI("return mock data placeholder") {
		t.Fatal("mock ui")
	}
	if !isFrontendPath("desktop/frontend/src/App.tsx") {
		t.Fatal("frontend path")
	}
	if addedLines("a\nb", "a\nc") != "c" {
		t.Fatalf("added=%q", addedLines("a\nb", "a\nc"))
	}
	if addedLines("same", "same") != "" {
		t.Fatal("no added lines")
	}
}

func TestBuildRetryContextWithConstraintEngine(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n- Keep logic in `internal/`.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	host := &constraint.Host{Root: dir}
	eng := constraint.NewEngine(host, constraint.DefaultSettings())
	eng.CheckPath("desktop/frontend/src/App.css", "", "const mock = true\n")
	g := New(archrag.NewIndex(dir, nil), eng)
	got := BuildRetryContext(g, []string{"desktop/frontend/src/App.css"})
	if !strings.Contains(got, "Architecture Guardian") || !strings.Contains(got, "Constraint System") {
		t.Fatalf("context=%q", got)
	}
}

func TestCheckSpecRulesEmptyAdded(t *testing.T) {
	if checkSpecRules([]SpecRule{{Kind: "forbid", Text: "mock"}}, "desktop/frontend/x.tsx", "") != nil {
		t.Fatal("empty added")
	}
}

func TestEvalSpecRuleNoMatch(t *testing.T) {
	if _, ok := evalSpecRule(SpecRule{Kind: "guidance", Text: "be nice"}, "x.go", "code"); ok {
		t.Fatal("guidance should not fire")
	}
}

func TestGuardianCheckWarnHint(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n- Prefer editing `internal/counter` instead of duplicating logic in `desktop/`.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(archrag.NewIndex(dir, nil), nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, g)
	check, _ := reg.Get("architecture_guardian_check")
	out, err := check.Execute(context.Background(), json.RawMessage(`{
		"path":"desktop/app.go",
		"new_text":"func Save() { return state }\n"
	}`))
	if err != nil || !strings.Contains(out, "blocked=false") || !strings.Contains(out, "internal/") {
		t.Fatalf("warn check=%q err=%v", out, err)
	}
}

func TestGuardianToolMetadata(t *testing.T) {
	g := New(nil, nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, g)
	for _, name := range []string{"architecture_guardian_status", "architecture_guardian_rules", "architecture_guardian_check"} {
		tl, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		if tl.Description() == "" || !tl.ReadOnly() {
			t.Fatalf("metadata %s", name)
		}
	}
}

func TestGuardianRulesHelpersExtended(t *testing.T) {
	if !looksLikeGoBusinessLogic("func Save() int {\n") {
		t.Fatal("plain func")
	}
	if !looksLikeGoBusinessLogic("var state int\n") {
		t.Fatal("state var")
	}
	if !looksLikeMockUI("return hardcoded data values") {
		t.Fatal("hardcoded mock")
	}
	if isFrontendPath("internal/x.go") {
		t.Fatal("not frontend")
	}
	dir := t.TempDir()
	body := "## Intro\n\nNo rules here.\n\n## Rules\n\n* star rule about `internal/`\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	rules := CompileRules(archrag.NewIndex(dir, nil))
	if len(rules) != 1 || rules[0].Kind == "" {
		t.Fatalf("rules=%+v", rules)
	}
}

func TestGuardianNilRulesAndSummary(t *testing.T) {
	var g *Guardian
	if g.Rules() != nil {
		t.Fatal("nil rules")
	}
	if g.LastResult().Blocked {
		t.Fatal("nil last")
	}
	g2 := New(nil, nil)
	if !strings.Contains(g2.SummaryLine(), "rules=0") {
		t.Fatal(g2.SummaryLine())
	}
}

func TestGuardianCheckBlockedMessage(t *testing.T) {
	dir := t.TempDir()
	spec := "## Rules\n\n- Wails bind methods live under `desktop/` only.\n"
	if err := os.WriteFile(filepath.Join(dir, "SPEC.md"), []byte(spec), 0o644); err != nil {
		t.Fatal(err)
	}
	g := New(archrag.NewIndex(dir, nil), nil)
	reg := tool.NewRegistry()
	RegisterTools(reg, g)
	check, _ := reg.Get("architecture_guardian_check")
	out, err := check.Execute(context.Background(), json.RawMessage(`{
		"path":"internal/x.go",
		"new_text":"func (a *App) Bind() {}\n"
	}`))
	if err != nil || !strings.Contains(out, "blocked=true") {
		t.Fatalf("blocked=%q err=%v", out, err)
	}
}

func TestGuardianRegisterNilSafe(t *testing.T) {
	RegisterTools(nil, New(nil, nil))
	RegisterTools(tool.NewRegistry(), nil)
}

func TestCompileRulesNilIndex(t *testing.T) {
	if CompileRules(nil) != nil {
		t.Fatal("nil compile")
	}
}

func TestGuardianNilCheckPath(t *testing.T) {
	var g *Guardian
	if g.CheckPath("x", "", "y").Blocked {
		t.Fatal("nil check")
	}
	if g.CheckEdit("w", diff.Change{}).Blocked {
		t.Fatal("nil edit")
	}
	checks, blocks, warns := g.Stats()
	if checks != 0 || blocks != 0 || warns != 0 {
		t.Fatal("nil stats")
	}
}
