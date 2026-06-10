package main

import (
	"context"
	"fmt"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/hook"
	"arcdesk/internal/skill"
)

// UntrustedMCPServerView describes a quarantined repo-local MCP server.
type UntrustedMCPServerView struct {
	Name        string `json:"name"`
	Source      string `json:"source"`
	CommandLine string `json:"commandLine"`
	ConfigPath  string `json:"configPath"`
}

// ListUntrustedMCPServers returns repo-local .mcp.json servers blocked for the active workspace.
func (a *App) ListUntrustedMCPServers() []UntrustedMCPServerView {
	root := strings.TrimSpace(a.activeWorkspaceRoot())
	if root == "" {
		return nil
	}
	cfg, err := config.LoadForRoot(root)
	if err != nil {
		return nil
	}
	path := config.MCPJSONPathForRoot(root)
	var out []UntrustedMCPServerView
	for _, p := range cfg.Plugins {
		if p.Source != "mcpjson" {
			continue
		}
		if hook.IsMCPServerTrusted(root, p.Name, "") {
			continue
		}
		cmd := hook.FormatMCPCommandLine(p.Command, p.Args)
		if strings.TrimSpace(p.URL) != "" {
			cmd = p.URL
		}
		out = append(out, UntrustedMCPServerView{
			Name:        p.Name,
			Source:      p.Source,
			CommandLine: cmd,
			ConfigPath:  path,
		})
	}
	return out
}

// TrustProjectMCPServer marks one repo-local MCP server trusted and rebuilds the active tab.
func (a *App) TrustProjectMCPServer(name string) error {
	root := strings.TrimSpace(a.activeWorkspaceRoot())
	name = strings.TrimSpace(name)
	if root == "" || name == "" {
		return fmt.Errorf("workspace and server name are required")
	}
	if !a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Trust MCP server?",
		Message:      fmt.Sprintf("Allow %q from this project's .mcp.json to start subprocesses or connect remotely?", name),
		Detail:       trustMCPDetail(root, name),
		ConfirmLabel: "Trust server",
		CancelLabel:  "Cancel",
		Destructive:  true,
	}) {
		return fmt.Errorf("cancelled")
	}
	if err := hook.TrustMCPServer(root, name, ""); err != nil {
		return err
	}
	return a.rebuildActiveTabController()
}

// TrustProjectSkills marks the active workspace trusted for repo-local hooks and skills.
func (a *App) TrustProjectSkills() error {
	root := strings.TrimSpace(a.activeWorkspaceRoot())
	if root == "" {
		return fmt.Errorf("no active workspace")
	}
	if !a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Trust project skills and hooks?",
		Message:      "Enable repo-local skills and shell hooks for this workspace.",
		Detail:       root,
		ConfirmLabel: "Trust project",
		CancelLabel:  "Cancel",
		Destructive:  true,
	}) {
		return fmt.Errorf("cancelled")
	}
	if err := hook.Trust(root, ""); err != nil {
		return err
	}
	return a.rebuildActiveTabController()
}

func trustMCPDetail(root, name string) string {
	cfg, err := config.LoadForRoot(root)
	if err != nil {
		return config.MCPJSONPathForRoot(root)
	}
	for _, p := range cfg.Plugins {
		if p.Name == name {
			cmd := hook.FormatMCPCommandLine(p.Command, p.Args)
			if strings.TrimSpace(p.URL) != "" {
				cmd = p.URL
			}
			return config.MCPJSONPathForRoot(root) + "\n" + cmd
		}
	}
	return config.MCPJSONPathForRoot(root)
}

func (a *App) rebuildActiveTabController() error {
	a.mu.RLock()
	tab := a.activeTabLocked()
	a.mu.RUnlock()
	if tab == nil {
		return fmt.Errorf("no active tab")
	}
	go a.buildTabController(tab)
	return nil
}

func (a *App) desktopSkillInstallGuard() skill.InstallGuard {
	return func(_ context.Context, req skill.InstallRequest) (bool, error) {
		preview := strings.TrimSpace(req.Body)
		if len(preview) > 400 {
			preview = preview[:397] + "…"
		}
		scope := string(req.Scope)
		ok := a.requireNativeConfirm(NativeConfirmRequest{
			Title:        "Install skill?",
			Message:      fmt.Sprintf("Allow install_skill to write %q (%s scope)?", req.Name, scope),
			Detail:       preview,
			ConfirmLabel: "Install",
			CancelLabel:  "Cancel",
			Destructive:  true,
		})
		if !ok {
			return false, nil
		}
		return true, nil
	}
}
