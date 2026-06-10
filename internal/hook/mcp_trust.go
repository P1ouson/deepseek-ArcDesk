package hook

import (
	"fmt"
	"strings"
)

// MCPServerTrustKey builds the persisted key for a project-scoped MCP server.
func MCPServerTrustKey(projectRoot, serverName string) string {
	return absRoot(projectRoot) + "\x00" + strings.TrimSpace(serverName)
}

// IsMCPServerTrusted reports whether a repo-local .mcp.json server may start.
func IsMCPServerTrusted(projectRoot, serverName, homeDir string) bool {
	if projectRoot == "" || strings.TrimSpace(serverName) == "" {
		return false
	}
	tf := readTrust(homeDir)
	if tf.MCPServers == nil {
		return false
	}
	proj := tf.MCPServers[absRoot(projectRoot)]
	return proj != nil && proj[strings.TrimSpace(serverName)]
}

// TrustMCPServer marks one repo-local MCP server as trusted for projectRoot.
func TrustMCPServer(projectRoot, serverName, homeDir string) error {
	projectRoot = strings.TrimSpace(projectRoot)
	serverName = strings.TrimSpace(serverName)
	if projectRoot == "" || serverName == "" {
		return fmt.Errorf("project and server name are required")
	}
	tf := readTrust(homeDir)
	if tf.MCPServers == nil {
		tf.MCPServers = map[string]map[string]bool{}
	}
	root := absRoot(projectRoot)
	if tf.MCPServers[root] == nil {
		tf.MCPServers[root] = map[string]bool{}
	}
	tf.MCPServers[root][serverName] = true
	return writeTrust(homeDir, tf)
}

// FormatMCPCommandLine renders a plugin entry command for trust prompts.
func FormatMCPCommandLine(command string, args []string) string {
	parts := make([]string, 0, 1+len(args))
	if c := strings.TrimSpace(command); c != "" {
		parts = append(parts, c)
	}
	for _, a := range args {
		if t := strings.TrimSpace(a); t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "(no command)"
	}
	return strings.Join(parts, " ")
}
