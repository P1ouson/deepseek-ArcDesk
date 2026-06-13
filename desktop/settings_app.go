package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
	"arcdesk/internal/outputstyle"
	"arcdesk/internal/provider"
	"arcdesk/internal/provider/apikey"
)

// settings_app.go is the desktop Settings panel's command surface: it reads the
// resolved config and applies edits through internal/config/edit.go (the
// purpose-built mutation API), then rebuilds the controller so the change takes
// effect live �?the same snapshot→reload→resume pattern as SetModel. Secrets are
// the exception: they go to the global credentials file (upsertDotEnv), since
// config stores only the env-var name, not the key.

// --- read ---

type ProviderView struct {
	Name             string   `json:"name"`
	Kind             string   `json:"kind"`
	BaseURL          string   `json:"baseUrl"`
	Models           []string `json:"models"`
	Default          string   `json:"default"`
	APIKeyEnv        string   `json:"apiKeyEnv"`
	KeySet           bool     `json:"keySet"` // the env var currently resolves to a non-empty value
	BalanceURL       string   `json:"balanceUrl"`
	ContextWindow    int      `json:"contextWindow"`
	SupportedEfforts []string `json:"supportedEfforts"`
	DefaultEffort    string   `json:"defaultEffort"`
}

type ProviderModelsResult struct {
	Provider string   `json:"provider"`
	Models   []string `json:"models"`
}

type PermissionsView struct {
	Mode  string   `json:"mode"`
	Allow []string `json:"allow"`
	Ask   []string `json:"ask"`
	Deny  []string `json:"deny"`
}

type SandboxView struct {
	Bash          string   `json:"bash"`
	Network       bool     `json:"network"`
	WorkspaceRoot string   `json:"workspaceRoot"`
	AllowWrite    []string `json:"allowWrite"`
}

type NetworkProxyView struct {
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type NetworkView struct {
	ProxyMode string           `json:"proxyMode"`
	ProxyURL  string           `json:"proxyUrl"`
	NoProxy   string           `json:"noProxy"`
	Proxy     NetworkProxyView `json:"proxy"`
}

type AgentView struct {
	Temperature        float64            `json:"temperature"`
	MaxSteps           int                  `json:"maxSteps"`
	SystemPrompt       string               `json:"systemPrompt"`
	SystemPromptFile   string               `json:"systemPromptFile"`
	OutputStyle        string               `json:"outputStyle"`
	AutoPlan           string               `json:"autoPlan"`
	AutoPlanClassifier string               `json:"autoPlanClassifier"`
	SoftCompactRatio   float64              `json:"softCompactRatio"`
	CompactRatio       float64              `json:"compactRatio"`
	CompactForceRatio  float64              `json:"compactForceRatio"`
	SubagentModel      string               `json:"subagentModel"`
	SubagentModels     map[string]string    `json:"subagentModels"`
	UsesDefaultPrompt  bool                 `json:"usesDefaultPrompt"`
	DefaultSystemPrompt string              `json:"defaultSystemPrompt"`
}

type OutputStyleView struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Builtin     bool   `json:"builtin"`
}

type AgentSettingsInput struct {
	Temperature        float64            `json:"temperature"`
	MaxSteps           int                  `json:"maxSteps"`
	SystemPrompt       string               `json:"systemPrompt"`
	SystemPromptFile   string               `json:"systemPromptFile"`
	OutputStyle        string               `json:"outputStyle"`
	AutoPlan           string               `json:"autoPlan"`
	AutoPlanClassifier string               `json:"autoPlanClassifier"`
	SoftCompactRatio   float64              `json:"softCompactRatio"`
	CompactRatio       float64              `json:"compactRatio"`
	CompactForceRatio  float64              `json:"compactForceRatio"`
	SubagentModel      string               `json:"subagentModel"`
	SubagentModels     map[string]string    `json:"subagentModels"`
}

type DesktopGitView struct {
	PRMergeMethod         string `json:"prMergeMethod"`
	CheckGitHubCli        bool   `json:"checkGitHubCli"`
	SyncRepoMergeToGitHub bool   `json:"syncRepoMergeToGitHub"`
	CommitInstructions    string `json:"commitInstructions"`
	PRInstructions        string `json:"prInstructions"`
}

