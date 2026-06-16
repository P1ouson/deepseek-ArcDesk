package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"arcdesk/internal/fileutil"
	"arcdesk/internal/mcpdiag"
	"arcdesk/internal/netclient"
	"arcdesk/internal/permission"
)

// edit.go is the programmatic mutation surface a settings UI drives: change the
// default model, add/remove a provider, set the planner, edit permission rules,
// add/remove an MCP server — each validated, then persisted with SaveTo. It is
// separate from the `ARCDESK setup` wizard (cli) so a GUI can apply one setting at a
// time without replaying the whole interactive flow. Every mutator works on the
// in-memory *Config; nothing writes to disk until SaveTo/Save is called, so a UI
// can stage several changes and commit once. Mutations round-trip through
// RenderTOML → Load (the wizard relies on the same guarantee).

// permission rule list names accepted by the rule mutators.
const (
	listAllow = "allow"
	listAsk   = "ask"
	listDeny  = "deny"
)

// SetDefaultModel points default_model at an existing provider. It errors if no
// provider by that name is configured, so a UI can't strand the config on a
// model that doesn't exist.
func (c *Config) SetDefaultModel(name string) error {
	if _, ok := c.Provider(name); !ok {
		return fmt.Errorf("set default: no provider %q (configured: %s)", name, c.providerNames())
	}
	c.DefaultModel = name
	return nil
}

// SetPlannerModel sets (or, with "", clears) agent.planner_model for two-model
// collaboration. A non-empty name must be a configured provider.
func (c *Config) SetPlannerModel(name string) error {
	if name == "" {
		c.Agent.PlannerModel = ""
		return nil
	}
	if _, ok := c.Provider(name); !ok {
		return fmt.Errorf("set planner: no provider %q (configured: %s)", name, c.providerNames())
	}
	c.Agent.PlannerModel = name
	return nil
}

// SetAutoPlan sets the interactive auto-plan gate. "off" keeps plan mode manual;
// "on" opts into automatic read-only planning for complex-looking turns.
// "ask" is accepted as a legacy synonym for "on" but is never written back.
func (c *Config) SetAutoPlan(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "off":
		c.Agent.AutoPlan = "off"
	case "on", "ask":
		c.Agent.AutoPlan = "on"
	default:
		return fmt.Errorf("auto_plan %q: must be off|on", mode)
	}
	return nil
}

// AgentSettingsInput is the editable [agent] surface for the desktop settings panel.
type AgentSettingsInput struct {
	Temperature        float64
	MaxSteps           int
	SystemPrompt       string
	SystemPromptFile   string
	OutputStyle        string
	AutoPlan           string
	AutoPlanClassifier string
	SoftCompactRatio   float64
	CompactRatio       float64
	CompactForceRatio  float64
	SubagentModel      string
	SubagentModels     map[string]string
}

func validateCompactRatio(name string, ratio float64) error {
	if ratio <= 0 || ratio > 1 {
		return fmt.Errorf("%s must be between 0 and 1 (exclusive of 0)", name)
	}
	return nil
}

func (c *Config) validateProviderName(name, field string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	if _, ok := c.Provider(name); !ok {
		return fmt.Errorf("%s: no provider %q (configured: %s)", field, name, c.providerNames())
	}
	return nil
}

