package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"arcdesk/internal/proc"
	"arcdesk/internal/sandbox"
)

// isDetachedGhShellCommand reports whether command is a read-only gh probe suitable
// for detached execution when no agent session is active (Git panel / settings).
func isDetachedGhShellCommand(command string) bool {
	cmd := strings.TrimSpace(strings.ToLower(command))
	if strings.HasPrefix(cmd, "gh ") {
		return true
	}
	return strings.HasPrefix(cmd, `"c:\program files\github cli\gh.exe"`)
}

// isDetachedGitInitCommand reports git init invocations safe to run without an
// active agent tab (sidebar "Create Git repository").
func isDetachedGitInitCommand(command string) bool {
	cmd := strings.TrimSpace(command)
	if strings.EqualFold(cmd, "git init") {
		return true
	}
	lower := strings.ToLower(cmd)
	if !strings.HasPrefix(lower, "git ") || !strings.HasSuffix(strings.TrimSpace(lower), " init") {
		return false
	}
	// Allow only `git -C <path> init` (path may be quoted).
	return strings.Contains(lower, " -c ")
}

func runDetachedShell(ctx context.Context, command, workDir string) (string, error) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", fmt.Errorf("empty_command")
	}
	sh := sandbox.ResolveShell()
	argv, _ := sandbox.Command(sandbox.Spec{}, sh, command)
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	proc.HideWindow(cmd)
	dir := strings.TrimSpace(workDir)
	if dir == "" {
		dir = os.Getenv("USERPROFILE")
	}
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if ctx.Err() == context.DeadlineExceeded {
		return out, fmt.Errorf("shell_timeout")
	}
	if err != nil {
		return out, fmt.Errorf("%w: %s", err, strings.TrimSpace(out))
	}
	return out, nil
}