type DesktopAppearanceView struct {
	BackgroundPreset string `json:"backgroundPreset"`
	ForegroundPreset string `json:"foregroundPreset"`
	TextSize         string `json:"textSize"`
	CodeFontSize     string `json:"codeFontSize"`
	DiffMarker       string `json:"diffMarker"`
}

type DesktopCodeReviewView struct {
	DefaultScope      string `json:"defaultScope"`
	SecurityByDefault bool   `json:"securityByDefault"`
}

type DesktopLocalPrefsMigrationInput struct {
	BackgroundPreset   string `json:"backgroundPreset"`
	ForegroundPreset   string `json:"foregroundPreset"`
	TextSize           string `json:"textSize"`
	CodeFontSize       string `json:"codeFontSize"`
	DiffMarker         string `json:"diffMarker"`
	CodeReviewScope    string `json:"codeReviewScope"`
	CodeReviewSecurity bool   `json:"codeReviewSecurity"`
	HasAppearance      bool   `json:"hasAppearance"`
	HasCodeReview      bool   `json:"hasCodeReview"`
}

type ProjectPreviewSettingsInput struct {
	PreviewHosts  []string `json:"previewHosts"`
	PreviewPorts  []int    `json:"previewPorts"`
	PreviewStrict bool     `json:"previewStrict"`
}

// SettingsView is the whole Settings panel payload.
type SettingsView struct {
	DefaultModel      string          `json:"defaultModel"`
	PlannerModel      string          `json:"plannerModel"`
	AutoPlan          string          `json:"autoPlan"`
	Providers         []ProviderView  `json:"providers"`
	Permissions       PermissionsView `json:"permissions"`
	Sandbox           SandboxView     `json:"sandbox"`
	Network           NetworkView     `json:"network"`
	Agent             AgentView       `json:"agent"`
	DesktopLanguage     string          `json:"desktopLanguage"`
	DesktopTheme        string          `json:"desktopTheme"`
	DesktopThemeStyle   string          `json:"desktopThemeStyle"`
	DesktopTerminalShell string         `json:"desktopTerminalShell"`
	DesktopGit           DesktopGitView `json:"desktopGit"`
	DesktopAppearance    DesktopAppearanceView `json:"desktopAppearance"`
	DesktopCodeReview    DesktopCodeReviewView `json:"desktopCodeReview"`
	CloseBehavior       string          `json:"closeBehavior"`
	ConfigPath        string          `json:"configPath"`
	// ProviderKinds lists the provider implementations the kernel actually
	// registered (provider.Kinds()), so the editor's "kind" picker offers only
	// kinds that resolve �?selecting an unregistered one would fail the rebuild.
	ProviderKinds []string `json:"providerKinds"`
	// Bypass is the live YOLO state (runtime-only, not from config), so the panel's
	// toggle reflects whether approvals are currently being skipped this session.
	Bypass bool `json:"bypass"`
}

func agentViewFromConfig(cfg *config.Config) AgentView {
	def := config.Default()
	soft := cfg.Agent.SoftCompactRatio
	if soft <= 0 {
		soft = def.Agent.SoftCompactRatio
	}
	compact := cfg.Agent.CompactRatio
	if compact <= 0 {
		compact = def.Agent.CompactRatio
	}
	force := cfg.Agent.CompactForceRatio
	if force <= 0 {
		force = def.Agent.CompactForceRatio
	}
	subagentModels := cfg.Agent.SubagentModels
	if subagentModels == nil {
		subagentModels = map[string]string{}
	}
	prompt := cfg.Agent.SystemPrompt
	usesDefault := strings.TrimSpace(prompt) == "" || prompt == def.Agent.SystemPrompt
	return AgentView{
		Temperature:        cfg.Agent.Temperature,
		MaxSteps:           cfg.Agent.MaxSteps,
		SystemPrompt:       prompt,
		SystemPromptFile:   cfg.Agent.SystemPromptFile,
		OutputStyle:        cfg.Agent.OutputStyle,
		AutoPlan:           desktopAutoPlanMode(cfg.Agent.AutoPlan),
		AutoPlanClassifier: cfg.Agent.AutoPlanClassifier,
		SoftCompactRatio:   soft,
		CompactRatio:       compact,
		CompactForceRatio:  force,
		SubagentModel:      cfg.Agent.SubagentModel,
		SubagentModels:     subagentModels,
		UsesDefaultPrompt:  usesDefault,
		DefaultSystemPrompt: config.DefaultSystemPrompt,
	}
}

