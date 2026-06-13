// Package callgraph indexes Wails cross-realm call chains: React/TS UI and hooks
// through bridge invocations to Go bind methods (and optional Go internal callees).
//
// Use callgraph_* tools for end-to-end UI↔Go traces. Use dependency_* for
// module-level package impact and codegraph_* for Go symbol-level callers.
package callgraph