// ApplyAgentSettings updates the [agent] section from the desktop settings panel.
func (c *Config) ApplyAgentSettings(in AgentSettingsInput) error {
	if in.Temperature < 0 || in.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}
	if in.MaxSteps < 0 {
		return fmt.Errorf("max_steps must be >= 0")
	}
	if err := validateCompactRatio("soft_compact_ratio", in.SoftCompactRatio); err != nil {
		return err
	}
	if err := validateCompactRatio("compact_ratio", in.CompactRatio); err != nil {
		return err
	}
	if err := validateCompactRatio("compact_force_ratio", in.CompactForceRatio); err != nil {
		return err
	}
	if in.SoftCompactRatio > in.CompactRatio {
		return fmt.Errorf("soft_compact_ratio must be <= compact_ratio")
	}
	if in.CompactRatio > in.CompactForceRatio {
		return fmt.Errorf("compact_ratio must be <= compact_force_ratio")
	}
	if err := c.SetAutoPlan(in.AutoPlan); err != nil {
		return err
	}
	if err := c.validateProviderName(in.AutoPlanClassifier, "auto_plan_classifier"); err != nil {
		return err
	}
	if err := c.validateProviderName(in.SubagentModel, "subagent_model"); err != nil {
		return err
	}
	c.Agent.Temperature = in.Temperature
	c.Agent.MaxSteps = in.MaxSteps
	c.Agent.SystemPrompt = strings.TrimSpace(in.SystemPrompt)
	c.Agent.SystemPromptFile = strings.TrimSpace(in.SystemPromptFile)
	c.Agent.OutputStyle = strings.TrimSpace(in.OutputStyle)
	c.Agent.AutoPlanClassifier = strings.TrimSpace(in.AutoPlanClassifier)
	c.Agent.SoftCompactRatio = in.SoftCompactRatio
	c.Agent.CompactRatio = in.CompactRatio
	c.Agent.CompactForceRatio = in.CompactForceRatio
	c.Agent.SubagentModel = strings.TrimSpace(in.SubagentModel)
	if in.SubagentModels == nil {
		c.Agent.SubagentModels = nil
	} else {
		out := map[string]string{}
		for skill, model := range in.SubagentModels {
			skill = strings.TrimSpace(skill)
			model = strings.TrimSpace(model)
			if skill == "" {
				continue
			}
			if model == "" {
				continue
			}
			if err := c.validateProviderName(model, "subagent_models."+skill); err != nil {
				return err
			}
			out[skill] = model
		}
		if len(out) == 0 {
			c.Agent.SubagentModels = nil
		} else {
			c.Agent.SubagentModels = out
		}
	}
	return nil
}

// UpsertProvider adds e, or replaces an existing provider with the same name
// (preserving its position). Required fields (name, kind, base_url, model) are
// validated; whether the kind is actually registered and the key resolves is
// checked later by provider.New / Validate, which give actionable errors.
func (c *Config) UpsertProvider(e ProviderEntry) error {
	normalizeProviderEffortFields(&e)
	if err := validateProvider(e); err != nil {
		return err
	}
	for i := range c.Providers {
		if c.Providers[i].Name == e.Name {
			c.Providers[i] = e
			return nil
		}
	}
	c.Providers = append(c.Providers, e)
	return nil
}

// SetProviderEffort updates a provider's provider-specific thinking effort knob.
func (c *Config) SetProviderEffort(name, effort string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			c.Providers[i].Effort = strings.ToLower(strings.TrimSpace(effort))
			return nil
		}
	}
	return fmt.Errorf("set provider effort: no provider %q", name)
}

// SetLanguage pins the CLI UI/model language; empty/auto clears the override so runtime detection falls back to ARCDESK_LANG / locale.
func (c *Config) SetLanguage(lang string) error {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "auto":
		c.Language = ""
	case "en":
		c.Language = "en"
	case "zh":
		c.Language = "zh"
	default:
		return fmt.Errorf("language %q: must be auto|en|zh", lang)
	}
	return nil
}

// SetDesktopLanguage pins the desktop UI language. It intentionally does not
// modify Config.Language, which is used by the CLI/model-facing runtime.
func (c *Config) SetDesktopLanguage(lang string) error {
	switch strings.ToLower(strings.TrimSpace(lang)) {
	case "", "auto":
		c.Desktop.Language = ""
	case "en":
		c.Desktop.Language = "en"
	case "zh":
		c.Desktop.Language = "zh"
	default:
		return fmt.Errorf("desktop language %q: must be auto|en|zh", lang)
	}
	return nil
}