func agentSettingsFromView(in AgentSettingsInput) config.AgentSettingsInput {
	subagentModels := in.SubagentModels
	if subagentModels == nil {
		subagentModels = map[string]string{}
	}
	return config.AgentSettingsInput{
		Temperature:        in.Temperature,
		MaxSteps:           in.MaxSteps,
		SystemPrompt:       in.SystemPrompt,
		SystemPromptFile:   in.SystemPromptFile,
		OutputStyle:        in.OutputStyle,
		AutoPlan:           in.AutoPlan,
		AutoPlanClassifier: in.AutoPlanClassifier,
		SoftCompactRatio:   in.SoftCompactRatio,
		CompactRatio:       in.CompactRatio,
		CompactForceRatio:  in.CompactForceRatio,
		SubagentModel:      in.SubagentModel,
		SubagentModels:     subagentModels,
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func desktopThemeStyleForSettings(cfg *config.Config) string {
	if cfg == nil {
		return "glacier"
	}
	if style := cfg.DesktopThemeStyle(); style != "" {
		return style
	}
	if cfg.DesktopTheme() == "dark" {
		return "cobalt"
	}
	return "glacier"
}

// Settings returns the current configuration for the Settings panel.
func (a *App) Settings() SettingsView {
	cfg, cfgPath, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return SettingsView{
			Providers:     []ProviderView{},
			ProviderKinds: nonNil(provider.Kinds()),
			Permissions: PermissionsView{
				Mode:  "ask",
				Allow: []string{},
				Ask:   []string{},
				Deny:  []string{},
			},
			Sandbox:           SandboxView{Bash: "enforce", AllowWrite: []string{}},
			AutoPlan:          "off",
			Agent:             agentViewFromConfig(config.Default()),
			DesktopTheme:      "light",
			DesktopThemeStyle: "glacier",
			DesktopGit: desktopGitView(config.DesktopGitConfig{
				PRMergeMethod: "merge",
			}),
			CloseBehavior:     "quit",
		}
	}
	ctrl := a.activeCtrl()
	bash := cfg.Sandbox.Bash
	if bash == "" {
		bash = "enforce"
	}
	v := SettingsView{
		DefaultModel: cfg.DefaultModel,
		PlannerModel: cfg.Agent.PlannerModel,
		AutoPlan:     desktopAutoPlanMode(cfg.Agent.AutoPlan),
		Providers:    []ProviderView{},
		Permissions: PermissionsView{
			Mode:  orDefault(cfg.Permissions.Mode, "ask"),
			Allow: nonNil(cfg.Permissions.Allow),
			Ask:   nonNil(cfg.Permissions.Ask),
			Deny:  nonNil(cfg.Permissions.Deny),
		},
		Sandbox: SandboxView{
			Bash: bash, Network: cfg.Sandbox.Network,
			WorkspaceRoot: cfg.Sandbox.WorkspaceRoot, AllowWrite: nonNil(cfg.Sandbox.AllowWrite),
		},
		Network: NetworkView{
			ProxyMode: cfg.NetworkProxyMode(),
			ProxyURL:  cfg.Network.ProxyURL,
			NoProxy:   cfg.Network.NoProxy,
			Proxy: NetworkProxyView{
				Type:     orDefault(cfg.Network.Proxy.Type, "socks5"),
				Server:   cfg.Network.Proxy.Server,
				Port:     cfg.Network.Proxy.Port,
				Username: cfg.Network.Proxy.Username,
				Password: cfg.Network.Proxy.Password,
			},
		},
		Agent:             agentViewFromConfig(cfg),
		DesktopLanguage:      cfg.DesktopLanguage(),
		DesktopTheme:         cfg.DesktopTheme(),
		DesktopThemeStyle:    desktopThemeStyleForSettings(cfg),
		DesktopTerminalShell: cfg.DesktopTerminalShell(),
		DesktopGit: desktopGitView(cfg.DesktopGitSettings()),
		DesktopAppearance:    desktopAppearanceView(cfg.DesktopAppearanceSettings()),
		DesktopCodeReview:    desktopCodeReviewView(cfg.DesktopCodeReviewSettings()),
		CloseBehavior:        cfg.DesktopCloseBehavior(),
		ConfigPath:        cfgPath,
		ProviderKinds:     nonNil(provider.Kinds()),
		Bypass:            ctrl != nil && ctrl.Bypass(),
	}
	for i := range cfg.Providers {
		p := &cfg.Providers[i]
		v.Providers = append(v.Providers, ProviderView{
			Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL,
			Models: nonNil(p.ListedModels()), Default: p.DefaultModel(),
			APIKeyEnv:        p.APIKeyEnv,
			KeySet:           p.APIKeyEnv != "" && os.Getenv(p.APIKeyEnv) != "",
			BalanceURL:       p.BalanceURL,
			ContextWindow:    p.ContextWindow,
			SupportedEfforts: nonNil(p.SupportedEfforts),
			DefaultEffort:    p.DefaultEffort,
		})
	}
	return v
}

func orDefault(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

func desktopGitView(g config.DesktopGitConfig) DesktopGitView {
	return DesktopGitView{
		PRMergeMethod:         g.PRMergeMethod,
		CheckGitHubCli:        g.CheckGitHubCli,
		SyncRepoMergeToGitHub: g.SyncRepoMergeToGitHub,
		CommitInstructions:    g.CommitInstructions,
		PRInstructions:        g.PRInstructions,
	}
}

func desktopAppearanceView(a config.DesktopAppearanceConfig) DesktopAppearanceView {
	return DesktopAppearanceView{
		BackgroundPreset: a.BackgroundPreset,
		ForegroundPreset: a.ForegroundPreset,
		TextSize:         orDefault(a.TextSize, "default"),
		CodeFontSize:     orDefault(a.CodeFontSize, "default"),
		DiffMarker:       orDefault(a.DiffMarker, "background"),
	}
}

func desktopCodeReviewView(cr config.DesktopCodeReviewConfig) DesktopCodeReviewView {
	return DesktopCodeReviewView{
		DefaultScope:      orDefault(cr.DefaultScope, "all"),
		SecurityByDefault: cr.SecurityByDefault,
	}
}

// --- apply (write config, then rebuild the controller so it's live) ---

// applyConfigChange mutates the user-global config and rebuilds the controller so
// the change takes effect this session. Desktop settings such as providers and
// keys are account-level, not per-project: writing them to the global config
// rather than the cwd's ARCDESK.toml is what lets them survive a workspace switch.
func (a *App) applyConfigChange(mutate func(*config.Config) error) error {
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	if err := cfg.SaveTo(path); err != nil {
		return err
	}
	config.InvalidateConfigCache("")
	if tab := a.activeTab(); tab != nil {
		a.invalidateWorkspaceKitForRoot(tab.Scope, tab.WorkspaceRoot)
	}
	return a.rebuild()
}

func (a *App) applyConfigOnly(mutate func(*config.Config) error) error {
	cfg, path, err := a.loadDesktopUserConfigForEdit()
	if err != nil {
		return err
	}
	if err := mutate(cfg); err != nil {
		return err
	}
	return cfg.SaveTo(path)
}

func (a *App) loadDesktopUserConfigForEdit() (*config.Config, string, error) {
	userPath := config.UserConfigPath()
	if userPath == "" {
		return nil, "", fmt.Errorf("cannot resolve user config directory")
	}
	if _, err := os.Stat(userPath); err == nil {
		return config.LoadForEdit(userPath), userPath, nil
	}
	cfg := config.LoadForEdit(userPath)
	legacyPath := config.SourcePathForRoot(a.activeWorkspaceRoot())
	if legacyPath == "" || sameConfigPath(legacyPath, userPath) {
		return cfg, userPath, nil
	}
	legacyCfg := config.LoadForEdit(legacyPath)
	legacyCfg.ConfigVersion = config.Default().ConfigVersion
	return legacyCfg, userPath, nil
}

func (a *App) activeWorkspaceRoot() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if tab := a.activeTabLocked(); tab != nil {
		return tab.WorkspaceRoot
	}
	return "."
}

func sameConfigPath(a, b string) bool {
	a = strings.TrimSpace(a)
	b = strings.TrimSpace(b)
	if a == "" || b == "" {
		return false
	}
	aAbs, aErr := filepath.Abs(a)
	bAbs, bErr := filepath.Abs(b)
	if aErr == nil && bErr == nil {
		return filepath.Clean(aAbs) == filepath.Clean(bAbs)
	}
	return filepath.Clean(a) == filepath.Clean(b)
}

// rebuild tears down the controller and rebuilds it from the (just-changed)
// config, carrying the conversation forward. It keeps the active model if it
// still resolves; otherwise it falls back to the new default. Mirrors SetModel.
func (a *App) rebuild() error {
	if a.ctx == nil {
		return nil
	}
	tab := a.activeTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	a.mu.Lock()
	tab.Ready = false
	tab.StartupErr = ""
	a.mu.Unlock()
	var carried []provider.Message
	prevPath := ""
	if tab.Ctrl != nil {
		prevPath = tab.Ctrl.SessionPath()
		_ = tab.Ctrl.Snapshot()
		carried = tab.Ctrl.History()
		a.closeTabController(tab)
	}
	model := tab.model
	if cfg, err := config.LoadForRoot(tab.WorkspaceRoot); err == nil {
		if _, ok := cfg.ResolveModel(model); !ok {
			model = cfg.DefaultModel
			if e, ok := cfg.ResolveModel(model); ok {
				model = e.Name + "/" + e.Model
			}
		}
	}
	buildCtx, cancel := context.WithTimeout(a.bootContext(), 90*time.Second)
	defer cancel()
	ctrl, _, err := a.buildControllerForTab(tab, buildCtx, model, cloneStringPtr(tab.effort))
	if err != nil {
		a.mu.Lock()
		tab.StartupErr = err.Error()
		tab.Ready = true
		a.mu.Unlock()
		a.emitReady(a.ctx)
		return err
	}
	a.mu.Lock()
	tab.Ctrl = ctrl
	tab.model = model
	tab.Label = ctrl.Label()
	tab.StartupErr = ""
	tab.Ready = true
	a.saveTabsLocked()
	a.mu.Unlock()
	a.emitReady(a.ctx)
	enableDesktopInteractive(ctrl)
	registerDesktopSessionTools(a, ctrl)
	applyTabModeToController(ctrl, tab.mode)
	path := agent.ContinueSessionPath(prevPath, ctrl.SessionDir(), ctrl.Label())
	if len(carried) > 0 {
		carried = withFreshSystemPrompt(carried, systemPromptFrom(ctrl.History()))
		ctrl.Resume(&agent.Session{Messages: carried}, path)
	} else if path != "" {
		ctrl.SetSessionPath(path)
	}
	return nil
}

func systemPromptFrom(messages []provider.Message) string {
	for _, m := range messages {
		if m.Role == provider.RoleSystem {
			return m.Content
		}
	}
	return ""
}

func withFreshSystemPrompt(messages []provider.Message, system string) []provider.Message {
	if strings.TrimSpace(system) == "" {
		return messages
	}
	out := append([]provider.Message(nil), messages...)
	for i := range out {
		if out[i].Role == provider.RoleSystem {
			out[i].Content = system
			out[i].ReasoningContent = ""
			out[i].ReasoningSignature = ""
			out[i].ToolCalls = nil
			out[i].ToolCallID = ""
			out[i].Name = ""
			return out
		}
	}
	return append([]provider.Message{{Role: provider.RoleSystem, Content: system}}, out...)
}

// SetDefaultModel sets the config default and switches the live model to it.
func (a *App) SetDefaultModel(ref string) error {
	tab := a.activeTab()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	prev := tab.model
	tab.model = ref
	if err := a.applyConfigChange(func(c *config.Config) error {
		if _, ok := c.ResolveModel(ref); !ok {
			return fmt.Errorf("unknown model %q", ref)
		}
		c.DefaultModel = ref
		return nil
	}); err != nil {
		tab.model = prev
		return err
	}
	return nil
}

// SetPlannerModel sets (or, with "", clears) the two-model planner.
func (a *App) SetPlannerModel(ref string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		if ref != "" {
			if _, ok := c.ResolveModel(ref); !ok {
				return fmt.Errorf("unknown planner model %q", ref)
			}
		}
		c.Agent.PlannerModel = ref
		return nil
	})
}

