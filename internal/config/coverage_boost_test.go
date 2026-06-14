package config

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodegraphConfigHelpers(t *testing.T) {
	cg := CodegraphConfig{Enabled: true, Tier: "lazy"}
	if !cg.ShouldAutoStart() {
		t.Fatal("expected auto start when enabled")
	}
	if cg.ResolvedTier() != "lazy" {
		t.Fatalf("tier = %q", cg.ResolvedTier())
	}
}

func TestVerificationConfigHelpers(t *testing.T) {
	disabled := false
	auto := false
	v := VerificationConfig{
		Enabled:      &disabled,
		AutoDiscover: &auto,
		MaxRetries:   0,
		OnFailure:    "rollback",
	}
	if !v.Disabled() {
		t.Fatal("expected disabled")
	}
	if v.AutoDiscoverEnabled() {
		t.Fatal("expected auto discover off")
	}
	max, policy := v.ResolvedPolicy()
	if max != 3 || policy != "rollback" {
		t.Fatalf("policy = %d %q", max, policy)
	}
	ask := VerificationConfig{OnFailure: "ask"}
	_, p := ask.ResolvedPolicy()
	if p != "ask" {
		t.Fatalf("policy = %q", p)
	}
	if v.EnforcesFinalAnswer() {
		t.Fatal("enforce_final_answer should default to false")
	}
	enforce := true
	enforced := VerificationConfig{EnforceFinalAnswer: &enforce}
	if !enforced.EnforcesFinalAnswer() {
		t.Fatal("explicit enforce_final_answer should be true")
	}
}

func TestDependencyCallgraphShouldIndex(t *testing.T) {
	falseVal := false
	dep := DependencyConfig{Enabled: &falseVal}
	if dep.ShouldIndex(true) {
		t.Fatal("explicit disabled should not index")
	}
	trueVal := true
	dep2 := DependencyConfig{Enabled: &trueVal}
	if !dep2.ShouldIndex(false) {
		t.Fatal("explicit enabled should index")
	}
	cg := CallgraphConfig{}
	if !cg.ShouldIndex(true) {
		t.Fatal("default auto discover should index discoverable")
	}
	if cg.Disabled() {
		t.Fatal("default not disabled")
	}
}

func TestProviderDefaultModelAndResolveModel(t *testing.T) {
	c := Default()
	c.Providers = []ProviderEntry{
		{Name: "p1", Kind: "openai", BaseURL: "http://x", Models: []string{"m1", "m2"}, Default: "m2"},
	}
	if got := c.Providers[0].DefaultModel(); got != "m2" {
		t.Fatalf("DefaultModel = %q", got)
	}
	if e, ok := c.ResolveModel("p1/m1"); !ok || e.Model != "m1" {
		t.Fatalf("ResolveModel provider/model = %+v ok=%v", e, ok)
	}
	if e, ok := c.ResolveModel("p1"); !ok || e.Model != "m2" {
		t.Fatalf("ResolveModel provider = %+v ok=%v", e, ok)
	}
	if e, ok := c.ResolveModel("m1"); !ok || e.Model != "m1" {
		t.Fatalf("ResolveModel bare = %+v ok=%v", e, ok)
	}
	if _, ok := c.ResolveModel("ghost"); ok {
		t.Fatal("expected miss")
	}
}

func TestValidateModel(t *testing.T) {
	t.Setenv("ARCDESK_TEST_VALIDATE_KEY", "secret")
	c := Default()
	c.Providers = []ProviderEntry{{
		Name: "testprov", Kind: "openai", BaseURL: "http://localhost",
		Model: "m1", APIKeyEnv: "ARCDESK_TEST_VALIDATE_KEY",
	}}
	c.DefaultModel = "testprov"
	if err := c.Validate("testprov"); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if err := c.Validate("missing"); err == nil {
		t.Fatal("expected unknown model error")
	}
	c.Providers[0].Kind = ""
	if err := c.Validate("testprov"); err == nil {
		t.Fatal("expected kind error")
	}
}

func TestResolveSystemPrompt(t *testing.T) {
	c := Default()
	got, err := c.ResolveSystemPrompt()
	if err != nil || got == "" {
		t.Fatalf("default prompt = %q err=%v", got, err)
	}
	c.Agent.SystemPrompt = "custom"
	got, err = c.ResolveSystemPrompt()
	if err != nil || got != "custom" {
		t.Fatalf("custom prompt = %q err=%v", got, err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "prompt.txt")
	_ = os.WriteFile(path, []byte(" from file "), 0o644)
	c.Agent.SystemPromptFile = path
	got, err = c.ResolveSystemPrompt()
	if err != nil || got != "from file" {
		t.Fatalf("file prompt = %q err=%v", got, err)
	}
}

func TestWriteRootsForRoot(t *testing.T) {
	c := Default()
	c.Sandbox.WorkspaceRoot = "/proj"
	c.Sandbox.AllowWrite = []string{"/extra", ""}
	roots := c.WriteRootsForRoot("/fallback")
	if len(roots) < 2 || roots[0] != "/proj" {
		t.Fatalf("roots = %v", roots)
	}
	c.Sandbox.WorkspaceRoot = ""
	roots = c.WriteRootsForRoot("/fallback")
	if roots[0] != "/fallback" {
		t.Fatalf("roots = %v", roots)
	}
}

func TestSkillCustomPathsAndDisabled(t *testing.T) {
	c := Default()
	c.Skills.Paths = []string{"~/skills", "", "${HOME}/more"}
	paths := c.SkillCustomPaths()
	if len(paths) == 0 {
		t.Fatal("expected paths")
	}
	c.Skills.DisabledSkills = []string{"foo", "foo", "bad name!"}
	names := c.DisabledSkillNames()
	if len(names) != 1 || names[0] != "foo" {
		t.Fatalf("names = %v", names)
	}
	if !c.IsSkillDisabled("Foo") {
		t.Fatal("expected foo disabled")
	}
}

func TestPathHelpers(t *testing.T) {
	_ = UserConfigPath()
	_ = UserCredentialsPath()
	_ = ArchiveDir()
	_ = SessionDir()
	_ = CacheDir()
	_ = MemoryUserDir()
	_ = ProjectConfigPathForRoot("/tmp")
	_ = legacyProjectConfigPathForRoot("/tmp")
	if got := firstExistingPath("", "/nonexistent/path"); got != "" {
		t.Fatalf("got %q", got)
	}
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, ProjectConfigFile)
	_ = os.WriteFile(cfgPath, []byte("[agent]\n"), 0o644)
	if got := SourcePathForRoot(dir); got != cfgPath {
		t.Fatalf("SourcePathForRoot = %q want %q", got, cfgPath)
	}
}

