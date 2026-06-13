package envaware

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// Snapshot is a point-in-time view of the execution environment.
type Snapshot struct {
	OS             string            `json:"os"`
	Arch           string            `json:"arch"`
	Shell          string            `json:"shell,omitempty"`
	CI             bool              `json:"ci"`
	Workspace      string            `json:"workspace,omitempty"`
	GoVersion      string            `json:"go_version,omitempty"`
	NodeVersion    string            `json:"node_version,omitempty"`
	NpmVersion     string            `json:"npm_version,omitempty"`
	PnpmVersion    string            `json:"pnpm_version,omitempty"`
	GitVersion     string            `json:"git_version,omitempty"`
	GhAvailable    bool              `json:"gh_available"`
	GhVersion      string            `json:"gh_version,omitempty"`
	WailsVersion   string            `json:"wails_version,omitempty"`
	PlatformNotes  []string          `json:"platform_notes,omitempty"`
	ToolchainHints map[string]string `json:"toolchain_hints,omitempty"`
}

// Probe collects environment facts for workspace root.
func Probe(ctx context.Context, workspace string) Snapshot {
	if ctx == nil {
		ctx = context.Background()
	}
	snap := Snapshot{
		OS:        runtime.GOOS,
		Arch:      runtime.GOARCH,
		Shell:     os.Getenv("SHELL"),
		CI:        os.Getenv("CI") != "" || os.Getenv("GITHUB_ACTIONS") != "",
		Workspace: strings.TrimSpace(workspace),
	}
	if snap.Shell == "" {
		snap.Shell = os.Getenv("COMSPEC")
	}
	snap.GoVersion = runVersion(ctx, "go", "version")
	snap.NodeVersion = runVersion(ctx, "node", "--version")
	snap.NpmVersion = runVersion(ctx, "npm", "--version")
	snap.PnpmVersion = runVersion(ctx, "pnpm", "--version")
	snap.GitVersion = runVersion(ctx, "git", "--version")
	if gh := runVersion(ctx, "gh", "--version"); gh != "" {
		snap.GhAvailable = true
		snap.GhVersion = gh
	}
	if looksLikeWails(workspace) {
		snap.WailsVersion = runVersion(ctx, "wails", "version")
	}
	snap.PlatformNotes = platformNotes(snap)
	snap.ToolchainHints = toolchainHints(snap)
	return snap
}

func looksLikeWails(workspace string) bool {
	workspace = strings.TrimSpace(workspace)
	if workspace == "" {
		return false
	}
	for _, name := range []string{"wails.json", filepath.Join("desktop", "wails.json")} {
		if _, err := os.Stat(filepath.Join(workspace, name)); err == nil {
			return true
		}
	}
	b, err := os.ReadFile(filepath.Join(workspace, "go.mod"))
	if err != nil {
		return false
	}
	return strings.Contains(string(b), "wails.io")
}

func runVersion(ctx context.Context, name string, args ...string) string {
	if _, err := exec.LookPath(name); err != nil {
		return ""
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(cctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	line := strings.TrimSpace(strings.Split(string(out), "\n")[0])
	if len(line) > 120 {
		line = line[:117] + "..."
	}
	return line
}

func platformNotes(s Snapshot) []string {
	var notes []string
	switch s.OS {
	case "windows":
		notes = append(notes, "Prefer PowerShell separators (;) over && in shell examples unless using bash explicitly.")
		if s.Shell != "" && strings.Contains(strings.ToLower(s.Shell), "powershell") {
			notes = append(notes, "Default shell appears to be PowerShell.")
		}
	case "darwin":
		notes = append(notes, "macOS: case-sensitive paths depend on volume format; prefer exact repo-relative paths.")
	case "linux":
		notes = append(notes, "Linux: watch for case-sensitive filesystem paths.")
	}
	if s.GoVersion == "" {
		notes = append(notes, "Go toolchain not found on PATH.")
	}
	if s.NodeVersion == "" {
		notes = append(notes, "Node.js not found on PATH.")
	}
	return notes
}

func toolchainHints(s Snapshot) map[string]string {
	h := map[string]string{}
	if s.GoVersion != "" {
		h["go"] = s.GoVersion
	}
	if s.NodeVersion != "" {
		h["node"] = s.NodeVersion
	}
	if s.PnpmVersion != "" {
		h["pnpm"] = s.PnpmVersion
	} else if s.NpmVersion != "" {
		h["npm"] = s.NpmVersion
	}
	if s.WailsVersion != "" {
		h["wails"] = s.WailsVersion
	}
	return h
}

// ComposeBlock returns a short markdown block for the system prompt.
func ComposeBlock(s Snapshot) string {
	var b strings.Builder
	b.WriteString("## Environment (host)\n")
	b.WriteString("- OS: ")
	b.WriteString(s.OS)
	b.WriteString("/")
	b.WriteString(s.Arch)
	if s.GoVersion != "" {
		b.WriteString("\n- ")
		b.WriteString(s.GoVersion)
	}
	if s.NodeVersion != "" {
		b.WriteString("\n- node ")
		b.WriteString(strings.TrimPrefix(s.NodeVersion, "v"))
	}
	if s.WailsVersion != "" {
		b.WriteString("\n- ")
		b.WriteString(s.WailsVersion)
	}
	if s.GhAvailable {
		b.WriteString("\n- gh available")
	}
	for _, n := range s.PlatformNotes {
		b.WriteString("\n- Note: ")
		b.WriteString(n)
	}
	return b.String()
}
