package control

import (
	"fmt"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/mcpcmd"
)

func (c *Controller) handleMCPSlash(trimmed string) {
	args := mcpcmd.TokenizeArgs(trimmed)
	if len(args) < 2 {
		c.notice(c.mcpStatusText())
		return
	}
	switch args[1] {
	case "list", "ls":
		c.notice(c.mcpStatusText())
	case "show":
		if len(args) < 3 {
			c.notice("usage: /mcp show <name>")
			return
		}
		c.notice(c.mcpShowText(args[2]))
	case "tools":
		if len(args) < 3 {
			c.notice("usage: /mcp tools <name>")
			return
		}
		c.notice(c.mcpToolsText(args[2]))
	case "add":
		entry, err := mcpcmd.ParseAdd(args[2:])
		if err != nil {
			c.notice(err.Error())
			return
		}
		n, err := c.AddMCPServer(entry)
		if err != nil {
			c.notice("mcp add: " + err.Error())
			return
		}
		c.notice(fmt.Sprintf("connected %s — %d tools, saved to config (available next message)", entry.Name, n))
	case "connect":
		if len(args) < 3 {
			c.notice("usage: /mcp connect <name>")
			return
		}
		n, err := c.ConnectConfiguredMCPServer(args[2])
		if err != nil {
			c.notice("mcp connect: " + err.Error())
			return
		}
		c.notice(fmt.Sprintf("connected %s — %d tools (available next message)", args[2], n))
	case "remove", "rm":
		if len(args) < 3 {
			c.notice("usage: /mcp remove <name>")
			return
		}
		name := args[2]
		disconnected, err := c.RemoveMCPServer(name)
		if err != nil {
			c.notice("mcp remove: " + err.Error())
			return
		}
		if disconnected {
			c.notice("disconnected " + name + " and removed it from config")
		} else {
			c.notice("removed " + name + " from config")
		}
	default:
		c.notice("unknown /mcp subcommand " + args[1] + " — try: /mcp, /mcp list, /mcp show, /mcp tools, /mcp add, /mcp connect, /mcp remove")
	}
}

func (c *Controller) mcpStatusText() string {
	if c.host == nil {
		if names := c.ConfiguredMCPNames(); len(names) == 0 {
			return c.mcpListText()
		}
	} else if len(c.host.Servers()) == 0 && len(c.host.Failures()) == 0 {
		if names := c.ConfiguredMCPNames(); len(names) == 0 {
			return c.mcpListText()
		}
	}
	var b strings.Builder
	b.WriteString("MCP servers\n")
	if c.host != nil {
		for _, s := range c.host.Servers() {
			transport := s.Transport
			if transport == "" {
				transport = "unknown"
			}
			fmt.Fprintf(&b, "  %s (%s) — %d tools", s.Name, transport, s.Tools)
			if s.Prompts > 0 || s.Resources > 0 {
				fmt.Fprintf(&b, ", %d prompts, %d resources", s.Prompts, s.Resources)
			}
			b.WriteByte('\n')
		}
		for _, f := range c.host.Failures() {
			transport := f.Transport
			if transport == "" {
				transport = "unknown"
			}
			fmt.Fprintf(&b, "  ! %s (%s) — %s\n", f.Name, transport, f.Error)
		}
	}
	for _, name := range c.DisconnectedMCPNames() {
		fmt.Fprintf(&b, "  %s (configured, not connected)\n", name)
	}
	return strings.TrimRight(b.String(), "\n")
}

func (c *Controller) mcpShowText(name string) string {
	if text, ok := c.mcpConnectedShowText(name); ok {
		return text
	}
	if c.host != nil {
		for _, f := range c.host.Failures() {
			if f.Name == name {
				transport := f.Transport
				if transport == "" {
					transport = "unknown"
				}
				return fmt.Sprintf("MCP server: %s\nstatus: failed\ntransport: %s\nerror: %s", name, transport, f.Error)
			}
		}
	}
	if entry, ok := c.configuredMCPEntry(name); ok {
		return c.mcpConfiguredShowText(name, entry)
	}
	return "MCP server not found: " + name
}