// SetDesktopAppearance sets desktop-only theme preferences. It must not affect
// CLI theme settings or provider-visible request data.
func (c *Config) SetDesktopAppearance(theme, style string) error {
	switch strings.ToLower(strings.TrimSpace(theme)) {
	case "auto":
		c.Desktop.Theme = "auto"
	case "light":
		c.Desktop.Theme = "light"
	case "", "dark":
		c.Desktop.Theme = "dark"
	default:
		return fmt.Errorf("desktop theme %q: must be auto|dark|light", theme)
	}
	if strings.TrimSpace(style) == "" {
		c.Desktop.ThemeStyle = ""
		return nil
	}
	normalized := normalizeThemeStyle(style)
	if normalized == "" {
		return fmt.Errorf("desktop theme style %q: must be indigo|graphite|ember|aurora|midnight|cobalt|sandstone|porcelain|linen|glacier", style)
	}
	c.Desktop.ThemeStyle = normalized
	return nil
}

// DesktopAppearanceInput is the editable desktop.appearance surface.
type DesktopAppearanceInput struct {
	BackgroundPreset string
	ForegroundPreset string
	TextSize         string
	CodeFontSize     string
	DiffMarker       string
}

// SetDesktopAppearancePrefs updates desktop surface and typography preferences.
func (c *Config) SetDesktopAppearancePrefs(in DesktopAppearanceInput) error {
	if bg := normalizeDesktopBackgroundPreset(in.BackgroundPreset); bg != "" {
		c.Desktop.Appearance.BackgroundPreset = bg
	} else if strings.TrimSpace(in.BackgroundPreset) != "" {
		return fmt.Errorf("desktop background preset %q: must be paper|white|fog|linen|studio|parchment|charcoal|graphite|slate|midnight|nightfall", in.BackgroundPreset)
	}
	if fg := normalizeDesktopForegroundPreset(in.ForegroundPreset); fg != "" {
		c.Desktop.Appearance.ForegroundPreset = fg
	} else if strings.TrimSpace(in.ForegroundPreset) != "" {
		return fmt.Errorf("desktop foreground preset %q: must be ink|charcoal|slate|snow|silver|white", in.ForegroundPreset)
	}
	if size := normalizeDesktopTextSize(in.TextSize); size != "" {
		c.Desktop.Appearance.TextSize = size
	} else if strings.TrimSpace(in.TextSize) != "" {
		return fmt.Errorf("desktop text size %q: must be small|default|large|xlarge", in.TextSize)
	}
	if size := normalizeDesktopTextSize(in.CodeFontSize); size != "" {
		c.Desktop.Appearance.CodeFontSize = size
	} else if strings.TrimSpace(in.CodeFontSize) != "" {
		return fmt.Errorf("desktop code font size %q: must be small|default|large|xlarge", in.CodeFontSize)
	}
	if marker := normalizeDesktopDiffMarker(in.DiffMarker); marker != "" {
		c.Desktop.Appearance.DiffMarker = marker
	} else if strings.TrimSpace(in.DiffMarker) != "" {
		return fmt.Errorf("desktop diff marker %q: must be background|signs", in.DiffMarker)
	}
	return nil
}

// SetDesktopCodeReviewSettings updates desktop code-review panel defaults.
func (c *Config) SetDesktopCodeReviewSettings(defaultScope string, securityByDefault bool) error {
	c.Desktop.CodeReview.DefaultScope = normalizeDesktopCodeReviewScope(defaultScope)
	c.Desktop.CodeReview.SecurityByDefault = securityByDefault
	return nil
}

// ImportDesktopLocalPrefs writes browser-local desktop prefs into config when the
// corresponding config fields are still empty (one-time migration).
func (c *Config) ImportDesktopLocalPrefs(in DesktopAppearanceInput, codeReviewScope string, codeReviewSecurity bool, hasAppearance, hasCodeReview bool) error {
	if hasAppearance {
		empty := c.Desktop.Appearance == DesktopAppearanceConfig{}
		if empty {
			if err := c.SetDesktopAppearancePrefs(in); err != nil {
				return err
			}
		}
	}
	if hasCodeReview && c.Desktop.CodeReview == (DesktopCodeReviewConfig{}) {
		if err := c.SetDesktopCodeReviewSettings(codeReviewScope, codeReviewSecurity); err != nil {
			return err
		}
	}
	return nil
}

