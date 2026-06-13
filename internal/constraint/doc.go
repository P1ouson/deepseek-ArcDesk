// Package constraint enforces P0 coding-agent guardrails before writes:
// no duplicate implementations, prefer reusing existing logic, no fake UI fixes,
// and Wails architectural consistency (UI → bridge → Go bind).
package constraint