// SetAutoPlan updates the automatic plan-mode gate (off|on).
func (a *App) SetAutoPlan(mode string) error {
	return a.applyConfigChange(func(c *config.Config) error { return c.SetAutoPlan(mode) })
}

func desktopAutoPlanMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "on", "ask":
		return "on"
	default:
		return "off"
	}
}

// SyncProviderModels fetches the live model list from the provider API and
// updates the provider entry in config.
func (a *App) SyncProviderModels(providerName string) (ProviderModelsResult, error) {
	name := strings.TrimSpace(providerName)
	if name == "" {
		return ProviderModelsResult{}, fmt.Errorf("provider name is required")
	}
	var out ProviderModelsResult
	err := a.applyConfigChange(func(c *config.Config) error {
		e, ok := c.Provider(name)
		if !ok {
			return fmt.Errorf("provider %q not found", name)
		}
		models, err := e.FetchModels(context.Background())
		if err != nil {
			return err
		}
		if len(models) == 0 {
			return fmt.Errorf("empty model list returned")
		}
		stored := config.ModelsForProviderStorage(e, models)
		if len(stored) == 0 {
			return fmt.Errorf("empty model list returned")
		}
		out = ProviderModelsResult{Provider: name, Models: stored}
		for i := range c.Providers {
			if c.Providers[i].APIKeyEnv != e.APIKeyEnv {
				continue
			}
			entry := config.ModelsForProviderStorage(&c.Providers[i], models)
			if len(entry) == 0 {
				continue
			}
			c.Providers[i].Models = append([]string(nil), entry...)
			c.Providers[i].Model = entry[0]
			if !c.Providers[i].HasModel(c.Providers[i].Default) {
				c.Providers[i].Default = entry[0]
			}
		}
		return nil
	})
	if err == nil && a.ctx != nil {
		runtime.EventsEmit(a.ctx, "agent:models-refreshed", map[string]any{
			"provider": name,
			"ok":       true,
		})
	}
	return out, err
}