// SetDesktopGitSettings updates desktop-only Git UI preferences.
func (c *Config) SetDesktopGitSettings(
	mergeMethod string,
	checkGitHubCli, syncRepoMergeToGitHub bool,
	commitInstructions, prInstructions string,
) error {
	switch strings.ToLower(strings.TrimSpace(mergeMethod)) {
	case "squash", "rebase":
		c.Desktop.Git.PRMergeMethod = strings.ToLower(strings.TrimSpace(mergeMethod))
	default:
		c.Desktop.Git.PRMergeMethod = "merge"
	}
	c.Desktop.Git.CheckGitHubCli = checkGitHubCli
	c.Desktop.Git.SyncRepoMergeToGitHub = syncRepoMergeToGitHub
	c.Desktop.Git.CommitInstructions = strings.TrimSpace(commitInstructions)
	c.Desktop.Git.PRInstructions = strings.TrimSpace(prInstructions)
	return nil
}

// SetDesktopTerminalShell sets the integrated terminal shell preference.
func (c *Config) SetDesktopTerminalShell(shell string) error {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "", "auto":
		c.Desktop.TerminalShell = ""
	case "powershell", "pwsh":
		c.Desktop.TerminalShell = "powershell"
	case "cmd", "command-prompt":
		c.Desktop.TerminalShell = "cmd"
	case "git-bash", "gitbash", "bash":
		c.Desktop.TerminalShell = "git-bash"
	case "wsl":
		c.Desktop.TerminalShell = "wsl"
	default:
		return fmt.Errorf("desktop terminal shell %q: must be auto|powershell|cmd|git-bash|wsl", shell)
	}
	return nil
}

// SetDesktopCloseBehavior sets the desktop close-window preference. It is
// intentionally UI-only and must not affect model prompts or provider-visible
// request data.
func (c *Config) SetDesktopCloseBehavior(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "quit", "exit":
		c.Desktop.CloseBehavior = "quit"
	case "", "background", "hide":
		c.Desktop.CloseBehavior = "background"
	default:
		return fmt.Errorf("close behavior %q: must be quit|background", mode)
	}
	return nil
}

// SetUICloseBehavior is kept for callers compiled against the old edit API.
func (c *Config) SetUICloseBehavior(mode string) error {
	return c.SetDesktopCloseBehavior(mode)
}

// SetProviderThinking updates a provider's provider-specific thinking mode knob.
func (c *Config) SetProviderThinking(name, thinking string) error {
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			c.Providers[i].Thinking = strings.ToLower(strings.TrimSpace(thinking))
			return nil
		}
	}
	return fmt.Errorf("set provider thinking: no provider %q", name)
}

// SetNetwork updates ordinary outbound network proxy settings. Invalid custom
// proxy settings are rejected here so the desktop panel cannot save a config that
// would break provider startup.
func (c *Config) SetNetwork(n NetworkConfig) error {
	n.ProxyMode = netclient.NormalizeMode(n.ProxyMode)
	n.ProxyURL = strings.TrimSpace(n.ProxyURL)
	n.NoProxy = strings.TrimSpace(n.NoProxy)
	n.Proxy.Type = strings.ToLower(strings.TrimSpace(n.Proxy.Type))
	n.Proxy.Server = strings.TrimSpace(n.Proxy.Server)
	n.Proxy.Username = strings.TrimSpace(n.Proxy.Username)
	c.Network = n
	return netclient.Validate(c.NetworkProxySpec())
}

// RemoveProvider deletes the named provider. It refuses to remove the current
// default_model (reassign it first, so the config never points at a missing
// model); if the removed provider was the planner, planner_model is cleared as
// a side effect since it is optional. Errors when the name isn't configured.
func (c *Config) RemoveProvider(name string) error {
	idx := -1
	for i := range c.Providers {
		if c.Providers[i].Name == name {
			idx = i
			break
		}
	}
	if idx < 0 {
		return fmt.Errorf("remove provider: no provider %q", name)
	}
	if c.DefaultModel == name {
		return fmt.Errorf("remove provider: %q is the default model — set a different default_model first", name)
	}
	c.Providers = append(c.Providers[:idx], c.Providers[idx+1:]...)
	if c.Agent.PlannerModel == name {
		c.Agent.PlannerModel = ""
	}
	return nil
}

