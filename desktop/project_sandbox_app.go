package main

import (
	"os"
	"path/filepath"
	"strings"

	"reasonix/internal/config"
)

func (a *App) activeTabMode() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if tab := a.activeTabLocked(); tab != nil {
		return tab.mode
	}
	return "normal"
}

func isYoloMode(mode string) bool {
	return normalizeTabMode(mode) == "yolo"
}

// ProjectSandboxStatus returns the current project's sandbox profile and whether
// YOLO currently requires completing setup first.
func (a *App) ProjectSandboxStatus() ProjectSandboxStatus {
	root := a.activeWorkspaceRoot()
	return projectSandboxStatus(root, isYoloMode(a.activeTabMode()))
}

// ConfigureProjectSandbox persists the wizard output for the active workspace and
// mirrors bash/network/write roots into the project reasonix.toml [sandbox].
func (a *App) ConfigureProjectSandbox(in ConfigureProjectSandboxInput) error {
	root := a.activeWorkspaceRoot()
	profile, err := mergeSandboxInput(root, in)
	if err != nil {
		return err
	}
	if err := saveProjectSandboxProfile(root, profile); err != nil {
		return err
	}
	return a.syncProjectSandboxSection(profile)
}

func (a *App) syncProjectSandboxSection(p ProjectSandboxProfile) error {
	path := projectConfigPathForRoot(a.activeWorkspaceRoot())
	userPath := config.UserConfigPath()
	if path == "" || sameConfigPath(path, userPath) {
		return nil
	}
	cfg := config.LoadForEdit(path)
	cfg.Sandbox.WorkspaceRoot = strings.TrimSpace(p.WorkspaceRoot)
	cfg.Sandbox.AllowWrite = append([]string(nil), p.AllowWrite...)
	cfg.Sandbox.Bash = p.Bash
	cfg.Sandbox.Network = p.Network
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}
	return cfg.SaveTo(path)
}

// ValidatePreviewURL applies project sandbox policy to a Web preview URL.
func (a *App) ValidatePreviewURL(raw string) PreviewURLValidation {
	root := a.activeWorkspaceRoot()
	profile, _ := loadProjectSandboxProfile(root)
	strict := isYoloMode(a.activeTabMode()) && profile.Configured
	return validatePreviewURL(raw, profile, strict)
}

func (a *App) projectSandboxConfigured(root string) bool {
	p, err := loadProjectSandboxProfile(root)
	return err == nil && p.Configured
}
