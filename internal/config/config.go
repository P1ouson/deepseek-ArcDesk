// Package config loads ARCDESK's runtime configuration from TOML. Resolution order:
// flag > project ./arcdesk.toml > user ~/.config/arcdesk/config.toml > built-in defaults.
// Secrets come from the environment via api_key_env and are never stored in
// config files.
package config

import (
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/BurntSushi/toml"

	"arcdesk/internal/netclient"
	"arcdesk/internal/provider"
	"arcdesk/internal/provider/apikey"
)

var validSkillName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._-]{0,63}$`)

// IsValidSkillName reports whether name is a usable skill identifier.
func IsValidSkillName(name string) bool { return validSkillName.MatchString(name) }

// SkillNameKey normalizes a skill identifier for config comparisons.
func SkillNameKey(name string) string {
	name = strings.TrimSpace(name)
	if !IsValidSkillName(name) {
		return ""
	}
	if runtime.GOOS == "windows" {
		return strings.ToLower(name)
	}
	return name
}

// Config is ARCDESK's runtime configuration.
type Config struct {
	ConfigVersion int               `toml:"config_version"`
	DefaultModel  string            `toml:"default_model"`
	Language      string            `toml:"language"` // ui/model language tag (e.g. "zh"); empty = auto-detect from $LANG / $ARCDESK_LANG
	UI            UIConfig          `toml:"ui"`
	Desktop       DesktopConfig     `toml:"desktop"`
	Agent         AgentConfig       `toml:"agent"`
	Providers     []ProviderEntry   `toml:"providers"`
	Tools         ToolsConfig       `toml:"tools"`
	Permissions   PermissionsConfig `toml:"permissions"`
	Sandbox       SandboxConfig     `toml:"sandbox"`
	Network       NetworkConfig     `toml:"network"`
	Plugins       []PluginEntry     `toml:"plugins"`
	Skills        SkillsConfig      `toml:"skills"`
	Codegraph     CodegraphConfig   `toml:"codegraph"`
	Verification  VerificationConfig `toml:"verification"`
	Dependency    DependencyConfig  `toml:"dependency"`
	Callgraph     CallgraphConfig   `toml:"callgraph"`
	Runtime       RuntimeConfig     `toml:"runtime"`
	Selfdebug     SelfdebugConfig   `toml:"selfdebug"`
	Reporag       ReporagConfig     `toml:"reporag"`
	Constraint    ConstraintConfig  `toml:"constraint"`
	GitRag        GitRagConfig      `toml:"git_rag"`
	ArchRag       ArchRagConfig     `toml:"architecture_rag"`
	ArchitectureGuardian ArchitectureGuardianConfig `toml:"architecture_guardian"`
	PhasePlanner  PhasePlannerConfig `toml:"phase_planner"`
	FailureMemory FailureMemoryConfig `toml:"failure_memory"`
	UIRag         UIRagConfig         `toml:"ui_rag"`
	TaskDAG       TaskDAGConfig       `toml:"task_dag"`
	CostRouter    CostRouterConfig    `toml:"cost_router"`
	ContextCompression ContextCompressionConfig `toml:"context_compression"`
	EnvAware      EnvAwareConfig    `toml:"env_aware"`
	Knowledge     KnowledgeConfig   `toml:"knowledge"`
	Statusline    StatuslineConfig  `toml:"statusline"`
	LSP           LSPConfig         `toml:"lsp"`
}

// UIConfig controls CLI presentation-only settings. Desktop appearance is kept in
// DesktopConfig so desktop preferences cannot alter terminal output or prompts.
type UIConfig struct {
	Theme         string `toml:"theme"`          // auto|dark|light; empty resolves to auto
	ThemeStyle    string `toml:"theme_style"`    // graphite|ember|aurora|midnight|cobalt|sandstone|porcelain|linen|glacier
	CloseBehavior string `toml:"close_behavior"` // legacy desktop close behavior; prefer desktop.close_behavior
}

// DesktopConfig controls desktop-only UI preferences. It is intentionally
// separate from top-level language and [ui] so desktop choices do not affect CLI
// language, terminal colours, or provider-visible prompt/request data.
type DesktopGitConfig struct {
	PRMergeMethod         string `toml:"pr_merge_method"`           // merge|squash|rebase
	CheckGitHubCli        bool   `toml:"check_github_cli"`          // probe gh for PR merge; opt-in, zero value = off
	SyncRepoMergeToGitHub bool   `toml:"sync_repo_merge_to_github"` // PATCH repo allow_* merge flags when method changes
	CommitInstructions    string `toml:"commit_instructions"`       // folded into commit-message prompts
	PRInstructions        string `toml:"pr_instructions"`           // folded into PR title/body prompts
}

// DesktopAppearanceConfig stores desktop surface and typography preferences.
type DesktopAppearanceConfig struct {
	BackgroundPreset string `toml:"background_preset"` // paper|white|fog|linen|charcoal|graphite|slate|midnight
	ForegroundPreset string `toml:"foreground_preset"` // ink|charcoal|slate|snow|silver|white
	TextSize         string `toml:"text_size"`         // small|default|large|xlarge
	CodeFontSize     string `toml:"code_font_size"`    // small|default|large|xlarge
	DiffMarker       string `toml:"diff_marker"`       // background|signs
}

// DesktopCodeReviewConfig stores desktop code-review panel defaults.
type DesktopCodeReviewConfig struct {
	DefaultScope      string `toml:"default_scope"`       // all|session|git
	SecurityByDefault bool   `toml:"security_by_default"` // start in security review mode
}

type DesktopConfig struct {
	Language      string                  `toml:"language"`        // auto|en|zh; empty/auto = browser/OS auto-detect
	Theme         string                  `toml:"theme"`           // auto|dark|light; empty resolves to dark
	ThemeStyle    string                  `toml:"theme_style"`     // graphite|ember|aurora|midnight|cobalt|sandstone|porcelain|linen|glacier
	CloseBehavior string                  `toml:"close_behavior"`  // quit|background; desktop window close behavior
	TerminalShell string                  `toml:"terminal_shell"` // powershell|cmd|git-bash|wsl; empty = auto
	Git           DesktopGitConfig        `toml:"git"`
	Appearance    DesktopAppearanceConfig `toml:"appearance"`
	CodeReview    DesktopCodeReviewConfig `toml:"code_review"`
}

// UITheme normalizes ui.theme to a supported value.
func (c *Config) UITheme() string {
	switch strings.ToLower(strings.TrimSpace(c.UI.Theme)) {
	case "dark":
		return "dark"
	case "light":
		return "light"
	default:
		return "auto"
	}
}

// UIThemeStyle normalizes ui.theme_style. Empty means "pick the default style
// for the resolved light/dark shell".
func (c *Config) UIThemeStyle() string {
	return normalizeThemeStyle(c.UI.ThemeStyle)
}

func normalizeThemeStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "graphite", "ember", "aurora", "midnight", "cobalt", "sandstone", "porcelain", "linen", "glacier":
		return strings.ToLower(strings.TrimSpace(style))
	default:
		return ""
	}
}

func normalizeCloseBehavior(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "background":
		return "background"
	case "quit", "exit":
		return "quit"
	default:
		return "quit"
	}
}

// DesktopLanguage normalizes the desktop UI language. Empty means auto-detect
// from the browser/OS locale; it deliberately does not read top-level language,
// which is used by the CLI/model-facing runtime.
func (c *Config) DesktopLanguage() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.Language)) {
	case "en":
		return "en"
	case "zh":
		return "zh"
	default:
		return ""
	}
}

// DesktopTheme normalizes desktop.theme. New desktop users default to the light
// glacier product look; an explicit auto/light/dark is preserved.
func (c *Config) DesktopTheme() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.Theme)) {
	case "auto":
		return "auto"
	case "light":
		return "light"
	case "dark":
		return "dark"
	default:
		return "light"
	}
}

// DesktopThemeStyle normalizes desktop.theme_style. Empty means the frontend
// chooses the default style for the resolved desktop theme.
func (c *Config) DesktopThemeStyle() string {
	return normalizeThemeStyle(c.Desktop.ThemeStyle)
}

// DesktopTerminalShell normalizes desktop.terminal_shell. Empty means auto-detect
// (PowerShell on Windows, $SHELL elsewhere).
func (c *Config) DesktopTerminalShell() string {
	switch strings.ToLower(strings.TrimSpace(c.Desktop.TerminalShell)) {
	case "powershell", "pwsh":
		return "powershell"
	case "cmd", "command-prompt":
		return "cmd"
	case "git-bash", "gitbash", "bash":
		return "git-bash"
	case "wsl":
		return "wsl"
	default:
		return ""
	}
}

func normalizeDesktopBackgroundPreset(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "paper", "white", "fog", "linen", "charcoal", "graphite", "slate", "midnight":
		return strings.ToLower(strings.TrimSpace(preset))
	default:
		return ""
	}
}

func normalizeDesktopForegroundPreset(preset string) string {
	switch strings.ToLower(strings.TrimSpace(preset)) {
	case "ink", "charcoal", "slate", "snow", "silver", "white":
		return strings.ToLower(strings.TrimSpace(preset))
	default:
		return ""
	}
}

func normalizeDesktopTextSize(size string) string {
	switch strings.ToLower(strings.TrimSpace(size)) {
	case "small", "default", "large", "xlarge":
		return strings.ToLower(strings.TrimSpace(size))
	default:
		return ""
	}
}

func normalizeDesktopDiffMarker(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "background", "signs":
		return strings.ToLower(strings.TrimSpace(style))
	default:
		return ""
	}
}

// DesktopAppearanceSettings normalizes desktop.appearance for the settings panel.
func (c *Config) DesktopAppearanceSettings() DesktopAppearanceConfig {
	a := c.Desktop.Appearance
	return DesktopAppearanceConfig{
		BackgroundPreset: normalizeDesktopBackgroundPreset(a.BackgroundPreset),
		ForegroundPreset: normalizeDesktopForegroundPreset(a.ForegroundPreset),
		TextSize:         normalizeDesktopTextSize(a.TextSize),
		CodeFontSize:     normalizeDesktopTextSize(a.CodeFontSize),
		DiffMarker:       normalizeDesktopDiffMarker(a.DiffMarker),
	}
}

func normalizeDesktopCodeReviewScope(scope string) string {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "session", "git":
		return strings.ToLower(strings.TrimSpace(scope))
	default:
		return "all"
	}
}

// DesktopCodeReviewSettings normalizes desktop.code_review for the settings panel.
func (c *Config) DesktopCodeReviewSettings() DesktopCodeReviewConfig {
	cr := c.Desktop.CodeReview
	return DesktopCodeReviewConfig{
		DefaultScope:      normalizeDesktopCodeReviewScope(cr.DefaultScope),
		SecurityByDefault: cr.SecurityByDefault,
	}
}

// DesktopGitSettings normalizes desktop.git preferences for the settings panel.
func (c *Config) DesktopGitSettings() DesktopGitConfig {
	g := c.Desktop.Git
	switch strings.ToLower(strings.TrimSpace(g.PRMergeMethod)) {
	case "squash", "rebase":
		g.PRMergeMethod = strings.ToLower(strings.TrimSpace(g.PRMergeMethod))
	default:
		g.PRMergeMethod = "merge"
	}
	return g
}

// DesktopCloseBehavior normalizes the desktop close-window preference. It falls
// back to the legacy ui.close_behavior value for configs written before [desktop]
// existed.
func (c *Config) DesktopCloseBehavior() string {
	if strings.TrimSpace(c.Desktop.CloseBehavior) != "" {
		return normalizeCloseBehavior(c.Desktop.CloseBehavior)
	}
	return normalizeCloseBehavior(c.UI.CloseBehavior)
}

// UICloseBehavior is the legacy name for DesktopCloseBehavior.
func (c *Config) UICloseBehavior() string {
	return c.DesktopCloseBehavior()
}

// LSPConfig governs the optional Language Server Protocol tools (lsp_definition,
// lsp_references, lsp_hover, lsp_diagnostics). Enabled defaults to true; the
// servers themselves are never bundled 鈥?each resolves on PATH and the tool
// returns an install hint when it is missing, so the capability is dormant until
// the user installs a server. Servers overrides or extends the built-in language
// 鈫?server map, keyed by language id (e.g. "go", "rust", "python").
type LSPConfig struct {
	Enabled bool                 `toml:"enabled"`
	Servers map[string]LSPServer `toml:"servers"`
}

// LSPServer overrides a built-in language's server or, when keyed by a new
// language, adds one. An empty field falls back to the built-in default for that
// language; Extensions is required when adding a language the built-ins don't
// cover (e.g. ".ex" for Elixir) so files route to it.
type LSPServer struct {
	Command     string            `toml:"command"`
	Args        []string          `toml:"args"`
	Env         map[string]string `toml:"env"`
	LanguageID  string            `toml:"language_id"`
	Extensions  []string          `toml:"extensions"`
	InstallHint string            `toml:"install_hint"`
}

// StatuslineConfig configures a custom status line. Command, when set, is run at
// startup and after each turn; its first line of stdout replaces the built-in
// status data row. A JSON payload (model, context tokens, cwd) is fed on stdin.
type StatuslineConfig struct {
	Command string `toml:"command"`
}

// CodegraphConfig governs the built-in CodeGraph MCP server 鈥?symbol/call-graph
// code intelligence (tree-sitter + SQLite) that gives the agent codegraph_*
// search / context / explore / trace / node tools. Enabled defaults to true so
// upgrades keep it for existing configs; first-run scaffolds write enabled =
// false so only brand-new users start without it. AutoInstall (default true)
// lets ARCDESK fetch the CodeGraph runtime into its cache when CodeGraph is
// enabled but missing; set false to require an explicit `ARCDESK codegraph
// install` (e.g. for air-gapped or headless runs). Path overrides binary
// resolution; empty resolves the cache, then a `codegraph` on PATH, then a
// bundle beside the executable. Tier matches ordinary MCP servers (lazy,
// background, eager); when unset it preserves the historical warm鈫抏ager /
// cold鈫抌ackground startup.
type CodegraphConfig struct {
	Enabled     bool   `toml:"enabled"`
	AutoInstall bool   `toml:"auto_install"`
	Path        string `toml:"path"`
	Tier        string `toml:"tier"`
}

