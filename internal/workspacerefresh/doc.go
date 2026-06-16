// Package workspacerefresh orchestrates repo index refresh (Phase 4): skip work
// when git HEAD changes only in non-index paths, bump meta without full rebuild,
// and expose unified refresh status for agents.
package workspacerefresh
