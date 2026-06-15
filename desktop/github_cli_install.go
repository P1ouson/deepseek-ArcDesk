package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// GitHubCLIInstallResult is returned by InstallGitHubCLI.
type GitHubCLIInstallResult struct {
	OK          bool   `json:"ok"`
	Detail      string `json:"detail"`
	Method      string `json:"method,omitempty"`
	NeedRestart bool   `json:"needRestart,omitempty"`
}

const (
	ghInstallTimeout = 5 * time.Minute
	ghDefaultWinPath = `C:\Program Files\GitHub CLI\gh.exe`
)

// InstallGitHubCLI installs GitHub CLI on Windows (winget → choco → MSI download).
func (a *App) InstallGitHubCLI() GitHubCLIInstallResult {
	if runtime.GOOS != "windows" {
		return GitHubCLIInstallResult{OK: false, Detail: "unsupported_platform"}
	}
	ctx, cancel := context.WithTimeout(context.Background(), ghInstallTimeout)
	defer cancel()

	if ghInstalled(ctx) {
		return GitHubCLIInstallResult{OK: true, Detail: "already_installed", Method: "detected"}
	}

	type step struct {
		method string
		run    func(context.Context) (string, error)
	}
	steps := []step{
		{method: "winget", run: installGhWinget},
		{method: "choco", run: installGhChoco},
		{method: "msi", run: installGhMSI},
	}

	var lastOut string
	var lastErr error
	for _, s := range steps {
		out, err := s.run(ctx)
		lastOut, lastErr = out, err
		if ghInstalled(ctx) {
			return GitHubCLIInstallResult{OK: true, Detail: trimInstallDetail(out), Method: s.method}
		}
		if installLooksSuccessful(out, err) {
			return GitHubCLIInstallResult{
				OK:          true,
				Detail:      trimInstallDetail(out),
				Method:      s.method,
				NeedRestart: true,
			}
		}
	}

	detail := "install_failed"
	if lastErr != nil {
		detail = lastErr.Error()
	} else if strings.TrimSpace(lastOut) != "" {
		detail = trimInstallDetail(lastOut)
	}
	return GitHubCLIInstallResult{OK: false, Detail: detail}
}

func trimInstallDetail(out string) string {
	const max = 400
	out = strings.TrimSpace(out)
	if len(out) <= max {
		return out
	}
	return out[:max] + "…"
}

func ghInstalled(ctx context.Context) bool {
	for _, command := range []string{
		`gh --version`,
		fmt.Sprintf(`"%s" --version`, ghDefaultWinPath),
	} {
		out, err := runInstallShell(ctx, command)
		if err == nil && strings.Contains(strings.ToLower(out), "gh version") {
			return true
		}
	}
	return false
}

func installLooksSuccessful(out string, err error) bool {
	joined := strings.ToLower(out)
	if err != nil {
		joined += " " + strings.ToLower(err.Error())
	}
	for _, marker := range []string{
		"successfully installed",
		"installation successful",
		"已成功安装",
		"successfully verified",
		"found an existing package already installed",
		"found existing installed package",
		"no applicable update found",
		"exit code: 0",
	} {
		if strings.Contains(joined, marker) {
			return true
		}
	}
	return false
}

func installGhWinget(ctx context.Context) (string, error) {
	winget := resolveWingetPath()
	if winget == "" {
		return "", fmt.Errorf("winget_not_found")
	}
	cmd := fmt.Sprintf(
		`"%s" install --id GitHub.cli -e --accept-package-agreements --accept-source-agreements --disable-interactivity`,
		winget,
	)
	return runInstallShell(ctx, cmd)
}

func installGhChoco(ctx context.Context) (string, error) {
	if _, err := exec.LookPath("choco"); err != nil {
		return "", fmt.Errorf("choco_not_found")
	}
	return runInstallShell(ctx, `choco install gh -y`)
}

func installGhMSI(ctx context.Context) (string, error) {
	script := `
$ErrorActionPreference = 'Stop'
$release = Invoke-RestMethod -Uri 'https://api.github.com/repos/cli/cli/releases/latest' -Headers @{ 'User-Agent' = 'ArcDesk' }
$asset = $release.assets | Where-Object { $_.name -match 'windows_amd64\.msi$' } | Select-Object -First 1
if (-not $asset) { throw 'no_windows_msi_asset' }
$dest = Join-Path $env:TEMP $asset.name
Invoke-WebRequest -Uri $asset.browser_download_url -OutFile $dest -UseBasicParsing
$p = Start-Process -FilePath 'msiexec.exe' -ArgumentList @('/i', $dest, '/passive', 'ADDLOCAL=Core,CliTools') -Wait -PassThru
if ($p.ExitCode -ne 0) { throw ('msi_exit_' + $p.ExitCode) }
Write-Output ('installed_msi ' + $asset.name)
`
	return runInstallShell(ctx, script)
}

func resolveWingetPath() string {
	if p, err := exec.LookPath("winget"); err == nil && fileExists(p) {
		return p
	}
	if local := os.Getenv("LOCALAPPDATA"); local != "" {
		p := filepath.Join(local, "Microsoft", "WindowsApps", "winget.exe")
		if fileExists(p) {
			return p
		}
	}
	for _, env := range []string{"ProgramFiles", "ProgramW6432"} {
		if root := os.Getenv(env); root != "" {
			p := filepath.Join(root, "WindowsApps", "winget.exe")
			if fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

func runInstallShell(ctx context.Context, command string) (string, error) {
	out, err := runDetachedShell(ctx, command, os.Getenv("USERPROFILE"))
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("install_timeout")
	}
	return out, err
}