func (c CodegraphConfig) ShouldAutoStart() bool {
	return c.Enabled
}

func (c CodegraphConfig) ResolvedTier() string {
	return resolvedMCPTier(c.Tier)
}

// VerificationConfig governs post-write project checks (P0 verification loop).
// When enabled, checks are registered for verification_plan/status and optional
// self-debug hints. They do not block the agent from finishing a turn unless
// EnforceFinalAnswer is true — the agent only runs what the user asked.
// AfterWrite lists commands; when empty, checks may come from AGENTS.md host
// checks or auto-discovery (go.mod / package.json). MaxRetries bounds how many
// times the harness re-prompts when enforcement is on; OnFailure selects retry
// (default), rollback, or ask.
type VerificationConfig struct {
	Enabled            *bool    `toml:"enabled"`
	EnforceFinalAnswer *bool    `toml:"enforce_final_answer"`
	AfterWrite         []string `toml:"after_write"`
	AutoDiscover       *bool    `toml:"auto_discover"`
	IncludeUnit        *bool    `toml:"include_unit"`
	IncludeE2E         *bool    `toml:"include_e2e"`
	MaxRetries         int      `toml:"max_retries"`
	OnFailure          string   `toml:"on_failure"`
}

func (c VerificationConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c VerificationConfig) AutoDiscoverEnabled() bool {
	if c.AutoDiscover != nil {
		return *c.AutoDiscover
	}
	return true
}

