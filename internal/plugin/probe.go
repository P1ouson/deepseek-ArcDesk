package plugin

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// ProbeExecutable resolves command on PATH, including a login-shell fallback on Unix.
func ProbeExecutable(command string) (path string, ok bool) {
	command = strings.TrimSpace(command)
	if command == "" {
		return "", false
	}
	if exe, ok := lookPathInEnv(command, os.Environ()); ok {
		return exe, true
	}
	currentPath, _ := envValue(os.Environ(), "PATH")
	if shellPath := strings.TrimSpace(stdioShellPATH(context.Background())); shellPath != "" {
		fallbackPath := mergePathLists(shellPath, currentPath)
		if fallbackPath != currentPath {
			fallbackEnv := setEnvValue(os.Environ(), "PATH", fallbackPath)
			if exe, ok := lookPathInEnv(command, fallbackEnv); ok {
				return exe, true
			}
		}
	}
	return "", false
}

// NodeRuntimeProbe reports whether node/npx are available for npx-based MCP servers.
func NodeRuntimeProbe() (nodePath, npxPath, version string, ok bool) {
	nodePath, nodeOK := ProbeExecutable("node")
	if !nodeOK {
		return "", "", "", false
	}
	if out, err := exec.Command(nodePath, "--version").CombinedOutput(); err == nil {
		version = strings.TrimSpace(string(out))
	}
	npxPath, npxOK := ProbeExecutable("npx")
	if !npxOK {
		return nodePath, "", version, false
	}
	return nodePath, npxPath, version, true
}
