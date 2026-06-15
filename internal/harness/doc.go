// Package harness is ArcDesk's unified control plane.
//
// LLM providers only generate text and tool-call proposals. Every lifecycle
// decision — plan vs execute, gates, memory consolidation, halt — is owned here
// so frontends and the agent loop share one orchestration contract.
//
// Seven logical layers (all entry points live under this package or are
// registered into [Plane] at boot):
//
//  1. ControlPlane — [Plane] + [FSM] (PLAN → EXECUTE → VERIFY → HALT/REPLAN)
//  2. Memory — [FourLayer] + [Consolidate]
//  3. Context — cache-stable prefix assembly (see [context] subpackage)
//  4. SubAgent — task / skill subagent delegation (see [subagent] subpackage)
//  5. RAG — repo indexes exposed as tools (see [rag] subpackage)
//  6. Skills — playbooks (wired at boot, indexed in prefix)
//  7. MCP — external tool servers (wired at boot)
//
// Use [Plane.Status] or the harness_status tool to introspect layers without
// searching the tree.
package harness