// EnforcesFinalAnswer reports whether the host may re-open a turn and require
// configured checks before the model's final answer. Default is false so the
// agent only executes what the user explicitly requested.
func (c VerificationConfig) EnforcesFinalAnswer() bool {
	return c.EnforceFinalAnswer != nil && *c.EnforceFinalAnswer
}

func (c VerificationConfig) ResolvedPolicy() (maxRetries int, onFailure string) {
	maxRetries = c.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	switch strings.ToLower(strings.TrimSpace(c.OnFailure)) {
	case "rollback":
		return maxRetries, "rollback"
	case "ask":
		return maxRetries, "ask"
	default:
		return maxRetries, "retry"
	}
}

// DependencyConfig governs the native module/package dependency index and
// dependency_* agent tools (complements codegraph MCP symbol tools).
type DependencyConfig struct {
	Enabled      *bool `toml:"enabled"`
	AutoDiscover *bool `toml:"auto_discover"`
}

func (c DependencyConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c DependencyConfig) AutoDiscoverEnabled() bool {
	if c.AutoDiscover != nil {
		return *c.AutoDiscover
	}
	return true
}

// ShouldIndex reports whether the dependency index should be wired for a workspace.
// When Enabled is explicitly true, indexing runs even without go.mod/package.json.
// When Enabled is unset, AutoDiscover controls whether project type is probed.
func (c DependencyConfig) ShouldIndex(discoverable bool) bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	if !c.AutoDiscoverEnabled() {
		return false
	}
	return discoverable
}

// CallgraphConfig governs the Wails cross-realm call graph index and callgraph_* tools.
type CallgraphConfig struct {
	Enabled      *bool `toml:"enabled"`
	AutoDiscover *bool `toml:"auto_discover"`
}

func (c CallgraphConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c CallgraphConfig) AutoDiscoverEnabled() bool {
	if c.AutoDiscover != nil {
		return *c.AutoDiscover
	}
	return true
}

// ShouldIndex reports whether the callgraph index should be wired for a workspace.
func (c CallgraphConfig) ShouldIndex(discoverable bool) bool {
	if c.Enabled != nil {
		return *c.Enabled
	}
	if !c.AutoDiscoverEnabled() {
		return false
	}
	return discoverable
}

// RuntimeConfig governs live session runtime observation (console, shell output,
// network, state) exposed through runtime_* agent tools.
type RuntimeConfig struct {
	Enabled    *bool `toml:"enabled"`
	MaxEntries int   `toml:"max_entries"`
}

func (c RuntimeConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c RuntimeConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c RuntimeConfig) ResolvedMaxEntries() int {
	if c.MaxEntries <= 0 {
		return 4096
	}
	return c.MaxEntries
}

// SelfdebugConfig governs the P0 write→verify→analyze→fix orchestrator.
// When enabled (default), failed verify commands get immediate hints and final-
// answer readiness retries receive a unified self-debug block.
type SelfdebugConfig struct {
	Enabled *bool `toml:"enabled"`
}

