package main

import (
	"encoding/json"
	"os"
)

func builtinMCPCatalog() []MCPCatalogEntry {
	return []MCPCatalogEntry{
		{
			ID: "github", Name: "GitHub", Category: "Developer",
			Description: "Browse repos, issues, and pull requests through the official GitHub MCP server.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-github"}, Tier: "lazy", Official: true,
			Requires: []string{"node"},
			EnvFields: []MCPCatalogEnvField{
				{Key: "GITHUB_PERSONAL_ACCESS_TOKEN", Required: true, Secret: true},
			},
			SetupNotes: []string{"githubToken"},
		},
		{
			ID: "filesystem", Name: "Filesystem", Category: "Files",
			Description: "Read and write files within allowed workspace directories.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-filesystem"}, Tier: "eager", Official: true,
			Requires:   []string{"node"},
			SetupNotes: []string{"filesystemPaths"},
		},
		{
			ID: "brave-search", Name: "Brave Search", Category: "Search",
			Description: "Web search for live documentation, release notes, and troubleshooting.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-brave-search"}, Tier: "lazy", Official: true,
			Requires: []string{"node"},
			EnvFields: []MCPCatalogEnvField{
				{Key: "BRAVE_API_KEY", Required: true, Secret: true},
			},
		},
		{
			ID: "linear", Name: "Linear", Category: "Project",
			Description: "Create and update Linear issues, projects, and teams.",
			Transport: "http", URL: "https://mcp.linear.app/mcp", Tier: "lazy", Official: true,
			SetupNotes: []string{"remoteOAuth"},
		},
		{
			ID: "playwright", Name: "Playwright", Category: "Browser",
			Description: "Drive a browser for UI verification, screenshots, and end-to-end checks.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@playwright/mcp@latest"}, Tier: "background",
			Requires:   []string{"node"},
			SetupNotes: []string{"playwrightBrowsers"},
		},
		{
			ID: "postgres", Name: "PostgreSQL", Category: "Database",
			Description: "Inspect schemas and run read-only SQL against a configured database.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-postgres"}, Tier: "lazy", Official: true,
			Requires: []string{"node"},
			EnvFields: []MCPCatalogEnvField{
				{Key: "POSTGRES_CONNECTION_STRING", Required: true, Secret: true},
			},
		},
		{
			ID: "sentry", Name: "Sentry", Category: "Developer",
			Description: "Query issues, releases, and project health from Sentry.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-sentry"}, Tier: "lazy", Official: true,
			Requires: []string{"node"},
			EnvFields: []MCPCatalogEnvField{
				{Key: "SENTRY_AUTH_TOKEN", Required: true, Secret: true},
			},
		},
		{
			ID: "slack", Name: "Slack", Category: "Communication",
			Description: "Read channels and post updates through the Slack MCP server.",
			Transport: "stdio", Command: "npx", Args: []string{"-y", "@modelcontextprotocol/server-slack"}, Tier: "lazy", Official: true,
			Requires: []string{"node"},
			EnvFields: []MCPCatalogEnvField{
				{Key: "SLACK_BOT_TOKEN", Required: true, Secret: true},
				{Key: "SLACK_TEAM_ID", Required: false, Secret: false},
			},
		},
	}
}

func loadMCPCatalogExtras() []MCPCatalogEntry {
	path := ARCDESKDesktopDataPath("mcp-catalog.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var items []MCPCatalogEntry
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

func mergeMCPCatalog(builtin, extras []MCPCatalogEntry) []MCPCatalogEntry {
	seen := make(map[string]struct{}, len(builtin)+len(extras))
	out := make([]MCPCatalogEntry, 0, len(builtin)+len(extras))
	for _, item := range builtin {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	for _, item := range extras {
		if item.ID == "" {
			continue
		}
		if _, ok := seen[item.ID]; ok {
			continue
		}
		seen[item.ID] = struct{}{}
		out = append(out, item)
	}
	return out
}
