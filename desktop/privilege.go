package main

import (
	"regexp"
	"strings"
)

// nativeConfirmHook is set by tests to bypass Wails dialogs.
var nativeConfirmHook func(NativeConfirmRequest) (bool, error)

func (a *App) requireNativeConfirm(req NativeConfirmRequest) bool {
	if nativeConfirmHook != nil {
		ok, _ := nativeConfirmHook(req)
		return ok
	}
	ok, err := a.ConfirmAction(req)
	return err == nil && ok
}

func (a *App) confirmYOLO(on bool) bool {
	if !on {
		return true
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Enable YOLO mode?",
		Message:      "YOLO auto-approves every tool call for this session.",
		Detail:       "Deny rules still apply. A compromised page must not enable this silently — confirm only if you intend to.",
		ConfirmLabel: "Enable YOLO",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmRunShell(command string) bool {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Run shell command?",
		Message:      "ArcDesk is about to execute a shell command outside the agent approval flow.",
		Detail:       cmd,
		ConfirmLabel: "Run",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

var riskyShellPattern = regexp.MustCompile(`(?i)(^|[\s;&|])(rm\s+-rf|rmdir\s+/s|del\s+/[fq]|format\s+|mkfs|shutdown|reboot|reg\s+(add|delete)|curl\s+.*\|\s*(ba)?sh|powershell(\.exe)?\s+-(?:enc|e\s)|cmd(\.exe)?\s+/c\s+.*(del|rmdir|format))`)

func (a *App) confirmRunShellQuiet(command string) bool {
	cmd := strings.TrimSpace(command)
	if cmd == "" {
		return false
	}
	if !riskyShellPattern.MatchString(cmd) {
		return true
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Run potentially destructive command?",
		Message:      "This quiet shell command matches a high-risk pattern.",
		Detail:       cmd,
		ConfirmLabel: "Run anyway",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmSetProviderKey(apiKeyEnv string) bool {
	env := strings.TrimSpace(apiKeyEnv)
	if env == "" {
		return false
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Update provider credential?",
		Message:      "A web page is requesting a change to stored API credentials.",
		Detail:       "Environment variable: " + env,
		ConfirmLabel: "Update",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmConnectKey() bool {
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Save API key?",
		Message:      "Store a new provider API key in the global credentials file?",
		ConfirmLabel: "Save",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmDeleteWriteFile(path string) bool {
	p := strings.TrimSpace(path)
	if p == "" {
		return false
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Delete file?",
		Message:      "Permanently delete this workspace file?",
		Detail:       p,
		ConfirmLabel: "Delete",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmSensitiveConfig(action, detail string) bool {
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        action,
		Message:      "Confirm this settings change initiated from the desktop UI.",
		Detail:       detail,
		ConfirmLabel: "Apply",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmStartMobileTunnel() bool {
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Expose remote control to the internet?",
		Message:      "This exposes a remote control endpoint to the internet.",
		Detail:       "Anyone with the tunnel URL can reach your mobile bridge until you stop the tunnel or it idles out. Pairing still required for control actions.",
		ConfirmLabel: "Start tunnel",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmAllowLAN() bool {
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Expose mobile bridge to your local network?",
		Message:      "This binds the mobile bridge to all network interfaces (0.0.0.0).",
		Detail:       "Devices on your Wi‑Fi or LAN can reach the pairing page and API on this machine until you turn this off. Pairing is still required for control actions.",
		ConfirmLabel: "Allow LAN access",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmAddMCPServer(name string) bool {
	n := strings.TrimSpace(name)
	if n == "" {
		n = "MCP server"
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Add MCP server?",
		Message:      "A page is requesting to connect a new MCP server with tool access.",
		Detail:       "Server: " + n,
		ConfirmLabel: "Add",
		CancelLabel:  "Cancel",
		Destructive:  true,
	})
}

func (a *App) confirmPromoteProjectSecrets(envKeys []string, projectRoot string) bool {
	if len(envKeys) == 0 {
		return false
	}
	detail := strings.Join(envKeys, ", ")
	if root := strings.TrimSpace(projectRoot); root != "" {
		detail = "Project: " + root + "\nKeys: " + detail
	}
	return a.requireNativeConfirm(NativeConfirmRequest{
		Title:        "Copy project API keys to global credentials?",
		Message:      "This workspace resolved provider keys from a project .env. Copy them into ArcDesk's global credentials store?",
		Detail:       detail,
		ConfirmLabel: "Copy keys",
		CancelLabel:  "Skip",
		Destructive:  true,
	})
}