func (c SelfdebugConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c SelfdebugConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// ReporagConfig governs the unified repo-aware RAG orchestrator (P0-#1).
type ReporagConfig struct {
	Enabled *bool `toml:"enabled"`
}

func (c ReporagConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c ReporagConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// ConstraintConfig governs the P0 constraint system (duplicate/reuse/fake-UI/arch checks).
type ConstraintConfig struct {
	Enabled *bool `toml:"enabled"`
}

func (c ConstraintConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c ConstraintConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// GitRagConfig governs git-aware retrieval tools (P1-#8): blame, log, gh context.
type GitRagConfig struct {
	Enabled      *bool `toml:"enabled"`
	AutoDiscover *bool `toml:"auto_discover"`
}

func (c GitRagConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c GitRagConfig) AutoDiscoverEnabled() bool {
	if c.AutoDiscover != nil {
		return *c.AutoDiscover
	}
	return true
}

func (c GitRagConfig) ShouldEnable(discoverable bool) bool {
	if c.Disabled() {
		return false
	}
	if c.Enabled != nil {
		return *c.Enabled
	}
	if !c.AutoDiscoverEnabled() {
		return false
	}
	return discoverable
}

// ArchRagConfig governs architecture document indexing (P1-#9).
type ArchRagConfig struct {
	Enabled *bool    `toml:"enabled"`
	Paths   []string `toml:"paths"`
}

func (c ArchRagConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c ArchRagConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// ArchitectureGuardianConfig governs P2-#15 SPEC-aware architecture enforcement.
type ArchitectureGuardianConfig struct {
	Enabled *bool `toml:"enabled"`
}

func (c ArchitectureGuardianConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c ArchitectureGuardianConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// PhasePlannerConfig governs phased execution (P1-#10): split plans into stages
// and gate final answers until each phase is explicitly completed.
type PhasePlannerConfig struct {
	Enabled      *bool `toml:"enabled"`
	EnforceGates *bool `toml:"enforce_gates"`
}

func (c PhasePlannerConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c PhasePlannerConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c PhasePlannerConfig) GatesEnforced() bool {
	if c.EnforceGates != nil {
		return *c.EnforceGates
	}
	return true
}

// UIRagConfig governs React/TSX component discovery tools (P2-#14).
type UIRagConfig struct {
	Enabled      *bool `toml:"enabled"`
	AutoDiscover *bool `toml:"auto_discover"`
}

func (c UIRagConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c UIRagConfig) AutoDiscoverEnabled() bool {
	if c.AutoDiscover != nil {
		return *c.AutoDiscover
	}
	return true
}

func (c UIRagConfig) ShouldEnable(discoverable bool) bool {
	if c.Disabled() {
		return false
	}
	if c.Enabled != nil {
		return *c.Enabled
	}
	if !c.AutoDiscoverEnabled() {
		return false
	}
	return discoverable
}

// TaskDAGConfig governs dependency-ordered sub-task orchestration (P2-#16).
type TaskDAGConfig struct {
	Enabled *bool `toml:"enabled"`
}

func (c TaskDAGConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c TaskDAGConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// CostRouterConfig governs tier-based model routing (P2-#17).
type CostRouterConfig struct {
	Enabled       *bool  `toml:"enabled"`
	ClassifyModel string `toml:"classify_model"`
	ExecuteModel  string `toml:"execute_model"`
	CompactModel  string `toml:"compact_model"`
	ExploreModel  string `toml:"explore_model"`
}

func (c CostRouterConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c CostRouterConfig) ShouldEnable() bool {
	return !c.Disabled()
}

// ContextCompressionConfig governs tool-output compression (P2-#18).
type ContextCompressionConfig struct {
	Enabled            *bool `toml:"enabled"`
	ToolOutputMaxBytes int   `toml:"tool_output_max_bytes"`
}

func (c ContextCompressionConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c ContextCompressionConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c ContextCompressionConfig) ResolvedToolOutputMaxBytes() int {
	if c.ToolOutputMaxBytes > 0 {
		return c.ToolOutputMaxBytes
	}
	// Enabled with no explicit cap: tighter than the agent's built-in 32 KiB default.
	return 16 * 1024
}

// FailureMemoryConfig governs persistent failure→fix lessons (P1-#11).
type FailureMemoryConfig struct {
	Enabled    *bool `toml:"enabled"`
	MaxEntries int   `toml:"max_entries"`
}

func (c FailureMemoryConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c FailureMemoryConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c FailureMemoryConfig) ResolvedMaxEntries() int {
	if c.MaxEntries <= 0 {
		return 500
	}
	return c.MaxEntries
}

// KnowledgeConfig governs experience retrieval, retry injection, and capture (v1).
type KnowledgeConfig struct {
	Enabled *bool `toml:"enabled"`

	InjectOnMessage      string `toml:"inject_on_message"`       // off | debug_only
	InjectOnVerifyRetry  *bool  `toml:"inject_on_verify_retry"`
	MaxRetryHintChars    int    `toml:"max_retry_hint_chars"`
	MaxRetryStderrExcerpt int   `toml:"max_retry_stderr_excerpt"`

	AutoCaptureOnVerify *bool  `toml:"auto_capture_on_verify"`
	// RequireCaptureConfirm when true keeps the Record/Ignore card instead of
	// writing qualified lessons automatically (default false).
	RequireCaptureConfirm *bool `toml:"capture_requires_confirm"`
	IndexInSystemPrompt *bool  `toml:"index_in_system_prompt"`
	MaxIndexLines       int    `toml:"max_index_lines"`
	MergeOnWrite        *bool  `toml:"merge_on_write"`
}

func (c KnowledgeConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c KnowledgeConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c KnowledgeConfig) VerifyRetryInjectEnabled() bool {
	if c.InjectOnVerifyRetry != nil {
		return *c.InjectOnVerifyRetry
	}
	return true
}

func (c KnowledgeConfig) VerifyAutoCaptureEnabled() bool {
	if c.AutoCaptureOnVerify != nil {
		return *c.AutoCaptureOnVerify
	}
	return true
}

func (c KnowledgeConfig) CaptureRequiresConfirm() bool {
	if c.RequireCaptureConfirm != nil {
		return *c.RequireCaptureConfirm
	}
	return false
}

func (c KnowledgeConfig) SystemPromptIndexEnabled() bool {
	if c.IndexInSystemPrompt != nil {
		return *c.IndexInSystemPrompt
	}
	return true
}

func (c KnowledgeConfig) ResolvedMaxRetryHintChars() int {
	if c.MaxRetryHintChars > 0 {
		return c.MaxRetryHintChars
	}
	return 200
}

func (c KnowledgeConfig) ResolvedMaxRetryStderrExcerpt() int {
	if c.MaxRetryStderrExcerpt > 0 {
		return c.MaxRetryStderrExcerpt
	}
	return 2048
}

func (c KnowledgeConfig) ResolvedMaxIndexLines() int {
	if c.MaxIndexLines > 0 {
		return c.MaxIndexLines
	}
	return 30
}

func (c KnowledgeConfig) InjectOnMessageDebugOnly() bool {
	switch strings.ToLower(strings.TrimSpace(c.InjectOnMessage)) {
	case "debug_only", "debug", "auto":
		return true
	default:
		return false
	}
}

// EnvAwareConfig governs host OS/toolchain probing (P1-#12).
type EnvAwareConfig struct {
	Enabled       *bool `toml:"enabled"`
	FoldIntoPrompt *bool `toml:"fold_into_prompt"`
}

func (c EnvAwareConfig) Disabled() bool {
	return c.Enabled != nil && !*c.Enabled
}

func (c EnvAwareConfig) ShouldEnable() bool {
	return !c.Disabled()
}

func (c EnvAwareConfig) PromptFoldEnabled() bool {
	if c.FoldIntoPrompt != nil {
		return *c.FoldIntoPrompt
	}
	return true
}

// NetworkConfig controls ordinary outbound HTTP traffic such as model providers,
// wallet-balance lookups, updater checks, and CodeGraph downloads. It intentionally
// does not apply to web_fetch, which keeps its own SSRF-guarded dialer.
type NetworkConfig struct {
	// ProxyMode is "auto" (default; environment proxy for now), "env", "custom",
	// or "off". auto leaves room for OS proxy detection later without changing the
	// config shape.
	ProxyMode string `toml:"proxy_mode"`
	// ProxyURL is an advanced custom override such as "socks5://127.0.0.1:7890".
	// When set and proxy_mode = "custom", it wins over the structured proxy table.
	ProxyURL string `toml:"proxy_url"`
	// NoProxy is honored for custom proxies. Env/auto modes use NO_PROXY from the
	// process environment instead.
	NoProxy string             `toml:"no_proxy"`
	Proxy   NetworkProxyConfig `toml:"proxy"`
}

// NetworkProxyConfig is the structured custom-proxy editor shape. Password is
// optional and supports ${VAR} expansion, so users can avoid storing it literally.
type NetworkProxyConfig struct {
	Type     string `toml:"type"` // http|https|socks5|socks5h
	Server   string `toml:"server"`
	Port     int    `toml:"port"`
	Username string `toml:"username"`
	Password string `toml:"password"`
}

// NetworkProxySpec returns the expanded proxy settings used by netclient.
func (c *Config) NetworkProxySpec() netclient.ProxySpec {
	return netclient.ProxySpec{
		Mode:        c.Network.ProxyMode,
		URL:         ExpandVars(c.Network.ProxyURL),
		NoProxy:     ExpandVars(c.Network.NoProxy),
		Type:        c.Network.Proxy.Type,
		Server:      ExpandVars(c.Network.Proxy.Server),
		Port:        c.Network.Proxy.Port,
		Username:    ExpandVars(c.Network.Proxy.Username),
		Password:    ExpandVars(c.Network.Proxy.Password),
		DirectHosts: c.directProxyHosts(),
	}
}

// directProxyHosts collects the base_url hosts of providers marked no_proxy, so
// netclient bypasses the proxy for them without knowing any provider by name.
func (c *Config) directProxyHosts() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range c.Providers {
		if !p.NoProxy {
			continue
		}
		u, err := url.Parse(strings.TrimSpace(p.BaseURL))
		if err != nil {
			continue
		}
		if h := u.Hostname(); h != "" && !seen[h] {
			seen[h] = true
			out = append(out, h)
		}
	}
	return out
}

// NetworkProxyMode normalizes network.proxy_mode to a known value.
func (c *Config) NetworkProxyMode() string {
	return netclient.NormalizeMode(c.Network.ProxyMode)
}

// SkillsConfig configures skill discovery. Paths adds extra "custom"-scope skill
// roots 鈥?each a directory of SKILL.md / <name>.md playbooks 鈥?scanned between
// the project roots (.arcdesk/.agents/.claude under the workspace) and the
// global roots (the same three under the home dir). ~ and relative paths and
// ${VAR} expansion are supported. DisabledSkills hides named skills from the
// agent prompt, slash invocation, and skill tools while keeping them manageable.
type SkillsConfig struct {
	Paths          []string `toml:"paths"`
	DisabledSkills []string `toml:"disabled_skills"`
}

// SkillCustomPaths returns the configured custom skill roots with ${VAR}
// expanded; empty entries are dropped.
func (c *Config) SkillCustomPaths() []string {
	var out []string
	for _, p := range c.Skills.Paths {
		if p = ExpandVars(p); strings.TrimSpace(p) != "" {
			out = append(out, p)
		}
	}
	return out
}

// DisabledSkillNames returns valid disabled skill identifiers, preserving the
// first spelling and dropping duplicates/empty entries.
func (c *Config) DisabledSkillNames() []string {
	seen := map[string]bool{}
	var out []string
	for _, name := range c.Skills.DisabledSkills {
		name = strings.TrimSpace(name)
		if !IsValidSkillName(name) {
			continue
		}
		key := SkillNameKey(name)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, name)
	}
	return out
}

// IsSkillDisabled reports whether name is configured as disabled.
func (c *Config) IsSkillDisabled(name string) bool {
	key := SkillNameKey(name)
	if key == "" {
		return false
	}
	for _, disabled := range c.DisabledSkillNames() {
		if SkillNameKey(disabled) == key {
			return true
		}
	}
	return false
}

// SandboxConfig bounds the blast radius of tool calls (Phase 0: file-writer
// confinement). WorkspaceRoot is the directory the built-in file writers
// (write_file / edit_file / multi_edit) may modify; empty means the current
// working directory, so writes stay inside the project by default. AllowWrite
// lists extra directories writers may also touch (e.g. a sibling repo or a temp
// dir). Both support ${VAR} / ${VAR:-default} expansion. Reads are unrestricted;
// confining `bash` is Phase 1 (OS-level sandbox).
type SandboxConfig struct {
	WorkspaceRoot string   `toml:"workspace_root"`
	AllowWrite    []string `toml:"allow_write"`
	// Bash is the OS-sandbox mode for the bash tool: "enforce" (default) jails
	// each command, "off" runs it unconfined. Phase 1; macOS only for now, with
	// a graceful fallback elsewhere (see internal/sandbox).
	Bash string `toml:"bash"`
	// Network allows network egress from inside the bash sandbox. Defaults true
	// so module/package downloads keep working; the boundary is then writes.
	Network bool `toml:"network"`
}

// WriteRoots returns the directories file-writer tools may modify: the
// workspace root (defaulting to the current working directory when unset) plus
// any AllowWrite extras, with ${VAR} expanded. The roots are returned as given
// (relative or absolute); the confiner resolves them to absolute, symlink-free
// paths. The result is always non-empty, so confinement is on by default.
func (c *Config) WriteRoots() []string {
	return c.WriteRootsForRoot(".")
}

// WriteRootsForRoot is like WriteRoots but falls back to fallbackRoot when the
// config doesn't explicitly set a workspace_root. Desktop tabs pass their
// project root here so tool confinement is correct without changing cwd.
func (c *Config) WriteRootsForRoot(fallbackRoot string) []string {
	root := ExpandVars(c.Sandbox.WorkspaceRoot)
	if root == "" {
		root = fallbackRoot
		if root == "" || root == "." {
			if wd, err := os.Getwd(); err == nil {
				root = wd
			} else {
				root = "."
			}
		}
	}
	roots := []string{root}
	for _, d := range c.Sandbox.AllowWrite {
		if d = ExpandVars(d); d != "" {
			roots = append(roots, d)
		}
	}
	return roots
}

// BashMode normalises the bash-sandbox mode: only an explicit "off" disables
// it; empty or any other value resolves to "enforce", so the sandbox is on by
// default and fails safe.
func (c *Config) BashMode() string {
	if c.Sandbox.Bash == "off" {
		return "off"
	}
	return "enforce"
}

// AgentConfig configures the harness loop. PlannerModel is optional: when set
// to another provider's name it enables two-model collaboration, where the
// planner handles low-frequency planning in its own session (kept separate so
// each model's prompt prefix stays cache-stable). SubagentModel is the optional
// default for runAs=subagent skills; SubagentModels overrides it per skill name.
type AgentConfig struct {
	SystemPrompt     string            `toml:"system_prompt"`
	SystemPromptFile string            `toml:"system_prompt_file"`
	MaxSteps         int               `toml:"max_steps"` // tool-call rounds per turn; 0 = unlimited
	Temperature      float64           `toml:"temperature"`
	PlannerModel     string            `toml:"planner_model"`
	SubagentModel    string            `toml:"subagent_model"`
	SubagentModels   map[string]string `toml:"subagent_models"`
	// OutputStyle selects a persona/tone block folded into the system prompt at
	// startup (a built-in like "explanatory"/"learning"/"concise", or a custom
	// .arcdesk/output-styles/<name>.md). Empty = the unmodified prompt.
	OutputStyle string `toml:"output_style"`
	// AutoPlan controls whether interactive turns that look multi-step start in
	// plan mode automatically: "off" keeps plan mode manual, "on" enables the
	// approval gate. Legacy "ask" is treated as "on".
	AutoPlan string `toml:"auto_plan"`
	// AutoPlanClassifier optionally names a provider/model used to classify
	// borderline auto-plan decisions. Empty keeps the zero-cost heuristic path.
	AutoPlanClassifier string `toml:"auto_plan_classifier"`
	// Compaction window fractions: soft = notice only, compact = trigger, force = hard ceiling.
	SoftCompactRatio  float64 `toml:"soft_compact_ratio"`
	CompactRatio      float64 `toml:"compact_ratio"`
	CompactForceRatio float64 `toml:"compact_force_ratio"`
}

// ProviderEntry declares a model provider instance. ContextWindow is the model's
// token budget; the harness compacts older history as a turn's prompt approaches
// it (see agent compaction). 0 disables compaction for the instance.
type ProviderEntry struct {
	Name          string            `toml:"name"`
	Kind          string            `toml:"kind"`
	BaseURL       string            `toml:"base_url"`
	Model         string            `toml:"model"`      // a single model (back-compat)
	Models        []string          `toml:"models"`     // a vendor's model list (one base_url/key, many models)
	ModelsURL     string            `toml:"models_url"` // auto-fetch models from this URL on startup
	Default       string            `toml:"default"`    // default model when Models is set (else Models[0])
	APIKeyEnv     string            `toml:"api_key_env"`
	BalanceURL    string            `toml:"balance_url"` // optional; a provider-specific wallet-balance endpoint (DeepSeek: https://api.deepseek.com/user/balance). Empty = no balance readout.
	ContextWindow int               `toml:"context_window"`
	Price         *provider.Pricing `toml:"price"`
	// Thinking / Effort are provider-kind-specific knobs forwarded to the provider
	// via Config.Extra. The anthropic provider reads Thinking="adaptive" to enable
	// extended thinking and Effort ("low".."max") to tune depth. The
	// openai-compatible provider forwards Effort as reasoning_effort for
	// thinking-capable models; DeepSeek accepts high|max.
	// Empty = provider default.
	Thinking string `toml:"thinking"`
	Effort   string `toml:"effort"`
	// SupportedEfforts lists the /effort levels this provider/model exposes.
	// When non-empty, it overrides the built-in defaults derived from
	// Kind/BaseURL and makes /effort configurable. "auto" is the implicit
	// prefix 鈥?always accepted. DefaultEffort resolves it; omit DefaultEffort
	// (or set one outside this list) to fall back to SupportedEfforts[0].
	SupportedEfforts []string `toml:"supported_efforts"`
	// DefaultEffort is the /effort level used when the user picks "auto" or
	// has not set Effort. Ignored when SupportedEfforts is empty.
	DefaultEffort string `toml:"default_effort"`
	// NoProxy reaches this provider's base_url directly, never through the proxy.
	// For China-only endpoints a foreign-exit proxy resets the TLS handshake (#2803).
	NoProxy bool `toml:"no_proxy"`
}

// ModelList returns the models this provider exposes: the explicit `models` list,
// or the single `model` as a one-element list (back-compat). Empty if neither set.
func (e *ProviderEntry) ModelList() []string {
	if len(e.Models) > 0 {
		return e.Models
	}
	if e.Model != "" {
		return []string{e.Model}
	}
	return nil
}

// DefaultModel returns the provider's default model: the explicit `default`, else
// the first of ModelList.
func (e *ProviderEntry) DefaultModel() string {
	if e.Default != "" {
		return e.Default
	}
	if l := e.ModelList(); len(l) > 0 {
		return l[0]
	}
	return ""
}

// HasModel reports whether m is one of the provider's models.
func (e *ProviderEntry) HasModel(m string) bool {
	for _, x := range e.ModelList() {
		if x == m {
			return true
		}
	}
	return false
}

// ToolsConfig selects which built-in tools are enabled. Empty means all of them.
type ToolsConfig struct {
	Enabled []string     `toml:"enabled"`
	Search  SearchConfig `toml:"search"`
}

// SearchConfig tunes the grep tool's engine. Engine is "auto" (default 鈥?use
// ripgrep when it's on PATH, else the native Go scanner), "native" (always Go),
// or "rg" (require ripgrep; warn at startup and fall back to native if absent).
// RgPath optionally points at a specific ripgrep binary instead of a PATH lookup.
type SearchConfig struct {
	Engine string `toml:"engine"`
	RgPath string `toml:"rg_path"`
}

// PermissionsConfig declares the per-call permission policy (see
// internal/permission). Mode is the fallback decision for writer tools when no
// rule matches ("ask" | "allow" | "deny"; default "ask"); read-only tools always
// fall back to allow. Allow/Ask/Deny are rule lists of the form "ToolName" or
// "ToolName(glob)". Precedence: deny > ask > allow > fallback.
type PermissionsConfig struct {
	Mode  string   `toml:"mode"`
	Allow []string `toml:"allow"`
	Ask   []string `toml:"ask"`
	Deny  []string `toml:"deny"`
}

// PluginEntry declares an external MCP server. Type selects the transport:
// "stdio" (default) launches Command/Args/Env as a subprocess; "http"
// (a.k.a. streamable-http) and "sse" connect to a remote URL with optional
// static Headers. String fields support ${VAR} / ${VAR:-default} expansion so
// secrets (bearer tokens, keys) come from the environment, not the file. The
// fields mirror Claude Code's mcpServers spec, so entries can come from either
// ARCDESK.toml's [[plugins]] or a project-root .mcp.json (see loadMCPJSON).
type PluginEntry struct {
	Name    string            `toml:"name"`
	Type    string            `toml:"type"` // "stdio" (default) | "http" | "sse"
	Command string            `toml:"command"`
	Args    []string          `toml:"args"`
	Env     map[string]string `toml:"env"`
	URL     string            `toml:"url"`
	Headers map[string]string `toml:"headers"`
	// AutoStart controls whether the server connects during session startup.
	// Nil preserves historical behavior: configured servers start automatically.
	AutoStart *bool `toml:"auto_start"`
	// Tier selects how aggressively the server is connected at boot:
	//   "eager"      鈥?blocks startup until the handshake completes; required for
	//                  servers whose tools the system prompt depends on.
	//   "lazy"       鈥?registers placeholder tools immediately (from on-disk
	//                  schema cache when available) and only spawns the real
	//                  subprocess on first model use. Default for user plugins.
	//   "background" 鈥?placeholder + spawn fired at boot but not waited on;
	//                  swap happens once the spawn finishes.
	// Empty defaults to "lazy" so adding a plugin never slows the next launch.
	Tier string `toml:"tier"`
	// Source records discovery origin: "" (explicit config), "mcpjson", "legacy".
	Source string `toml:"-" json:"-"`
}

func (e PluginEntry) ShouldAutoStart() bool {
	return e.AutoStart == nil || *e.AutoStart
}

// ResolvedTier returns the normalized tier ("eager"|"lazy"|"background") with
// the project default applied. Unknown values fall back to "lazy" so a typo
// never forces a slow boot.
func (e PluginEntry) ResolvedTier() string {
	return resolvedMCPTier(e.Tier)
}

func resolvedMCPTier(tier string) string {
	switch strings.ToLower(strings.TrimSpace(tier)) {
	case "eager":
		return "eager"
	case "background":
		return "background"
	default:
		return "lazy"
	}
}

func (c *Config) AutoStartPlugins() []PluginEntry {
	out := make([]PluginEntry, 0, len(c.Plugins))
	for _, p := range c.Plugins {
		if p.ShouldAutoStart() {
			out = append(out, p)
		}
	}
	return out
}

// DefaultSystemPrompt is used when config provides none.
const DefaultSystemPrompt = `You are ArcDesk, a coding agent focused on executing code tasks.
Use the provided tools to read and write files and run shell commands.
Principles: understand the request before acting; verify with tools instead of
guessing; keep changes minimal and correct; briefly summarize what you did.
When the request leaves a real choice to the user — which approach or library,
the scope, or a consequential or ambiguous decision — call the ask tool to offer
2-4 concrete options rather than guessing or burying the question in prose. Skip
it when there's an obvious default; don't ask just to confirm.
For multi-step work, break the task into clear phases, work through them in order,
and state progress in your replies. Prefer dedicated file tools (read_file, grep,
glob, ls) over shell when searching or reading code.
In plan mode the harness blocks writer tools: do read-only research, then write a
concise plan as your reply and stop. The user is asked to approve before anything
is changed; once approved, execute the plan step by step and verify before claiming done.`

// LanguagePolicy is the auto fallback appended to the system prompt when no
// concrete UI language is resolved. It is static English text, so it stays part
// of the cache-stable prefix and avoids per-turn language injection.
const LanguagePolicy = `Reply in the same language the user is using in their most recent message: ` +
	`if they write in Chinese answer in Chinese, in English answer in English, and switch ` +
	`whenever they switch. Let this also guide the language you think in. Always keep code, ` +
	`identifiers, file paths, shell commands, and technical terms in their original form — never translate them.`

// Default returns the built-in default configuration (DeepSeek + MiMo presets).
func Default() *Config {
	return &Config{
		ConfigVersion: 2,
		DefaultModel:  "deepseek-flash",
		UI:            UIConfig{Theme: "auto"},
		Agent: AgentConfig{
			SystemPrompt: DefaultSystemPrompt,
			// 0 = no step cap: the agent loops until the model gives a final answer,
			// the user cancels, or the provider errors. Context stays bounded by
			// compaction, not by a round count. Set a positive agent.max_steps only
			// if you want a hard guard against runaway.
			MaxSteps:          0,
			AutoPlan:          "off",
			SoftCompactRatio:  0.5,
			CompactRatio:      0.8,
			CompactForceRatio: 0.9,
		},
		// Mode "ask" with no rules keeps `ARCDESK run` autonomous (no TTY 鈫?ask
		// resolves to allow) while `ARCDESK chat` prompts before writers. Users add
		// deny/allow rules to harden or quiet specific tools.
		Permissions: PermissionsConfig{
			Mode: "ask",
			Deny: []string{"bash(rm -rf*)"},
		},
		// Sandbox on by default: bash is jailed (macOS), network allowed so
		// builds/downloads work. Set bash = "off" to disable. Network=true here
		// so an absent [sandbox] in a user's file keeps egress (zero value would
		// wrongly deny it).
		Sandbox: SandboxConfig{Bash: "enforce", Network: true},
		// CodeGraph code-intelligence defaults on so existing configs (which never
		// wrote a [codegraph] section) keep it after an upgrade. First-run scaffolds
		// write enabled = false instead, so only brand-new users start without it.
		// AutoInstall fetches the runtime into the cache when enabled and missing.
		Codegraph: CodegraphConfig{Enabled: true, AutoInstall: true},
		// Dependency index defaults on for upgrades (nil enabled = true). First-run
		// scaffolds write enabled = false instead.
		Dependency: DependencyConfig{},
		Callgraph:  CallgraphConfig{},
		Runtime:    RuntimeConfig{},
		// LSP tools on by default, but dormant until a language server is on PATH;
		// a missing server yields an install hint rather than an error.
		LSP:     LSPConfig{Enabled: true},
		Network: NetworkConfig{ProxyMode: netclient.ModeAuto},
		Providers: []ProviderEntry{
			{Name: "deepseek-flash", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-flash", APIKeyEnv: "DEEPSEEK_API_KEY", BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: &provider.Pricing{CacheHit: 0.02, Input: 1, Output: 2, Currency: "楼"}},
			{Name: "deepseek-pro", Kind: "openai", BaseURL: "https://api.deepseek.com", Model: "deepseek-v4-pro", APIKeyEnv: "DEEPSEEK_API_KEY", BalanceURL: "https://api.deepseek.com/user/balance", ContextWindow: 1_000_000, Price: &provider.Pricing{CacheHit: 0.025, Input: 3, Output: 6, Currency: "楼"}},
			{Name: "mimo-pro", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5-pro", APIKeyEnv: "MIMO_API_KEY", ContextWindow: 1_000_000, Price: &provider.Pricing{CacheHit: 0.025, Input: 3, Output: 6, Currency: "楼"}, NoProxy: true},
			{Name: "mimo-flash", Kind: "openai", BaseURL: "https://token-plan-cn.xiaomimimo.com/v1", Model: "mimo-v2.5", APIKeyEnv: "MIMO_API_KEY", ContextWindow: 1_000_000, Price: &provider.Pricing{CacheHit: 0.02, Input: 1, Output: 2, Currency: "楼"}, NoProxy: true},
		},
	}
}

// Load builds the configuration: defaults, then user config, then project
// config, then MCP servers from Claude Code's .mcp.json, then (lowest priority)
// the v0.x ~/.arcdesk/config.json's mcpServers. A .env in the working directory
// is loaded first so api_key_env can resolve.
func Load() (*Config, error) {
	return LoadForRoot(".")
}

// LoadForRoot builds the configuration with project files resolved from root
// instead of the current working directory. When root is "" or ".", it behaves
// like Load(). This is the workspace-aware entry point: desktop tabs use it so
// each project's ARCDESK.toml + .env + .mcp.json are resolved independently
// without changing the process cwd.
func LoadForRoot(root string) (*Config, error) {
	key := configCacheKey(root)
	fps := sourceFingerprints(configSourcePaths(root))

	configCacheMu.Lock()
	if ent, ok := configCache[key]; ok && fingerprintsMatch(ent.sources, fps) {
		cfg := ent.cfg
		configCacheMu.Unlock()
		return cfg, nil
	}
	configCacheMu.Unlock()

	cfg, err := loadForRootUncached(root)
	if err != nil {
		return nil, err
	}

	configCacheMu.Lock()
	configCache[key] = &configCacheEntry{cfg: cfg, sources: fps}
	configCacheMu.Unlock()
	return cfg, nil
}

// backfillDeepSeekPro restores deepseek-pro for configs the pre-fix setup wizard
// wrote with only deepseek-v4-flash: a keyless /models probe used to drop the Pro
// SKU, leaving users unable to switch to it. In-memory only 鈥?the user's file is
// untouched. Narrowly scoped to the official DeepSeek endpoint (which is known to
// serve pro) so a custom flash-only deployment isn't given an entry that 404s.
func backfillDeepSeekPro(c *Config) {
	const flashModel, proModel = "deepseek-v4-flash", "deepseek-v4-pro"
	var flash *ProviderEntry
	for i := range c.Providers {
		p := &c.Providers[i]
		if p.Name == "deepseek-pro" {
			return
		}
		for _, m := range p.ModelList() {
			switch m {
			case proModel:
				return // pro already reachable
			case flashModel:
				if strings.Contains(p.BaseURL, "api.deepseek.com") {
					flash = p
				}
			}
		}
	}
	if flash == nil {
		return
	}
	for _, bp := range Default().Providers {
		if bp.Name == "deepseek-pro" {
			bp.APIKeyEnv = flash.APIKeyEnv
			c.Providers = append(c.Providers, bp)
			return
		}
	}
}

func sameProviderEndpoint(a, b *ProviderEntry) bool {
	return strings.EqualFold(strings.TrimSpace(a.BaseURL), strings.TrimSpace(b.BaseURL)) &&
		strings.EqualFold(strings.TrimSpace(a.APIKeyEnv), strings.TrimSpace(b.APIKeyEnv))
}

// dedupeRedundantProviders drops single-model providers whose SKU is already
// exposed by another provider's models list on the same endpoint/key.
func dedupeRedundantProviders(c *Config) {
	multiOwner := map[string]int{}
	for i := range c.Providers {
		p := &c.Providers[i]
		if len(p.Models) <= 1 {
			continue
		}
		for _, m := range p.ModelList() {
			if m != "" {
				multiOwner[m] = i
			}
		}
	}
	if len(multiOwner) == 0 {
		return
	}
	kept := make([]ProviderEntry, 0, len(c.Providers))
	for i := range c.Providers {
		p := c.Providers[i]
		models := p.ModelList()
		if len(models) == 1 {
			ownerIdx, ok := multiOwner[models[0]]
			if ok && ownerIdx != i && sameProviderEndpoint(&c.Providers[ownerIdx], &p) {
				continue
			}
		}
		kept = append(kept, p)
	}
	c.Providers = kept
}

// pruneUnconfiguredProviders removes preset providers whose api_key_env is not
// set, so model pickers only reflect APIs the user actually configured. The
// default_model provider is always kept so RequireKey=false boot paths can
// resolve the model and surface a missing-key notice instead of failing early.
func pruneUnconfiguredProviders(c *Config) {
	defaultModel := strings.TrimSpace(c.DefaultModel)
	kept := make([]ProviderEntry, 0, len(c.Providers))
	for _, p := range c.Providers {
		if p.Configured() {
			kept = append(kept, p)
			continue
		}
		if defaultModel != "" && strings.EqualFold(strings.TrimSpace(p.Name), defaultModel) {
			kept = append(kept, p)
		}
	}
	c.Providers = kept
}

func resolveRoot(root string) string {
	if root == "" || root == "." {
		return "."
	}
	return filepath.Clean(root)
}

// normalizeLegacyEffort migrates the retired DeepSeek effort="off" (the old
// /thinking off that disabled thinking) to the provider default, so a config
// written by an older version keeps loading instead of erroring on a value the
// provider no longer accepts.
func normalizeLegacyEffort(c *Config) {
	for i := range c.Providers {
		if strings.EqualFold(strings.TrimSpace(c.Providers[i].Effort), "off") {
			c.Providers[i].Effort = ""
		}
	}
}

// mergeTOMLPlugins merges [[plugins]] across TOML sources by name (later source wins).
func mergeTOMLPlugins(paths []string) ([]PluginEntry, error) {
	var merged []PluginEntry
	index := map[string]int{}
	for _, path := range paths {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		var f Config
		if _, err := toml.DecodeFile(path, &f); err != nil {
			return nil, fmt.Errorf("config %s: %w", path, err)
		}
		for _, p := range f.Plugins {
			if i, ok := index[p.Name]; ok {
				merged[i] = p
				continue
			}
			index[p.Name] = len(merged)
			merged = append(merged, p)
		}
	}
	return merged, nil
}

// LoadForEdit returns a config to seed the `ARCDESK setup` wizard when reconfiguring:
// the built-in defaults with the file at path (if present) decoded on top, so a
// reconfigure preserves the user's existing providers and agent settings instead
// of resetting to defaults. .env is loaded so api_key_env resolution works while
// the wizard decides which keys are still missing.
func LoadForEdit(path string) *Config {
	loadDotEnv()
	cfg := Default()
	if err := mergeFile(cfg, path); err != nil {
		slog.Warn("config: load for edit failed, using defaults", "path", path, "err", err)
	}
	normalizeLegacyEffort(cfg)
	normalizeEffortConfig(cfg)
	return cfg
}

// mergeFile decodes a TOML file onto cfg if it exists. An absent file is not an error.
func mergeFile(cfg *Config, path string) error {
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return fmt.Errorf("config %s: %w", path, err)
	}
	return nil
}

func userConfigPath() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "config.toml")
}

// UserConfigPath is the user-global config file (~/.config/arcdesk/config.toml),
// or "" when the user config dir can't be resolved.
func UserConfigPath() string { return userConfigPath() }

// UserCredentialsPath is the ARCDESK-owned global secrets file, beside
// config.toml in the user config dir (e.g. ~/.config/arcdesk/credentials). It
// holds KEY=value lines loaded into the environment by loadDotEnv. The setup
// wizard writes API keys here, deliberately NOT named .env: keys never land in a
// project's own .env (which can't be selectively gitignored), never get
// committed, and resolve from any working directory. "" when the user config dir
// can't be resolved.
func UserCredentialsPath() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "credentials")
}

// ArchiveDir is where compacted conversation history is archived for
// traceability (one timestamped .jsonl per compaction). Empty if the user config
// directory cannot be resolved, in which case archiving is skipped.
func ArchiveDir() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "archive")
}