// validateProvider checks the fields a provider can't function without.
func validateProvider(e ProviderEntry) error {
	switch {
	case strings.TrimSpace(e.Name) == "":
		return fmt.Errorf("provider: name is required")
	case strings.TrimSpace(e.Kind) == "":
		return fmt.Errorf("provider %q: kind is required", e.Name)
	case strings.TrimSpace(e.BaseURL) == "":
		return fmt.Errorf("provider %q: base_url is required", e.Name)
	case strings.TrimSpace(e.Model) == "":
		return fmt.Errorf("provider %q: model is required", e.Name)
	}
	return nil
}

// SetPermissionMode sets the writer-fallback mode. Accepts "ask", "allow", or
// "deny" (case-insensitive); anything else errors rather than silently
// defaulting, so a UI surfaces a typo instead of installing a surprising mode.
func (c *Config) SetPermissionMode(mode string) error {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "ask", "allow", "deny":
		c.Permissions.Mode = strings.ToLower(strings.TrimSpace(mode))
		return nil
	default:
		return fmt.Errorf("permission mode %q: must be ask|allow|deny", mode)
	}
}

// AddPermissionRule appends a rule ("ToolName" or "ToolName(glob)") to the
// allow / ask / deny list. The rule is validated with the same parser the gate
// uses, and a duplicate is a no-op so a UI can call it idempotently.
func (c *Config) AddPermissionRule(list, rule string) error {
	target, err := c.ruleList(list)
	if err != nil {
		return err
	}
	rule = strings.TrimSpace(rule)
	if _, ok := permission.ParseRule(rule); !ok {
		return fmt.Errorf("invalid permission rule %q (want \"ToolName\" or \"ToolName(glob)\")", rule)
	}
	for _, existing := range *target {
		if existing == rule {
			return nil // already present
		}
	}
	*target = append(*target, rule)
	return nil
}