// SaveProvider adds or updates a provider. A single model fills `model`; several
// fill `models` (with `default`). The shared key/endpoint live on the entry.
func (a *App) SaveProvider(p ProviderView) error {
	err := a.applyConfigChange(func(c *config.Config) error {
		e := config.ProviderEntry{
			Name: p.Name, Kind: p.Kind, BaseURL: p.BaseURL,
			APIKeyEnv: p.APIKeyEnv, BalanceURL: strings.TrimSpace(p.BalanceURL), ContextWindow: p.ContextWindow,
			SupportedEfforts: p.SupportedEfforts,
			DefaultEffort:    p.DefaultEffort,
		}
		if len(p.Models) > 0 {
			e.Model = p.Models[0] // also satisfies validateProvider's model requirement
			if len(p.Models) > 1 {
				e.Models = p.Models
				e.Default = p.Default
			}
		}
		config.ApplyDeepSeekProviderEndpoints(&e)
		if err := c.UpsertProvider(e); err != nil {
			return err
		}
		if e.APIKeyEnv == onboardingKeyEnv {
			c.SyncDeepSeekEndpoints(e.BaseURL)
		}
		return nil
	})
	if err != nil {
		return err
	}
	if strings.TrimSpace(p.APIKeyEnv) == onboardingKeyEnv {
		go a.refreshProviderModelsAsync(onboardingKeyEnv)
	}
	return nil
}