// SessionDir is where chat sessions are persisted (one .jsonl per session).
// Used by `ARCDESK chat --continue` / `--resume` to find the recent ones. Empty
// if the user config dir can't be resolved 鈥?sessions then aren't saved.
func SessionDir() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "sessions")
}

// CacheDir is the per-user cache root for derived/regenerable artefacts: MCP
// handshake snapshots, plugin startup-latency telemetry. Lives beside the
// existing dirs (UserConfigDir/arcdesk/...) so the whole ARCDESK state tree
// shares one root the user can wipe in a single rm. Empty when the OS dir is
// unavailable 鈥?callers must tolerate that (caching is best-effort).
func CacheDir() string {
	dir := userConfigDir()
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "cache")
}

// MemoryUserDir returns the ARCDESK user config root (鈥?ARCDESK), under which
// the user-global ARCDESK.md and the per-project auto-memory store live. Empty
// when the user config dir can't be resolved, which disables user-scoped memory.
func MemoryUserDir() string {
	return userConfigDir()
}

// ConventionDirs are the parent directories scanned for agent assets (skills,
// commands), in canonical-first order. .arcdesk is ours; .agents / .agent /
// .claude let users drop in assets authored for other agent tools without moving
// files. Shared so skills (internal/skill) and commands (CommandDirs) discover
// the same set. Note: hooks are NOT scanned across these 鈥?a .claude/settings.json
// uses a different hook schema that can't be parsed as ours, so hooks stay in
// .arcdesk/settings.json (see internal/hook).
var ConventionDirs = []string{ProjectMetaDir, LegacyProjectMetaDir, ".agents", ".agent", ".claude"}