// RemovePermissionRule drops the first exact match of rule from the named list,
// reporting whether anything was removed.
func (c *Config) RemovePermissionRule(list, rule string) (bool, error) {
	target, err := c.ruleList(list)
	if err != nil {
		return false, err
	}
	rule = strings.TrimSpace(rule)
	for i, existing := range *target {
		if existing == rule {
			*target = append((*target)[:i], (*target)[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// ruleList returns a pointer to the named rule slice so mutators can append to
// it in place. An unknown list name errors.
func (c *Config) ruleList(list string) (*[]string, error) {
	switch strings.ToLower(strings.TrimSpace(list)) {
	case listAllow:
		return &c.Permissions.Allow, nil
	case listAsk:
		return &c.Permissions.Ask, nil
	case listDeny:
		return &c.Permissions.Deny, nil
	default:
		return nil, fmt.Errorf("unknown permission list %q (want allow|ask|deny)", list)
	}
}

// AddSkillPath appends a custom skill root, deduping by its expanded absolute
// path while preserving the caller's original spelling in the config file.
func (c *Config) AddSkillPath(path string) error {
	path = strings.TrimSpace(path)
	if path == "" {
		return fmt.Errorf("skill path: empty path")
	}
	want := CanonicalSkillPath(path)
	for _, existing := range c.Skills.Paths {
		if CanonicalSkillPath(existing) == want {
			return nil
		}
	}
	c.Skills.Paths = append(c.Skills.Paths, path)
	return nil
}

// RemoveSkillPath removes the first custom skill root matching path after
// expansion and path cleaning. It reports whether anything changed.
func (c *Config) RemoveSkillPath(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return false, fmt.Errorf("skill path: empty path")
	}
	want := CanonicalSkillPath(path)
	for i, existing := range c.Skills.Paths {
		if CanonicalSkillPath(existing) == want {
			c.Skills.Paths = append(c.Skills.Paths[:i], c.Skills.Paths[i+1:]...)
			return true, nil
		}
	}
	return false, nil
}

// SetSkillEnabled persists a per-skill enable/disable preference. Skills are
// enabled by default; disabling records the name, enabling removes it.
func (c *Config) SetSkillEnabled(name string, enabled bool) error {
	name = strings.TrimSpace(name)
	key := SkillNameKey(name)
	if key == "" {
		return fmt.Errorf("skill name %q: use letters, digits, '_', '-', '.', 1-64 chars, starting alphanumeric", name)
	}
	next := c.DisabledSkillNames()
	idx := -1
	for i, existing := range next {
		if SkillNameKey(existing) == key {
			idx = i
			break
		}
	}
	if enabled {
		if idx >= 0 {
			next = append(next[:idx], next[idx+1:]...)
		}
		c.Skills.DisabledSkills = next
		return nil
	}
	if idx < 0 {
		next = append(next, name)
	}
	c.Skills.DisabledSkills = next
	return nil
}

// CanonicalSkillPath expands env vars, ~ and relative segments to an absolute
// cleaned path for comparing skill roots. On Windows it folds case so paths that
// differ only in casing dedupe. Use only for comparison, never as stored config.
func CanonicalSkillPath(path string) string {
	path = ExpandVars(strings.TrimSpace(path))
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.Clean(path)
	if runtime.GOOS == "windows" {
		return strings.ToLower(path)
	}
	return path
}

// UpsertPlugin adds e, or replaces an MCP server with the same name (preserving
// position). The transport-specific required fields are validated: stdio needs
// a command, http/sse need a url.
func (c *Config) UpsertPlugin(e PluginEntry) error {
	if err := validatePlugin(e); err != nil {
		return err
	}
	for i := range c.Plugins {
		if c.Plugins[i].Name == e.Name {
			c.Plugins[i] = e
			return nil
		}
	}
	c.Plugins = append(c.Plugins, e)
	return nil
}

// RemovePlugin deletes the named MCP server, reporting whether it was present.
func (c *Config) RemovePlugin(name string) bool {
	for i := range c.Plugins {
		if c.Plugins[i].Name == name {
			c.Plugins = append(c.Plugins[:i], c.Plugins[i+1:]...)
			return true
		}
	}
	return false
}

// ClearPluginAuthentication removes locally stored auth-like material for one
// MCP server while keeping the server entry itself. It intentionally leaves
// non-auth config (command, URL host/path, ordinary env/header keys, tier) alone.
func (c *Config) ClearPluginAuthentication(name string) (PluginEntry, bool, error) {
	for i := range c.Plugins {
		if c.Plugins[i].Name != name {
			continue
		}
		headers, env, url, changed := mcpdiag.ClearAuthConfig(c.Plugins[i].Headers, c.Plugins[i].Env, c.Plugins[i].URL)
		c.Plugins[i].Headers = headers
		c.Plugins[i].Env = env
		c.Plugins[i].URL = url
		return c.Plugins[i], changed, nil
	}
	return PluginEntry{}, false, fmt.Errorf("clear plugin authentication: no plugin %q", name)
}

// ClearPluginAuthenticationInSource clears auth material in the file that actually
// owns the MCP server. Load() merges user/project TOML and project .mcp.json into
// one Config, so callers must not mutate that merged view and Save() it back: a
// .mcp.json-only server would otherwise be serialized into ARCDESK.toml or the
// user config. Source priority mirrors Load(): project TOML, user TOML, then the
// project .mcp.json entry if TOML did not define that server.
func ClearPluginAuthenticationInSource(name string) (PluginEntry, bool, string, error) {
	if path := pluginTOMLSourcePath(name); path != "" {
		cfg := LoadForEdit(path)
		updated, changed, err := cfg.ClearPluginAuthentication(name)
		if err != nil {
			return PluginEntry{}, false, path, err
		}
		if changed {
			if err := cfg.SaveTo(path); err != nil {
				return PluginEntry{}, false, path, err
			}
		}
		return updated, changed, path, nil
	}
	updated, changed, err := clearMCPJSONAuthentication(mcpJSONFile, name)
	if err != nil {
		return PluginEntry{}, false, "", err
	}
	return updated, changed, mcpJSONFile, nil
}

func pluginTOMLSourcePath(name string) string {
	for _, path := range []string{"arcdesk.toml", userConfigPath()} {
		if strings.TrimSpace(path) == "" {
			continue
		}
		cfg := LoadForEdit(path)
		for _, p := range cfg.Plugins {
			if p.Name == name {
				return path
			}
		}
	}
	return ""
}

// validatePlugin checks a plugin entry by transport. An empty Type means stdio.
func validatePlugin(e PluginEntry) error {
	if strings.TrimSpace(e.Name) == "" {
		return fmt.Errorf("plugin: name is required")
	}
	switch strings.ToLower(strings.TrimSpace(e.Type)) {
	case "", "stdio":
		if strings.TrimSpace(e.Command) == "" {
			return fmt.Errorf("plugin %q: command is required for a stdio server", e.Name)
		}
	case "http", "sse", "streamable-http":
		if strings.TrimSpace(e.URL) == "" {
			return fmt.Errorf("plugin %q: url is required for a %s server", e.Name, e.Type)
		}
	default:
		return fmt.Errorf("plugin %q: unknown type %q (want stdio|http|sse)", e.Name, e.Type)
	}
	return nil
}

// SaveTo writes the configuration to path as annotated TOML, atomically: it
// writes a sibling temp file then renames, so a crash mid-write can't leave a
// half-written ARCDESK.toml that fails to parse on next load. Parent directories
// are created as needed.
func (c *Config) SaveTo(path string) error {
	return c.SaveToScope(path, renderScopeForPath(path))
}

func (c *Config) SaveToScope(path string, scope RenderScope) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("save: empty config path")
	}
	return writeConfigFile(path, RenderTOMLForScope(c, scope))
}

// SaveMinimalProjectAutoPlan writes a new project config that only overrides
// [agent].auto_plan. It is intentionally minimal so toggling a project-local
// auto-plan preference in an otherwise unconfigured workspace does not pin
// default_model or providers from built-in defaults.
func SaveMinimalProjectAutoPlan(path, mode string) (string, error) {
	cfg := Default()
	if err := cfg.SetAutoPlan(mode); err != nil {
		return "", err
	}
	body := fmt.Sprintf(`# ARCDESK project configuration.
# Project-local overrides are merged over the user config.

[agent]
auto_plan = %q
`, cfg.Agent.AutoPlan)
	return cfg.Agent.AutoPlan, writeConfigFile(path, body)
}

func writeConfigFile(path, body string) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("save: empty config path")
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("save: create dir: %w", err)
	}
	tmp, err := os.CreateTemp(dir, ".arcdesk.*.toml.tmp")
	if err != nil {
		return fmt.Errorf("save: create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("save: write: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("save: close temp: %w", err)
	}
	return fileutil.ReplaceFile(tmpPath, path)
}

func renderScopeForPath(path string) RenderScope {
	if isUserConfigPath(path) {
		return RenderScopeUser
	}
	return RenderScopeProject
}

func isUserConfigPath(path string) bool {
	path = strings.TrimSpace(path)
	uc := strings.TrimSpace(userConfigPath())
	if path == "" || uc == "" {
		return false
	}
	pathAbs, pathErr := filepath.Abs(path)
	ucAbs, ucErr := filepath.Abs(uc)
	if pathErr == nil && ucErr == nil {
		return filepath.Clean(pathAbs) == filepath.Clean(ucAbs)
	}
	return filepath.Clean(path) == filepath.Clean(uc)
}

// Save writes the configuration back to the file it was loaded from
// (SourcePath), or to ./ARCDESK.toml when none exists yet — the conventional
// project-local target a fresh GUI session would create.
func (c *Config) Save() error {
	path := SourcePath()
	if path == "" {
		path = "arcdesk.toml"
	}
	return c.SaveTo(path)
}

// SaveForRoot saves the config to root's ARCDESK.toml, falling back to the
// user's global config when root has no existing ARCDESK.toml.
func (c *Config) SaveForRoot(root string) error {
	root = resolveRoot(root)
	projectTOML := "arcdesk.toml"
	if root != "." {
		projectTOML = filepath.Join(root, "arcdesk.toml")
	}
	if _, err := os.Stat(projectTOML); err == nil {
		return c.SaveTo(projectTOML)
	}
	if uc := userConfigPath(); uc != "" {
		if err := os.MkdirAll(filepath.Dir(uc), 0o755); err != nil {
			return err
		}
		return c.SaveTo(uc)
	}
	return c.SaveTo(projectTOML)
}