func (c *Controller) mcpConnectedShowText(name string) (string, bool) {
	if c.host == nil {
		return "", false
	}
	for _, s := range c.host.Servers() {
		if s.Name != name {
			continue
		}
		transport := s.Transport
		if transport == "" {
			transport = "unknown"
		}
		var b strings.Builder
		fmt.Fprintf(&b, "MCP server: %s\nstatus: connected\ntransport: %s\ntools: %d", name, transport, s.Tools)
		if s.Prompts > 0 {
			fmt.Fprintf(&b, "\nprompts: %d", s.Prompts)
		}
		if s.Resources > 0 {
			fmt.Fprintf(&b, "\nresources: %d", s.Resources)
		}
		if entry, ok := c.configuredMCPEntry(name); ok {
			if line := mcpCommandLine(entry); line != "" {
				fmt.Fprintf(&b, "\ncommand: %s", line)
			} else if entry.URL != "" {
				fmt.Fprintf(&b, "\nurl: %s", entry.URL)
			}
		}
		return strings.TrimRight(b.String(), "\n"), true
	}
	return "", false
}

func (c *Controller) mcpConfiguredShowText(name string, entry config.PluginEntry) string {
	typ := entry.Type
	if typ == "" {
		if entry.URL != "" {
			typ = "http"
		} else {
			typ = "stdio"
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, "MCP server: %s\nstatus: configured (not connected)\ntransport: %s", name, typ)
	if line := mcpCommandLine(entry); line != "" {
		fmt.Fprintf(&b, "\ncommand: %s", line)
	} else if entry.URL != "" {
		fmt.Fprintf(&b, "\nurl: %s", entry.URL)
	}
	if len(entry.Env) > 0 {
		keys := make([]string, 0, len(entry.Env))
		for k := range entry.Env {
			keys = append(keys, k)
		}
		fmt.Fprintf(&b, "\nenv: %s", strings.Join(keys, ", "))
	}
	return strings.TrimRight(b.String(), "\n")
}

func (c *Controller) mcpToolsText(name string) string {
	if c.host != nil {
		for _, s := range c.host.Servers() {
			if s.Name != name {
				continue
			}
			if len(s.ToolList) == 0 {
				return fmt.Sprintf("%s tools\n\nCurrent connection did not return tool details.", name)
			}
			var b strings.Builder
			fmt.Fprintf(&b, "%s tools\n", name)
			for _, t := range s.ToolList {
				desc := strings.Join(strings.Fields(t.Description), " ")
				if desc == "" {
					fmt.Fprintf(&b, "  %s\n", t.Name)
				} else {
					fmt.Fprintf(&b, "  %-20s %s\n", t.Name, desc)
				}
			}
			return strings.TrimRight(b.String(), "\n")
		}
	}
	if _, ok := c.configuredMCPEntry(name); ok {
		return name + " is configured but not connected — run /mcp connect " + name
	}
	if c.host != nil {
		for _, f := range c.host.Failures() {
			if f.Name == name {
				return name + " failed to connect — fix the error first (/mcp show " + name + ")"
			}
		}
	}
	return "MCP server not found: " + name
}

func (c *Controller) configuredMCPEntry(name string) (config.PluginEntry, bool) {
	cfg, err := config.Load()
	if err != nil {
		return config.PluginEntry{}, false
	}
	for _, p := range cfg.Plugins {
		if p.Name == name {
			return p, true
		}
	}
	return config.PluginEntry{}, false
}

func mcpCommandLine(entry config.PluginEntry) string {
	if entry.Command == "" {
		return ""
	}
	if len(entry.Args) == 0 {
		return entry.Command
	}
	return strings.TrimSpace(entry.Command + " " + strings.Join(entry.Args, " "))
}