// conventionSubdirsAsc joins sub under each ConventionDir of base, in ascending
// priority (reverse of ConventionDirs) so the canonical .arcdesk ends up the
// highest-priority entry 鈥?command.Load lets a later directory win on a clash.
func conventionSubdirsAsc(base, sub string) []string {
	out := make([]string, 0, len(ConventionDirs))
	for i := len(ConventionDirs) - 1; i >= 0; i-- {
		out = append(out, filepath.Join(base, ConventionDirs[i], sub))
	}
	return out
}

// CommandDirs returns the directories scanned for custom slash commands, lowest
// priority first, so a later (more specific) directory overrides an earlier one
// on a name clash. Order: home-dir convention dirs (~/.claude/commands 鈥?~/.arcdesk/commands),
// the legacy XDG user dir (~/.config/arcdesk/commands), then the project's
// convention dirs (.claude/commands 鈥?.arcdesk/commands). Scanning the .claude /
// .agents / .agent dirs lets commands authored for other agent tools (same .md +
// frontmatter format) work here unchanged.
func CommandDirs() []string {
	return CommandDirsForRoot(".")
}

// CommandDirsForRoot is like CommandDirs but resolves the project convention
// dirs under root instead of the current working directory. Global (home/XDG)
// dirs are unchanged 鈥?they are always user-scoped.
func CommandDirsForRoot(root string) []string {
	root = resolveRoot(root)
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, conventionSubdirsAsc(home, "commands")...)
	}
	if dir, err := os.UserConfigDir(); err == nil {
		dirs = append(dirs, filepath.Join(dir, LegacyConfigDirName, "commands")) // legacy XDG user dir
	}
	dirs = append(dirs, conventionSubdirsAsc(root, "commands")...)
	return dirs
}

