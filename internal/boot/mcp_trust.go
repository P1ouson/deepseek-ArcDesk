package boot

import (
	"fmt"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/hook"
)

// filterTrustedMCPPlugins drops repo-local .mcp.json servers until the user trusts
// them for the workspace. Explicit ARCDESK.toml [[plugins]] and legacy home config
// entries are treated as manually configured and pass through unchanged.
func filterTrustedMCPPlugins(root string, plugins []config.PluginEntry, homeDir string) (trusted, blocked []config.PluginEntry) {
	for _, p := range plugins {
		if p.Source == "mcpjson" && !hook.IsMCPServerTrusted(root, p.Name, homeDir) {
			blocked = append(blocked, p)
			continue
		}
		trusted = append(trusted, p)
	}
	return trusted, blocked
}

func mcpTrustNotice(entry config.PluginEntry, mcpPath string) string {
	cmd := hook.FormatMCPCommandLine(entry.Command, entry.Args)
	if strings.TrimSpace(entry.URL) != "" {
		cmd = entry.URL
	}
	return fmt.Sprintf("mcp server %q from %s is quarantined until trusted — %s. Trust it from desktop settings or TrustProjectMCPServer.", entry.Name, mcpPath, cmd)
}

func autoStartPlugins(plugins []config.PluginEntry) []config.PluginEntry {
	out := make([]config.PluginEntry, 0, len(plugins))
	for _, p := range plugins {
		if p.ShouldAutoStart() {
			out = append(out, p)
		}
	}
	return out
}