func TestApplyAgentSettings(t *testing.T) {
	c := Default()
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		Temperature:        0.5,
		MaxSteps:           10,
		AutoPlan:           "off",
		SoftCompactRatio:   0.5,
		CompactRatio:       0.7,
		CompactForceRatio:  0.9,
		SystemPrompt:       " hi ",
	}); err != nil {
		t.Fatalf("ApplyAgentSettings: %v", err)
	}
	if c.Agent.SystemPrompt != "hi" {
		t.Fatalf("prompt = %q", c.Agent.SystemPrompt)
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{Temperature: 3}); err == nil {
		t.Fatal("expected temperature error")
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		SoftCompactRatio: 0.9, CompactRatio: 0.5, CompactForceRatio: 0.95,
	}); err == nil {
		t.Fatal("expected ratio order error")
	}
}

func TestSetDesktopAppearancePrefs(t *testing.T) {
	c := Default()
	if err := c.SetDesktopAppearancePrefs(DesktopAppearanceInput{
		BackgroundPreset: "paper",
		ForegroundPreset: "ink",
		TextSize:         "large",
		CodeFontSize:     "default",
		DiffMarker:       "signs",
	}); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDesktopAppearancePrefs(DesktopAppearanceInput{BackgroundPreset: "neon"}); err == nil {
		t.Fatal("expected invalid preset error")
	}
}

func TestSetDesktopCodeReviewAndGit(t *testing.T) {
	c := Default()
	if err := c.SetDesktopCodeReviewSettings("staged", true); err != nil {
		t.Fatal(err)
	}
	if err := c.SetDesktopGitSettings("squash", true, false, "commit msg", "pr msg"); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Git.PRMergeMethod != "squash" {
		t.Fatalf("merge = %q", c.Desktop.Git.PRMergeMethod)
	}
}

func TestImportDesktopLocalPrefs(t *testing.T) {
	c := Default()
	if err := c.ImportDesktopLocalPrefs(DesktopAppearanceInput{
		BackgroundPreset: "paper",
	}, "staged", true, true, true); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Appearance.BackgroundPreset != "paper" {
		t.Fatalf("bg = %q", c.Desktop.Appearance.BackgroundPreset)
	}
}

