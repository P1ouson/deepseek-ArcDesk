package main

import (
	"strings"

	"arcdesk/internal/plugin"
)

// MCPPrerequisiteView is one prerequisite check result for the install wizard.
type MCPPrerequisiteView struct {
	ID     string `json:"id"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

// MCPPrerequisitesView aggregates prerequisite checks requested by the catalog.
type MCPPrerequisitesView struct {
	Items []MCPPrerequisiteView `json:"items"`
}

// CheckMCPPrerequisites runs lightweight host checks (node, npx, …) for the UI.
func (a *App) CheckMCPPrerequisites(ids []string) MCPPrerequisitesView {
	seen := map[string]bool{}
	out := MCPPrerequisitesView{Items: []MCPPrerequisiteView{}}
	for _, raw := range ids {
		id := strings.ToLower(strings.TrimSpace(raw))
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		switch id {
		case "node":
			out.Items = append(out.Items, probeNodePrerequisite())
		default:
			out.Items = append(out.Items, MCPPrerequisiteView{ID: id, OK: true})
		}
	}
	return out
}

func probeNodePrerequisite() MCPPrerequisiteView {
	nodePath, npxPath, version, ok := plugin.NodeRuntimeProbe()
	if !ok {
		detail := ""
		if nodePath != "" && npxPath == "" {
			detail = "node=" + nodePath
		}
		return MCPPrerequisiteView{ID: "node", OK: false, Detail: detail}
	}
	detail := version
	if detail == "" {
		detail = npxPath
	}
	return MCPPrerequisiteView{ID: "node", OK: true, Detail: detail}
}

func catalogRequiresNode(entry MCPCatalogEntry) bool {
	for _, req := range entry.Requires {
		if strings.EqualFold(strings.TrimSpace(req), "node") {
			return true
		}
	}
	transport := strings.ToLower(strings.TrimSpace(entry.Transport))
	command := strings.ToLower(strings.TrimSpace(entry.Command))
	return (transport == "" || transport == "stdio") && command == "npx"
}

func inferredCatalogRequires(entry MCPCatalogEntry) []string {
	if len(entry.Requires) > 0 {
		return append([]string(nil), entry.Requires...)
	}
	if catalogRequiresNode(entry) {
		return []string{"node"}
	}
	return nil
}
