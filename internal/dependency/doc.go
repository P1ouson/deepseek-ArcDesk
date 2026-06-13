// Package dependency provides module-level dependency analysis for ArcDesk workspaces:
// Go and frontend package import graphs, manifest dependencies, impact analysis, and
// cycle detection persisted under <workspace>/.arcdesk/dependency/.
//
// It complements internal/codegraph, which wraps an external MCP server for symbol-
// and call-graph queries (functions, types, callers). Use codegraph_* tools for
// "who calls GetUser"; use dependency_* tools for "what packages break if I change
// internal/agent".
//
// Design principles: native Go, no third-party deps, JSON persistence, in-memory
// adjacency indexes for O(1) neighbor lookup, precomputed impact for O(1) AffectedBy.
package dependency