func TestSetDesktopTerminalShell(t *testing.T) {
	c := Default()
	for _, shell := range []string{"powershell", "cmd", "git-bash", "wsl", "auto"} {
		if err := c.SetDesktopTerminalShell(shell); err != nil {
			t.Fatalf("shell %q: %v", shell, err)
		}
	}
	if err := c.SetDesktopTerminalShell("fish"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSetProviderThinking(t *testing.T) {
	c := Default()
	name := c.Providers[0].Name
	if err := c.SetProviderThinking(name, "Adaptive"); err != nil {
		t.Fatal(err)
	}
	if err := c.SetProviderThinking("missing", "x"); err == nil {
		t.Fatal("expected error")
	}
}

func TestSaveAndSaveForRoot(t *testing.T) {
	c := Default()
	dir := t.TempDir()
	path := filepath.Join(dir, "arcdesk.toml")
	if err := c.SaveTo(path); err != nil {
		t.Fatalf("SaveTo: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
	if err := c.SaveForRoot(dir); err != nil {
		t.Fatalf("SaveForRoot: %v", err)
	}
}

func TestSaveMinimalProjectAutoPlan(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "arcdesk.toml")
	mode, err := SaveMinimalProjectAutoPlan(path, "on")
	if err != nil || mode != "on" {
		t.Fatalf("mode=%q err=%v", mode, err)
	}
	body, _ := os.ReadFile(path)
	if !strings.Contains(string(body), "auto_plan") {
		t.Fatalf("body = %s", body)
	}
}

func TestSaveEmptyPath(t *testing.T) {
	c := Default()
	if err := c.SaveTo(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestValidateCompactRatio(t *testing.T) {
	if err := validateCompactRatio("x", 0); err == nil {
		t.Fatal("expected error for zero")
	}
	if err := validateCompactRatio("x", 1.5); err == nil {
		t.Fatal("expected error for >1")
	}
}

func TestValidateProviderName(t *testing.T) {
	c := Default()
	if err := c.validateProviderName("missing", "field"); err == nil {
		t.Fatal("expected error")
	}
}

func TestPluginShouldAutoStart(t *testing.T) {
	falseVal := false
	e := PluginEntry{AutoStart: &falseVal}
	if e.ShouldAutoStart() {
		t.Fatal("expected false")
	}
}

func TestDesktopTerminalShellAndTheme(t *testing.T) {
	c := Default()
	if got := c.DesktopTerminalShell(); got != "" {
		t.Fatalf("auto shell = %q, want empty", got)
	}
	if err := c.SetDesktopTerminalShell("powershell"); err != nil {
		t.Fatal(err)
	}
	if got := c.DesktopTerminalShell(); got != "powershell" {
		t.Fatalf("shell = %q", got)
	}
}

func TestNormalizeDesktopPresets(t *testing.T) {
	if got := normalizeDesktopBackgroundPreset("PAPER"); got != "paper" {
		t.Fatalf("bg = %q", got)
	}
	if got := normalizeDesktopForegroundPreset("INK"); got != "ink" {
		t.Fatalf("fg = %q", got)
	}
	if got := normalizeDesktopTextSize("LARGE"); got != "large" {
		t.Fatalf("size = %q", got)
	}
	if got := normalizeDesktopDiffMarker("SIGNS"); got != "signs" {
		t.Fatalf("marker = %q", got)
	}
	if got := normalizeDesktopCodeReviewScope("ALL"); got != "all" {
		t.Fatalf("scope = %q", got)
	}
}

func TestBashMode(t *testing.T) {
	c := Default()
	if c.BashMode() != "enforce" {
		t.Fatalf("mode = %q", c.BashMode())
	}
	c.Sandbox.Bash = "off"
	if c.BashMode() != "off" {
		t.Fatalf("mode = %q", c.BashMode())
	}
}

func TestResolveRoot(t *testing.T) {
	if resolveRoot("") != "." {
		t.Fatal("empty root")
	}
	if resolveRoot(".") != "." {
		t.Fatal("dot root")
	}
}

func TestProjectMetaPaths(t *testing.T) {
	if got := projectMetaPath("/tmp", "x"); !strings.Contains(got, ".arcdesk") {
		t.Fatalf("meta = %q", got)
	}
	if got := legacyProjectMetaPath(".", "y"); !strings.Contains(got, ".reasonix") {
		t.Fatalf("legacy meta = %q", got)
	}
}

func TestCommandDirsForRoot(t *testing.T) {
	dirs := CommandDirsForRoot(t.TempDir())
	if len(dirs) == 0 {
		t.Fatal("expected dirs")
	}
}

func TestWriteRoots(t *testing.T) {
	c := Default()
	roots := c.WriteRoots()
	if len(roots) == 0 {
		t.Fatal("expected non-empty write roots")
	}
}

func TestProviderDefaultModelBranches(t *testing.T) {
	if got := (&ProviderEntry{Default: "b", Models: []string{"a", "b"}}).DefaultModel(); got != "b" {
		t.Fatalf("explicit default = %q", got)
	}
	if got := (&ProviderEntry{Models: []string{"a", "b"}}).DefaultModel(); got != "a" {
		t.Fatalf("first model = %q", got)
	}
	if got := (&ProviderEntry{Model: "solo"}).DefaultModel(); got != "solo" {
		t.Fatalf("single model = %q", got)
	}
	if got := (&ProviderEntry{}).DefaultModel(); got != "" {
		t.Fatalf("empty = %q", got)
	}
}

func TestDesktopThemeAndTerminalShell(t *testing.T) {
	c := Default()
	for theme, want := range map[string]string{
		"auto": "auto", "light": "light", "dark": "dark", "": "light", "unknown": "light",
	} {
		c.Desktop.Theme = theme
		if got := c.DesktopTheme(); got != want {
			t.Fatalf("DesktopTheme(%q) = %q, want %q", theme, got, want)
		}
	}
	for in, want := range map[string]string{
		"pwsh": "powershell", "command-prompt": "cmd", "gitbash": "git-bash", "bash": "git-bash",
	} {
		c.Desktop.TerminalShell = in
		if got := c.DesktopTerminalShell(); got != want {
			t.Fatalf("DesktopTerminalShell(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAutoDiscoverEnabledExplicitFalse(t *testing.T) {
	off := false
	if (CallgraphConfig{AutoDiscover: &off}).AutoDiscoverEnabled() {
		t.Fatal("callgraph auto_discover should be off")
	}
	if (DependencyConfig{AutoDiscover: &off}).AutoDiscoverEnabled() {
		t.Fatal("dependency auto_discover should be off")
	}
	if (VerificationConfig{AutoDiscover: &off}).AutoDiscoverEnabled() {
		t.Fatal("verification auto_discover should be off")
	}
	on := true
	if !(CallgraphConfig{AutoDiscover: &on}).AutoDiscoverEnabled() {
		t.Fatal("callgraph auto_discover should be on")
	}
}

func TestCallgraphShouldIndexExplicitEnabled(t *testing.T) {
	on := true
	cg := CallgraphConfig{Enabled: &on}
	if !cg.ShouldIndex(false) {
		t.Fatal("explicit enabled should index even when not discoverable")
	}
}

func TestSourcePathAndSourcePathForRoot(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	dir := t.TempDir()
	t.Chdir(dir)
	if got := SourcePath(); got != "" {
		t.Fatalf("SourcePath with no files = %q, want empty", got)
	}

	userPath := UserConfigPath()
	if err := os.MkdirAll(filepath.Dir(userPath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(userPath, []byte("[agent]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(t.TempDir()); got != userPath {
		t.Fatalf("user fallback = %q, want %q", got, userPath)
	}

	projDir := t.TempDir()
	projPath := filepath.Join(projDir, ProjectConfigFile)
	if err := os.WriteFile(projPath, []byte("[agent]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := SourcePathForRoot(projDir); got != projPath {
		t.Fatalf("project path = %q, want %q", got, projPath)
	}
}

func TestLoadForRootMergesProjectConfig(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	custom := `default_model = "custom"
[[providers]]
name = "custom"
kind = "openai"
base_url = "https://example.com"
model = "m"
api_key_env = "CUSTOM_KEY"
`
	if err := os.WriteFile(filepath.Join(dir, ProjectConfigFile), []byte(custom), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CUSTOM_KEY", "secret")
	cfg, err := LoadForRoot(dir)
	if err != nil {
		t.Fatalf("LoadForRoot: %v", err)
	}
	if cfg.DefaultModel != "custom" {
		t.Fatalf("default_model = %q, want custom", cfg.DefaultModel)
	}
	if len(cfg.Providers) != 1 || cfg.Providers[0].Name != "custom" {
		t.Fatalf("providers = %+v", cfg.Providers)
	}
}

func TestMergeTOMLPlugins(t *testing.T) {
	dir := t.TempDir()
	user := filepath.Join(dir, "user.toml")
	proj := filepath.Join(dir, "proj.toml")
	if err := os.WriteFile(user, []byte(`[[plugins]]
name = "shared"
command = "cmd-user"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(proj, []byte(`[[plugins]]
name = "shared"
command = "cmd-project"
[[plugins]]
name = "project-only"
command = "cmd-b"
`), 0o644); err != nil {
		t.Fatal(err)
	}
	merged, err := mergeTOMLPlugins([]string{user, proj})
	if err != nil {
		t.Fatalf("mergeTOMLPlugins: %v", err)
	}
	if len(merged) != 2 {
		t.Fatalf("merged plugins = %+v, want 2", merged)
	}
	byName := map[string]string{}
	for _, p := range merged {
		byName[p.Name] = p.Command
	}
	if byName["shared"] != "cmd-project" {
		t.Fatalf("later source should win: %+v", byName)
	}
	if byName["project-only"] != "cmd-b" {
		t.Fatalf("project-only plugin missing: %+v", byName)
	}
}

func TestMergeTOMLPluginsInvalidFile(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("not [[valid toml"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := mergeTOMLPlugins([]string{bad}); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestValidateEdgeCases(t *testing.T) {
	t.Setenv("ARCDESK_VALIDATE_KEY", "secret")
	c := Default()
	c.Providers = []ProviderEntry{{
		Name: "testprov", Kind: "openai", BaseURL: "http://localhost",
		Model: "m1", APIKeyEnv: "ARCDESK_VALIDATE_KEY",
	}}
	if err := c.Validate("testprov"); err != nil {
		t.Fatalf("Validate with key: %v", err)
	}
	c.Providers[0].APIKeyEnv = "UNSET_KEY"
	if err := c.Validate("testprov"); err == nil {
		t.Fatal("expected missing API key error")
	}
	c2 := Default()
	c2.Providers = []ProviderEntry{{
		Name: "nobase", Kind: "openai", Model: "m", APIKeyEnv: "ARCDESK_VALIDATE_KEY",
	}}
	if err := c2.Validate("nobase"); err == nil {
		t.Fatal("expected missing base_url error")
	}
}

func TestApplyAgentSettingsSubagentModels(t *testing.T) {
	c := Default()
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		Temperature:        1,
		MaxSteps:           5,
		AutoPlan:           "on",
		SoftCompactRatio:   0.5,
		CompactRatio:       0.7,
		CompactForceRatio:  0.9,
		SystemPrompt:       " prompt ",
		SystemPromptFile:   " /tmp/prompt.txt ",
		OutputStyle:        " concise ",
		AutoPlanClassifier: "deepseek-flash",
		SubagentModel:      "deepseek-pro",
		SubagentModels: map[string]string{
			"explore": "deepseek-flash",
			"":        "deepseek-pro",
			"empty":   "",
		},
	}); err != nil {
		t.Fatalf("ApplyAgentSettings: %v", err)
	}
	if c.Agent.SubagentModels["explore"] != "deepseek-flash" {
		t.Fatalf("subagent_models = %v", c.Agent.SubagentModels)
	}
	if c.Agent.SystemPrompt != "prompt" || c.Agent.OutputStyle != "concise" {
		t.Fatalf("trim failed: prompt=%q style=%q", c.Agent.SystemPrompt, c.Agent.OutputStyle)
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		SoftCompactRatio:  0.5,
		CompactRatio:      0.7,
		CompactForceRatio: 0.9,
		AutoPlan:          "off",
		SubagentModels:    nil,
	}); err != nil {
		t.Fatal(err)
	}
	if c.Agent.SubagentModels != nil {
		t.Fatal("nil subagent_models should clear map")
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		SoftCompactRatio:  0.5,
		CompactRatio:      0.7,
		CompactForceRatio: 0.9,
		AutoPlan:          "off",
		SubagentModels:    map[string]string{"ghost-skill": "missing-provider"},
	}); err == nil {
		t.Fatal("expected unknown subagent model error")
	}
}

func TestApplyAgentSettingsValidationErrors(t *testing.T) {
	c := Default()
	if err := c.ApplyAgentSettings(AgentSettingsInput{MaxSteps: -1}); err == nil {
		t.Fatal("expected max_steps error")
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{AutoPlanClassifier: "ghost"}); err == nil {
		t.Fatal("expected auto_plan_classifier error")
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{SubagentModel: "ghost"}); err == nil {
		t.Fatal("expected subagent_model error")
	}
	if err := c.ApplyAgentSettings(AgentSettingsInput{
		SoftCompactRatio:  0.5,
		CompactRatio:      0.7,
		CompactForceRatio: 0.9,
		AutoPlan:          "off",
		SubagentModels:    map[string]string{},
	}); err != nil {
		t.Fatal(err)
	}
	if c.Agent.SubagentModels != nil {
		t.Fatal("empty subagent_models map should clear to nil")
	}
}

func TestSetDesktopAppearanceAllBranches(t *testing.T) {
	c := Default()
	for _, theme := range []string{"auto", "light", "dark", "", "DARK"} {
		if err := c.SetDesktopAppearance(theme, ""); err != nil {
			t.Fatalf("SetDesktopAppearance(%q): %v", theme, err)
		}
	}
	if err := c.SetDesktopAppearance("dark", "glacier"); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.ThemeStyle != "glacier" {
		t.Fatalf("theme_style = %q", c.Desktop.ThemeStyle)
	}
	if err := c.SetDesktopAppearance("dark", "invalid-style"); err == nil {
		t.Fatal("expected invalid theme style error")
	}
	if err := c.SetDesktopAppearance("neon", "glacier"); err == nil {
		t.Fatal("expected invalid theme error")
	}
}

func TestSetDesktopAppearancePrefsInvalidFields(t *testing.T) {
	c := Default()
	for _, in := range []DesktopAppearanceInput{
		{ForegroundPreset: "neon"},
		{TextSize: "huge"},
		{CodeFontSize: "tiny"},
		{DiffMarker: "arrows"},
	} {
		if err := c.SetDesktopAppearancePrefs(in); err == nil {
			t.Fatalf("expected error for %+v", in)
		}
	}
}

func TestSetDesktopCodeReviewScopeNormalization(t *testing.T) {
	c := Default()
	if err := c.SetDesktopCodeReviewSettings("session", true); err != nil {
		t.Fatal(err)
	}
	if got := c.DesktopCodeReviewSettings().DefaultScope; got != "session" {
		t.Fatalf("scope = %q, want session", got)
	}
	if err := c.SetDesktopCodeReviewSettings("unknown", false); err != nil {
		t.Fatal(err)
	}
	if got := c.DesktopCodeReviewSettings().DefaultScope; got != "all" {
		t.Fatalf("unknown scope = %q, want all", got)
	}
}

func TestImportDesktopLocalPrefsSkipsWhenNonEmpty(t *testing.T) {
	c := Default()
	c.Desktop.Appearance.BackgroundPreset = "charcoal"
	if err := c.ImportDesktopLocalPrefs(DesktopAppearanceInput{
		BackgroundPreset: "paper",
	}, "session", true, true, true); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Appearance.BackgroundPreset != "charcoal" {
		t.Fatalf("appearance should stay charcoal, got %q", c.Desktop.Appearance.BackgroundPreset)
	}

	c2 := Default()
	c2.Desktop.CodeReview.DefaultScope = "git"
	if err := c2.ImportDesktopLocalPrefs(DesktopAppearanceInput{}, "session", true, true, false); err != nil {
		t.Fatal(err)
	}
	if c2.Desktop.CodeReview.DefaultScope != "git" {
		t.Fatalf("code review scope should stay git, got %q", c2.Desktop.CodeReview.DefaultScope)
	}
}

func TestSetDesktopGitSettingsMergeDefault(t *testing.T) {
	c := Default()
	if err := c.SetDesktopGitSettings("merge", false, true, " commit ", " pr "); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Git.PRMergeMethod != "merge" {
		t.Fatalf("merge method = %q", c.Desktop.Git.PRMergeMethod)
	}
	if c.Desktop.Git.CommitInstructions != "commit" || c.Desktop.Git.PRInstructions != "pr" {
		t.Fatalf("instructions not trimmed: %+v", c.Desktop.Git)
	}
}

func TestCanonicalSkillPath(t *testing.T) {
	if got := CanonicalSkillPath("./skills"); got == "" {
		t.Fatal("expected absolute path for relative input")
	}
	if got := CanonicalSkillPath("~/skills"); got == "" {
		t.Fatal("expected expanded home path")
	}
	if got := CanonicalSkillPath("~"); got == "" {
		t.Fatal("expected home dir for ~")
	}
}

func TestSaveWhenSourcePathEmpty(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	dir := t.TempDir()
	t.Chdir(dir)
	c := Default()
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if _, err := os.Stat(ProjectConfigFile); err != nil {
		t.Fatalf("expected %s in cwd: %v", ProjectConfigFile, err)
	}
}

func TestSaveForRootExistingProjectAndUserFallback(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))

	projDir := t.TempDir()
	projPath := filepath.Join(projDir, ProjectConfigFile)
	if err := os.WriteFile(projPath, []byte("[agent]\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c := Default()
	c.Agent.AutoPlan = "on"
	if err := c.SaveForRoot(projDir); err != nil {
		t.Fatalf("SaveForRoot project: %v", err)
	}
	body, err := os.ReadFile(projPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "auto_plan") {
		t.Fatalf("project save missing auto_plan:\n%s", body)
	}

	emptyDir := t.TempDir()
	if err := c.SaveForRoot(emptyDir); err != nil {
		t.Fatalf("SaveForRoot user fallback: %v", err)
	}
	userPath := UserConfigPath()
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected user config at %q: %v", userPath, err)
	}
}

func TestWriteConfigFileCreatesNestedDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "deep", ProjectConfigFile)
	if err := writeConfigFile(path, "# test\n"); err != nil {
		t.Fatalf("writeConfigFile: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestBrandingLegacyAndProjectPaths(t *testing.T) {
	if got := ProjectConfigPathForRoot("/proj"); !strings.HasSuffix(got, ProjectConfigFile) {
		t.Fatalf("project config = %q", got)
	}
	if got := ProjectConfigPathForRoot("."); got != ProjectConfigFile {
		t.Fatalf("dot project config = %q", got)
	}
	if got := legacyProjectConfigPathForRoot("/proj"); !strings.Contains(got, LegacyProjectConfigFile) {
		t.Fatalf("legacy project config = %q", got)
	}
	if got := legacyProjectConfigPathForRoot("."); got != LegacyProjectConfigFile {
		t.Fatalf("legacy dot config = %q", got)
	}
	if got := legacyProjectMetaPath("/r", "leaf"); !strings.Contains(got, LegacyProjectMetaDir) {
		t.Fatalf("legacy meta = %q", got)
	}
	if got := projectMetaPath(".", "leaf"); !strings.Contains(got, ProjectMetaDir) {
		t.Fatalf("meta = %q", got)
	}
	if got := firstExistingPath("", "/missing", filepath.Join(t.TempDir(), "x")); got != "" {
		// only tests skip-empty; create real file below
	}
	exists := filepath.Join(t.TempDir(), "exists.toml")
	_ = os.WriteFile(exists, []byte("x"), 0o644)
	if got := firstExistingPath("/missing", exists); got != exists {
		t.Fatalf("firstExistingPath = %q, want %q", got, exists)
	}
}

func TestResolveSystemPromptDefaultWhenEmpty(t *testing.T) {
	c := Default()
	c.Agent.SystemPrompt = "   "
	got, err := c.ResolveSystemPrompt()
	if err != nil || got != DefaultSystemPrompt {
		t.Fatalf("default prompt = %q err=%v", got, err)
	}
}

func TestValidateProviderNameEmptyAllowed(t *testing.T) {
	c := Default()
	if err := c.validateProviderName("", "field"); err != nil {
		t.Fatalf("empty name should be allowed: %v", err)
	}
}

func TestSetLanguageInvalid(t *testing.T) {
	c := Default()
	if err := c.SetLanguage("fr"); err == nil {
		t.Fatal("expected language error")
	}
	if err := c.SetDesktopLanguage("de"); err == nil {
		t.Fatal("expected desktop language error")
	}
}

func TestIsSkillDisabledWhenNone(t *testing.T) {
	c := Default()
	if c.IsSkillDisabled("review") {
		t.Fatal("no disabled skills")
	}
}

func TestEffortDisplayAndProviderSync(t *testing.T) {
	if EffortDisplay(nil) != "auto" {
		t.Fatal("nil provider should display auto")
	}
	e := &ProviderEntry{Effort: "MAX"}
	if EffortDisplay(e) != "max" {
		t.Fatalf("EffortDisplay = %q", EffortDisplay(e))
	}

	c := Default()
	c.SyncDeepSeekEndpoints("https://api.deepseek.com/v1/models")
	p, ok := c.ProviderByAPIKeyEnv("DEEPSEEK_API_KEY")
	if !ok || !strings.Contains(p.BaseURL, "api.deepseek.com") {
		t.Fatalf("sync endpoints: %+v ok=%v", p, ok)
	}
	ApplyDeepSeekProviderEndpoints(nil)
	ApplyDeepSeekProviderEndpoints(&ProviderEntry{APIKeyEnv: "OTHER"})
	ep := &ProviderEntry{APIKeyEnv: "DEEPSEEK_API_KEY", BaseURL: "https://api.deepseek.com/v1/chat/completions"}
	ApplyDeepSeekProviderEndpoints(ep)
	if ep.BalanceURL == "" {
		t.Fatal("expected balance URL on DeepSeek entry")
	}
}

func TestMCPJSONPathForRoot(t *testing.T) {
	if got := MCPJSONPathForRoot(""); got != ".mcp.json" {
		t.Fatalf("empty root = %q", got)
	}
	if got := MCPJSONPathForRoot("/workspace"); !strings.HasSuffix(got, ".mcp.json") {
		t.Fatalf("workspace path = %q", got)
	}
}

func TestMigrationResultNotice(t *testing.T) {
	r := &MigrationResult{
		From: "old", To: "new", Plugins: 2, KeyToEnv: true,
		Warnings: []string{"check this"},
	}
	n := r.Notice()
	if !strings.Contains(n, "old") || !strings.Contains(n, "MCP") || !strings.Contains(n, "check this") {
		t.Fatalf("notice = %q", n)
	}
}

func TestFetchModelsErrors(t *testing.T) {
	if _, err := (&ProviderEntry{Name: "x"}).FetchModels(context.Background()); err == nil {
		t.Fatal("expected missing base_url error")
	}
	if _, err := (&ProviderEntry{Name: "x", BaseURL: "http://x", APIKeyEnv: "MISSING"}).FetchModels(context.Background()); err == nil {
		t.Fatal("expected missing API key error")
	}
}

func TestSwitchableModelsUsesListedModels(t *testing.T) {
	e := &ProviderEntry{
		BaseURL: "https://relay.example.com/v1",
		Models:  []string{"gpt-4o", "claude-3"},
	}
	got := e.SwitchableModels(context.Background())
	if len(got) != 2 {
		t.Fatalf("SwitchableModels = %v", got)
	}
}

func TestRefreshProviderModelsFromAPIEarlyReturn(t *testing.T) {
	c := Default()
	if err := RefreshProviderModelsFromAPI(c, ""); err != nil {
		t.Fatal(err)
	}
	if err := RefreshProviderModelsFromAPI(c, "NO_SUCH_ENV"); err != nil {
		t.Fatal(err)
	}
}

func TestLoadForEditBadFileUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "bad.toml")
	if err := os.WriteFile(bad, []byte("[[[invalid"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadForEdit(bad)
	if cfg.DefaultModel != Default().DefaultModel {
		t.Fatalf("bad file should fall back to defaults, got %q", cfg.DefaultModel)
	}
}

func TestSaveMinimalProjectAutoPlanInvalidMode(t *testing.T) {
	_, err := SaveMinimalProjectAutoPlan(filepath.Join(t.TempDir(), ProjectConfigFile), "invalid")
	if err == nil {
		t.Fatal("expected invalid auto_plan error")
	}
}

func isolateUserConfigHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("AppData", filepath.Join(home, "AppData"))
	return home
}

func testModelsServer(t *testing.T, ids ...string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			http.NotFound(w, r)
			return
		}
		data := make([]map[string]string, len(ids))
		for i, id := range ids {
			data[i] = map[string]string{"id": id}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": data})
	}))
}

func TestFilterModelsForProviderFallbackToSingleModel(t *testing.T) {
	deepseek := &ProviderEntry{
		BaseURL:   "https://api.deepseek.com",
		APIKeyEnv: "DEEPSEEK_API_KEY",
		Model:     "deepseek-v4-flash",
	}
	if got := FilterModelsForProvider(deepseek, []string{"mimo-v2.5"}); len(got) != 1 || got[0] != "deepseek-v4-flash" {
		t.Fatalf("deepseek fallback = %v", got)
	}
	mimo := &ProviderEntry{
		BaseURL:   "https://token-plan-cn.xiaomimimo.com/v1",
		APIKeyEnv: "MIMO_API_KEY",
		Model:     "mimo-v2.5-pro",
	}
	if got := FilterModelsForProvider(mimo, []string{"deepseek-v4-flash"}); len(got) != 1 || got[0] != "mimo-v2.5-pro" {
		t.Fatalf("mimo fallback = %v", got)
	}
}

func TestModelsForProviderStorageOfficialHost(t *testing.T) {
	p := &ProviderEntry{BaseURL: "https://api.deepseek.com", APIKeyEnv: "DEEPSEEK_API_KEY"}
	raw := []string{"deepseek-v4-flash", "mimo-v2.5-pro"}
	got := ModelsForProviderStorage(p, raw)
	if len(got) != 1 || got[0] != "deepseek-v4-flash" {
		t.Fatalf("storage filter = %v", got)
	}
}

func TestFetchModelsHTTPSuccess(t *testing.T) {
	srv := testModelsServer(t, "alpha", "beta")
	defer srv.Close()
	isolateUserConfigHome(t)
	t.Setenv("FETCH_TEST_KEY", "secret")
	e := &ProviderEntry{Name: "t", BaseURL: srv.URL, APIKeyEnv: "FETCH_TEST_KEY"}
	models, err := e.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 || models[0] != "alpha" {
		t.Fatalf("models = %v", models)
	}
}

func TestSwitchableModelsLiveFetch(t *testing.T) {
	srv := testModelsServer(t, "deepseek-v4-flash", "deepseek-v4-pro")
	defer srv.Close()
	isolateUserConfigHome(t)
	t.Setenv("SWITCH_KEY", "secret")
	e := &ProviderEntry{
		BaseURL:   srv.URL,
		APIKeyEnv: "DEEPSEEK_API_KEY",
		Model:     "deepseek-v4-flash",
	}
	got := e.SwitchableModels(context.Background())
	if len(got) != 2 {
		t.Fatalf("SwitchableModels = %v", got)
	}
}

func TestRefreshProviderModelsFromAPIPopulatesModels(t *testing.T) {
	srv := testModelsServer(t, "deepseek-v4-flash", "deepseek-v4-pro")
	defer srv.Close()
	isolateUserConfigHome(t)
	t.Setenv("REFRESH_KEY", "secret")
	c := Default()
	c.Providers = []ProviderEntry{{
		Name: "deepseek-flash", Kind: "openai",
		BaseURL: srv.URL, Model: "deepseek-v4-flash", APIKeyEnv: "REFRESH_KEY",
	}}
	if err := RefreshProviderModelsFromAPI(c, "REFRESH_KEY"); err != nil {
		t.Fatalf("RefreshProviderModelsFromAPI: %v", err)
	}
	if len(c.Providers[0].Models) != 2 {
		t.Fatalf("models = %v", c.Providers[0].Models)
	}
}

func TestWriteConfigFileInvalidParent(t *testing.T) {
	blocker := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := writeConfigFile(filepath.Join(blocker, "cfg.toml"), "body"); err == nil {
		t.Fatal("expected mkdir error")
	}
}

func TestImportDesktopLocalPrefsImportsEmptyFields(t *testing.T) {
	c := Default()
	if err := c.ImportDesktopLocalPrefs(DesktopAppearanceInput{
		BackgroundPreset: "paper",
	}, "session", true, true, true); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Appearance.BackgroundPreset != "paper" {
		t.Fatalf("appearance = %+v", c.Desktop.Appearance)
	}
	c2 := Default()
	if err := c2.ImportDesktopLocalPrefs(DesktopAppearanceInput{}, "git", false, false, true); err != nil {
		t.Fatal(err)
	}
	if c2.Desktop.CodeReview.DefaultScope != "git" {
		t.Fatalf("code review = %+v", c2.Desktop.CodeReview)
	}
}

func TestVerificationAutoDiscoverExplicitTrue(t *testing.T) {
	on := true
	if !(VerificationConfig{AutoDiscover: &on}).AutoDiscoverEnabled() {
		t.Fatal("expected explicit auto discover on")
	}
	max, policy := (VerificationConfig{MaxRetries: 5, OnFailure: "unknown"}).ResolvedPolicy()
	if max != 5 || policy != "retry" {
		t.Fatalf("policy = %d %q", max, policy)
	}
}

func TestSkillNameKeyInvalidName(t *testing.T) {
	if SkillNameKey("bad name!") != "" {
		t.Fatal("invalid skill name should yield empty key")
	}
	if SkillNameKey("Review") == "" {
		t.Fatal("valid skill name should yield key")
	}
}

func TestIsMimoOfficialBase(t *testing.T) {
	if !IsMimoOfficialBase("https://token-plan-cn.xiaomimimo.com/v1") {
		t.Fatal("expected official mimo host")
	}
	if IsMimoOfficialBase("") {
		t.Fatal("empty base should not be official mimo")
	}
}

func TestEffectiveEffortDefaultFromSupported(t *testing.T) {
	e := &ProviderEntry{SupportedEfforts: []string{"low", "high"}, DefaultEffort: "high"}
	if got := EffectiveEffort(e); got != "high" {
		t.Fatalf("EffectiveEffort = %q", got)
	}
}

func TestSyncProvidersBaseURLGuards(t *testing.T) {
	var nilCfg *Config
	nilCfg.SyncProvidersBaseURL("DEEPSEEK_API_KEY", "https://api.deepseek.com")
	c := Default()
	c.SyncProvidersBaseURL("", "https://api.deepseek.com")
	c.SyncProvidersBaseURL("DEEPSEEK_API_KEY", "")
}

func TestDesktopGitSettingsSquashAndRebase(t *testing.T) {
	c := Default()
	for _, method := range []string{"squash", "rebase", "SQUASH"} {
		if err := c.SetDesktopGitSettings(method, true, true, "", ""); err != nil {
			t.Fatal(err)
		}
		want := strings.ToLower(method)
		if c.Desktop.Git.PRMergeMethod != want {
			t.Fatalf("merge = %q, want %q", c.Desktop.Git.PRMergeMethod, want)
		}
	}
}

func TestSetLanguageAutoClears(t *testing.T) {
	c := Default()
	c.Language = "zh"
	if err := c.SetLanguage("auto"); err != nil {
		t.Fatal(err)
	}
	if c.Language != "" {
		t.Fatalf("language = %q, want cleared", c.Language)
	}
	if err := c.SetDesktopLanguage("auto"); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Language != "" {
		t.Fatalf("desktop language = %q", c.Desktop.Language)
	}
}

func TestResolveSystemPromptFileError(t *testing.T) {
	c := Default()
	c.Agent.SystemPromptFile = filepath.Join(t.TempDir(), "missing.txt")
	if _, err := c.ResolveSystemPrompt(); err == nil {
		t.Fatal("expected read error")
	}
}

func TestProviderByAPIKeyEnvMissing(t *testing.T) {
	c := Default()
	if _, ok := c.ProviderByAPIKeyEnv("NO_SUCH_ENV"); ok {
		t.Fatal("expected miss")
	}
	if _, ok := (*Config)(nil).ProviderByAPIKeyEnv("X"); ok {
		t.Fatal("nil config should miss")
	}
}

func TestFetchModelsUsesModelsURL(t *testing.T) {
	srv := testModelsServer(t, "custom-model")
	defer srv.Close()
	isolateUserConfigHome(t)
	t.Setenv("MODELS_URL_KEY", "secret")
	e := &ProviderEntry{
		Name: "t", BaseURL: "http://unused", ModelsURL: srv.URL, APIKeyEnv: "MODELS_URL_KEY",
	}
	models, err := e.FetchModels(context.Background())
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 1 || models[0] != "custom-model" {
		t.Fatalf("models = %v", models)
	}
}

func TestWriteConfigFileEmptyPath(t *testing.T) {
	if err := writeConfigFile("  ", "body"); err == nil {
		t.Fatal("expected empty path error")
	}
}

func TestClearPluginAuthenticationInSourceProjectTOML(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)
	body := `[[plugins]]
name = "svc"
type = "http"
url = "https://example.com/mcp?access_token=secret"
`
	if err := os.WriteFile("arcdesk.toml", []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	updated, changed, path, err := ClearPluginAuthenticationInSource("svc")
	if err != nil {
		t.Fatalf("ClearPluginAuthenticationInSource: %v", err)
	}
	if !changed || path != "arcdesk.toml" {
		t.Fatalf("changed=%v path=%q", changed, path)
	}
	if strings.Contains(updated.URL, "access_token") {
		t.Fatalf("url still has token: %q", updated.URL)
	}
}

func TestAddPermissionRuleAskList(t *testing.T) {
	c := Default()
	if err := c.AddPermissionRule("ask", "bash(test*)"); err != nil {
		t.Fatal(err)
	}
	if len(c.Permissions.Ask) != 1 {
		t.Fatalf("ask = %v", c.Permissions.Ask)
	}
}

func TestRenderConfigVersionFallback(t *testing.T) {
	c := Default()
	c.ConfigVersion = 0
	body := RenderTOMLForScope(c, RenderScopeUser)
	if !strings.Contains(body, "config_version = 2") {
		t.Fatalf("missing config_version in render output")
	}
}

func TestDesktopGitSettingsNormalization(t *testing.T) {
	c := Default()
	c.Desktop.Git.PRMergeMethod = "rebase"
	if got := c.DesktopGitSettings().PRMergeMethod; got != "rebase" {
		t.Fatalf("merge = %q", got)
	}
}

func TestImportDesktopLocalPrefsSkipsAppearanceWhenFlagFalse(t *testing.T) {
	c := Default()
	if err := c.ImportDesktopLocalPrefs(DesktopAppearanceInput{
		BackgroundPreset: "paper",
	}, "all", false, false, false); err != nil {
		t.Fatal(err)
	}
	if c.Desktop.Appearance.BackgroundPreset != "" {
		t.Fatalf("appearance should stay empty, got %+v", c.Desktop.Appearance)
	}
}

func TestRefreshProviderModelsFromAPIEmptyList(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []any{}})
	}))
	defer srv.Close()
	isolateUserConfigHome(t)
	t.Setenv("EMPTY_KEY", "secret")
	c := Default()
	c.Providers = []ProviderEntry{{
		Name: "p", Kind: "openai", BaseURL: srv.URL, Model: "m", APIKeyEnv: "EMPTY_KEY",
	}}
	if err := RefreshProviderModelsFromAPI(c, "EMPTY_KEY"); err == nil {
		t.Fatal("expected empty model list error")
	}
}

func TestValidatePluginSSE(t *testing.T) {
	c := Default()
	if err := c.UpsertPlugin(PluginEntry{Name: "remote", Type: "sse", URL: "https://example.com/sse"}); err != nil {
		t.Fatalf("UpsertPlugin sse: %v", err)
	}
}

func TestRuntimeAndSelfdebugConfigHelpers(t *testing.T) {
	off := false
	disabledRT := RuntimeConfig{Enabled: &off}
	if !disabledRT.Disabled() || disabledRT.ShouldEnable() {
		t.Fatal("expected runtime disabled")
	}
	if def := (RuntimeConfig{}); !def.ShouldEnable() {
		t.Fatal("default runtime should enable")
	}
	rc128 := RuntimeConfig{MaxEntries: 128}
	if rc128.ResolvedMaxEntries() != 128 {
		t.Fatal("max entries")
	}
	def := RuntimeConfig{}
	if def.ResolvedMaxEntries() != 4096 {
		t.Fatal("default max entries")
	}
	disabledSD := SelfdebugConfig{Enabled: &off}
	if !disabledSD.Disabled() || disabledSD.ShouldEnable() {
		t.Fatal("expected selfdebug disabled")
	}
	if sd := (SelfdebugConfig{}); !sd.ShouldEnable() {
		t.Fatal("default selfdebug should enable")
	}
}