// DeleteProvider removes a provider (refused for the current default_model).
func (a *App) DeleteProvider(name string) error {
	if !a.confirmSensitiveConfig("Delete provider?", name) {
		return fmt.Errorf("cancelled")
	}
	return a.applyConfigChange(func(c *config.Config) error { return c.RemoveProvider(name) })
}

// SetProviderKey writes a secret to the global credentials file under the given
// env-var name (the one a provider's api_key_env points at) and rebuilds so it
// resolves immediately.
func (a *App) SetProviderKey(apiKeyEnv, value string) error {
	if !a.confirmSetProviderKey(apiKeyEnv) {
		return fmt.Errorf("cancelled")
	}
	if strings.TrimSpace(apiKeyEnv) == "" {
		return fmt.Errorf("this provider has no api_key_env set")
	}
	value = apikey.Normalize(value)
	if err := upsertDotEnv(apiKeyEnv, value); err != nil {
		return err
	}
	if err := a.rebuild(); err != nil {
		return err
	}
	go a.refreshProviderModelsAsync(apiKeyEnv)
	return nil
}

// SetPermissionMode sets the writer-fallback mode (ask|allow|deny).
func (a *App) SetPermissionMode(mode string) error {
	if !a.confirmSensitiveConfig("Change permission mode?", "New mode: "+strings.TrimSpace(mode)) {
		return fmt.Errorf("cancelled")
	}
	return a.applyConfigChange(func(c *config.Config) error { return c.SetPermissionMode(mode) })
}