// SourcePath returns the highest-priority config file that exists, or "" if none.
func SourcePath() string {
	return SourcePathForRoot(".")
}

// SourcePathForRoot returns the highest-priority config file that exists under
// root, or "" if none. Equivalent to SourcePath() when root is ".".
func SourcePathForRoot(root string) string {
	root = resolveRoot(root)
	projectTOML := projectConfigPathForRoot(root)
	if _, err := os.Stat(projectTOML); err == nil {
		return projectTOML
	}
	if uc := userConfigPath(); uc != "" {
		if _, err := os.Stat(uc); err == nil {
			return uc
		}
	}
	return ""
}

// WriteFile writes the configuration to path as annotated TOML.
func (c *Config) WriteFile(path string) error {
	return os.WriteFile(path, []byte(RenderTOMLForScope(c, renderScopeForPath(path))), 0o644)
}

// Provider returns the named provider entry.
func (c *Config) Provider(name string) (*ProviderEntry, bool) {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			return &c.Providers[i], true
		}
	}
	return nil, false
}

// ResolveModel resolves a model reference to a provider entry whose Model is the
// selected model string (a copy, so the config's lists stay intact). It accepts:
//   - "provider/model" 鈥?that exact model under that provider;
//   - a provider name   鈥?the provider's default model;
//   - a bare model name 鈥?the (first) provider that lists it.
//
// The returned entry is ready to build a provider from (NewProvider reads .Model),
// so a single "vendor with many models" entry yields one instance per model
// without duplicating base_url/api_key_env. Single-`model` entries still resolve
// by provider name, keeping older configs working unchanged.
func (c *Config) ResolveModel(ref string) (*ProviderEntry, bool) {
	if ref == "" {
		return nil, false
	}
	// "provider/model"
	if prov, model, ok := strings.Cut(ref, "/"); ok {
		if e, found := c.Provider(prov); found && e.HasModel(model) {
			cp := *e
			cp.Model = model
			return &cp, true
		}
	}
	// a provider name 鈫?its default model
	if e, found := c.Provider(ref); found {
		cp := *e
		cp.Model = e.DefaultModel()
		return &cp, true
	}
	// a bare model name 鈫?the provider that lists it
	for i := range c.Providers {
		if c.Providers[i].HasModel(ref) {
			cp := c.Providers[i]
			cp.Model = ref
			return &cp, true
		}
	}
	return nil, false
}

