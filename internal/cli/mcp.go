package cli

import (
	"fmt"
	"os"
	"strings"

	"arcdesk/internal/codegraph"
	"arcdesk/internal/config"
	"arcdesk/internal/mcpcmd"
)

// mcp.go holds the MCP server-management surface shared by the `ARCDESK mcp`
// subcommand (config-only; takes effect next session) and the in-chat `/mcp add`
// / `/mcp remove` slash commands (which hot-connect via the controller). Both
// parse arguments through parseMCPAdd so the grammar is identical everywhere.

func parseMCPAdd(args []string) (config.PluginEntry, error) {
	return mcpcmd.ParseAdd(args)
}

func tokenizeArgs(s string) []string {
	return mcpcmd.TokenizeArgs(s)
}

// mcpCommand implements `ARCDESK mcp <add|remove|list>`. It edits config only
// (validate → UpsertPlugin/RemovePlugin → Save); the server connects on the next
// session start. For a live connect inside an open chat, use `/mcp add`.
func mcpCommand(args []string) int {
	if len(args) == 0 {
		mcpUsage()
		return 2
	}
	switch args[0] {
	case "list", "ls":
		return mcpList()
	case "add":
		return mcpAddCLI(args[1:])
	case "remove", "rm":
		return mcpRemoveCLI(args[1:])
	case "help", "-h", "--help":
		mcpUsage()
		return 0
	default:
		fmt.Fprintf(os.Stderr, "unknown mcp subcommand %q\n\n", args[0])
		mcpUsage()
		return 2
	}
}

func mcpList() int {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	listed := 0
	// CodeGraph is a built-in server injected by boot, not a [[plugins]] entry, so
	// report its resolved status here too. It is listed even when disabled, matching
	// the MCP manager where the user can enable it and choose a startup tier.
	codegraphMeta := fmt.Sprintf(" [auto_start=%v tier=%s]", cfg.Codegraph.Enabled, cfg.Codegraph.ResolvedTier())
	if bin, ok := codegraph.Resolve(cfg.Codegraph.Path); ok {
		fmt.Printf("%-16s (stdio, built-in)%s  %s serve --mcp\n", "codegraph", codegraphMeta, bin)
	} else {
		fmt.Printf("%-16s (built-in, not installed)%s  run `ARCDESK codegraph install`", "codegraph", codegraphMeta)
		if cfg.Codegraph.Enabled && cfg.Codegraph.AutoInstall {
			fmt.Print(" (or let auto_install fetch it on next startup)")
		}
		fmt.Println()
	}
	listed++
	for _, p := range cfg.Plugins {
		typ := p.Type
		if typ == "" {
			typ = "stdio"
		}
		auto := ""
		if !p.ShouldAutoStart() {
			auto = " [auto_start=false]"
		}
		if typ == "stdio" {
			line := strings.TrimSpace(p.Command + " " + strings.Join(p.Args, " "))
			fmt.Printf("%-16s (stdio)%s  %s\n", p.Name, auto, line)
		} else {
			fmt.Printf("%-16s (%s)%s  %s\n", p.Name, typ, auto, p.URL)
		}
		listed++
	}
	if listed == 0 {
		fmt.Println("no MCP servers configured")
	}
	return 0
}

func mcpAddCLI(args []string) int {
	entry, err := parseMCPAdd(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 2
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := cfg.UpsertPlugin(entry); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("added MCP server %q — loads on the next session (or run `/mcp add` inside chat to connect it live now)\n", entry.Name)
	return 0
}

func mcpRemoveCLI(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: ARCDESK mcp remove <name>")
		return 2
	}
	name := args[0]
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if !cfg.RemovePlugin(name) {
		fmt.Fprintf(os.Stderr, "no MCP server named %q in config\n", name)
		return 1
	}
	if err := cfg.Save(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Printf("removed MCP server %q\n", name)
	return 0
}

func mcpUsage() {
	fmt.Println(`Manage MCP servers (persisted to ARCDESK.toml).

Usage:
  ARCDESK mcp list
  ARCDESK mcp add <name> <command> [args...]        stdio server
  ARCDESK mcp add <name> --http <url> [--header K=V] remote (Streamable HTTP)
  ARCDESK mcp add <name> --sse  <url>               remote (legacy SSE)
  ARCDESK mcp remove <name>

Flags for add:
  --http <url> | --sse <url>   remote transport (omit for a stdio command)
  --env K=V                    set an environment variable (repeatable, stdio)
  --header K=V                 set an HTTP header (repeatable, remote)

Examples:
  ARCDESK mcp add fs npx -y @modelcontextprotocol/server-filesystem .
  ARCDESK mcp add stripe --http https://mcp.stripe.com --header "Authorization=Bearer $STRIPE_KEY"

Changes take effect on the next session; inside a running chat, use /mcp add to
connect a server live.`)
}