// AddPermissionRule appends a rule to the allow/ask/deny list.
func (a *App) AddPermissionRule(list, rule string) error {
	return a.applyConfigChange(func(c *config.Config) error { return c.AddPermissionRule(list, rule) })
}

// RemovePermissionRule drops a rule from the allow/ask/deny list.
func (a *App) RemovePermissionRule(list, rule string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		_, err := c.RemovePermissionRule(list, rule)
		return err
	})
}

// SetSandbox updates the bash sandbox mode, network egress, and write roots.
func (a *App) SetSandbox(bash string, network bool, workspaceRoot string, allowWrite []string) error {
	detail := "bash=" + strings.TrimSpace(bash)
	if network {
		detail += "; network egress enabled"
	}
	if !a.confirmSensitiveConfig("Change sandbox settings?", detail) {
		return fmt.Errorf("cancelled")
	}
	return a.applyConfigChange(func(c *config.Config) error {
		c.Sandbox.Bash = bash
		c.Sandbox.Network = network
		c.Sandbox.WorkspaceRoot = strings.TrimSpace(workspaceRoot)
		c.Sandbox.AllowWrite = trimList(allowWrite)
		return nil
	})
}

// SetNetwork updates ordinary outbound proxy settings.
func (a *App) SetNetwork(n NetworkView) error {
	return a.applyConfigChange(func(c *config.Config) error {
		return c.SetNetwork(config.NetworkConfig{
			ProxyMode: n.ProxyMode,
			ProxyURL:  n.ProxyURL,
			NoProxy:   n.NoProxy,
			Proxy: config.NetworkProxyConfig{
				Type:     n.Proxy.Type,
				Server:   n.Proxy.Server,
				Port:     n.Proxy.Port,
				Username: n.Proxy.Username,
				Password: n.Proxy.Password,
			},
		})
	})
}

// SetCloseBehavior updates desktop-only window close behavior without rebuilding
// the active controller. It must stay out of provider-visible prompt/request data.
func (a *App) SetCloseBehavior(mode string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopCloseBehavior(mode) })
}

// SetDesktopLanguage updates only the desktop UI language. It deliberately does
// not touch config.language, which the CLI/model-facing runtime uses.
func (a *App) SetDesktopLanguage(lang string) error {
	if err := a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopLanguage(lang) }); err != nil {
		return err
	}
	a.updateTrayLocale(lang)
	return nil
}

// SetDesktopTerminalShell updates the integrated terminal shell preference.
func (a *App) SetDesktopTerminalShell(shell string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopTerminalShell(shell) })
}

// SetDesktopGitSettings updates desktop Git UI preferences.
func (a *App) SetDesktopGitSettings(git DesktopGitView) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		return c.SetDesktopGitSettings(
			git.PRMergeMethod,
			git.CheckGitHubCli,
			git.SyncRepoMergeToGitHub,
			git.CommitInstructions,
			git.PRInstructions,
		)
	})
}