// APIKey resolves the entry's API key from its api_key_env.
func (e *ProviderEntry) APIKey() string {
	if e.APIKeyEnv == "" {
		return ""
	}
	return apikey.Normalize(os.Getenv(e.APIKeyEnv))
}

// Configured reports whether the provider's api_key_env is set 鈥?the same check
// Validate enforces, so pickers can filter on it.
func (e *ProviderEntry) Configured() bool {
	return e.APIKey() != ""
}

// ResolveSystemPrompt returns the system prompt, reading system_prompt_file if set.
func (c *Config) ResolveSystemPrompt() (string, error) {
	if c.Agent.SystemPromptFile != "" {
		b, err := os.ReadFile(c.Agent.SystemPromptFile)
		if err != nil {
			return "", fmt.Errorf("system_prompt_file: %w", err)
		}
		return strings.TrimSpace(string(b)), nil
	}
	if strings.TrimSpace(c.Agent.SystemPrompt) == "" {
		return DefaultSystemPrompt, nil
	}
	return c.Agent.SystemPrompt, nil
}

// Validate checks that the selected model's provider is usable.
func (c *Config) Validate(model string) error {
	e, ok := c.ResolveModel(model)
	if !ok {
		return fmt.Errorf("unknown model %q (configured: %s)", model, c.providerNames())
	}
	if e.Kind == "" {
		return fmt.Errorf("provider %q: kind is required", model)
	}
	if e.BaseURL == "" {
		return fmt.Errorf("provider %q: base_url is required", model)
	}
	if e.APIKey() == "" {
		return fmt.Errorf("provider %q: missing env %s", model, e.APIKeyEnv)
	}
	return nil
}

func (c *Config) providerNames() string {
	names := make([]string, len(c.Providers))
	for i, p := range c.Providers {
		names[i] = p.Name
	}
	return strings.Join(names, ", ")
}