// SetTrayLocale mirrors the resolved desktop UI language into the native tray
// menu. It is runtime-only; the persisted preference remains [desktop].language.
func (a *App) SetTrayLocale(locale string) error {
	if locale != "zh" {
		locale = "en"
	}
	a.updateTrayLocale(locale)
	return nil
}

// SetDesktopAppearancePrefs updates desktop surface and typography preferences.
func (a *App) SetDesktopAppearancePrefs(in DesktopAppearanceView) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		return c.SetDesktopAppearancePrefs(config.DesktopAppearanceInput{
			BackgroundPreset: in.BackgroundPreset,
			ForegroundPreset: in.ForegroundPreset,
			TextSize:         in.TextSize,
			CodeFontSize:     in.CodeFontSize,
			DiffMarker:       in.DiffMarker,
		})
	})
}

// SetDesktopCodeReviewSettings updates desktop code-review panel defaults.
func (a *App) SetDesktopCodeReviewSettings(in DesktopCodeReviewView) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		return c.SetDesktopCodeReviewSettings(in.DefaultScope, in.SecurityByDefault)
	})
}

// MigrateDesktopLocalPrefs imports browser-local desktop prefs into config once.
func (a *App) MigrateDesktopLocalPrefs(in DesktopLocalPrefsMigrationInput) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		return c.ImportDesktopLocalPrefs(
			config.DesktopAppearanceInput{
				BackgroundPreset: in.BackgroundPreset,
				ForegroundPreset: in.ForegroundPreset,
				TextSize:         in.TextSize,
				CodeFontSize:     in.CodeFontSize,
				DiffMarker:       in.DiffMarker,
			},
			in.CodeReviewScope,
			in.CodeReviewSecurity,
			in.HasAppearance,
			in.HasCodeReview,
		)
	})
}

// SetDesktopAppearance updates only desktop theme preferences. It does not
// rebuild the active controller and must stay out of provider-visible requests.
func (a *App) SetDesktopAppearance(theme, style string) error {
	return a.applyConfigOnly(func(c *config.Config) error { return c.SetDesktopAppearance(theme, style) })
}

// MigrateDesktopPreferences imports old browser-local desktop preferences into
// the user config once. Existing [desktop] values win so stale localStorage never
// overwrites an explicit config edit.
func (a *App) MigrateDesktopPreferences(language, theme, style string) error {
	return a.applyConfigOnly(func(c *config.Config) error {
		if strings.TrimSpace(c.Desktop.Language) == "" {
			if err := c.SetDesktopLanguage(language); err != nil {
				return err
			}
		}
		if strings.TrimSpace(c.Desktop.Theme) == "" && strings.TrimSpace(c.Desktop.ThemeStyle) == "" {
			if err := c.SetDesktopAppearance(theme, style); err != nil {
				return err
			}
		}
		return nil
	})
}

// SetAgentSettings updates the full [agent] section from the desktop settings panel.
func (a *App) SetAgentSettings(in AgentSettingsInput) error {
	return a.applyConfigChange(func(c *config.Config) error {
		return c.ApplyAgentSettings(agentSettingsFromView(in))
	})
}

// ListOutputStyles returns built-in and discovered custom output styles.
func (a *App) ListOutputStyles() []OutputStyleView {
	styles := outputstyle.List(outputstyle.Dirs())
	out := make([]OutputStyleView, 0, len(styles))
	for _, st := range styles {
		out = append(out, OutputStyleView{
			Name:        st.Name,
			Description: st.Description,
			Builtin:     st.Builtin,
		})
	}
	return out
}

// SetAgentParams updates sampling temperature, the optional max-steps guard, and
// the base system prompt without changing other agent fields.
func (a *App) SetAgentParams(temperature float64, maxSteps int, systemPrompt string) error {
	return a.applyConfigChange(func(c *config.Config) error {
		c.Agent.Temperature = temperature
		c.Agent.MaxSteps = maxSteps
		c.Agent.SystemPrompt = strings.TrimSpace(systemPrompt)
		return nil
	})
}

// trimList drops blank entries from a string slice (and returns a non-nil slice).
func trimList(in []string) []string {
	out := []string{}
	for _, s := range in {
		if t := strings.TrimSpace(s); t != "" {
			out = append(out, t)
		}
	}
	return out
}
