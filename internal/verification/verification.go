// Package verification resolves post-write project checks and retry policy for
// the agent's final-answer readiness gate (P0 verification loop).
package verification

import (
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/instruction"
	"arcdesk/internal/memory"
)

// Policy controls how many times the agent may retry before verification is
// considered exhausted and what the host does then.
type Policy struct {
	MaxRetries int
	OnFailure  string // retry | rollback | ask
}

const defaultMaxRetries = 3

// Resolve merges explicit config, AGENTS.md host checks, and optional auto-
// discovery into the checks wired into the agent. The returned enabled flag is
// false when verification is disabled or no checks were resolved.
func Resolve(root string, cfg config.VerificationConfig, memDocs []memory.Source) ([]instruction.VerifyCheck, Policy, bool) {
	maxRetries, onFailure := cfg.ResolvedPolicy()
	policy := Policy{MaxRetries: maxRetries, OnFailure: onFailure}
	if cfg.Disabled() {
		return nil, policy, false
	}

	seen := map[string]bool{}
	var checks []instruction.VerifyCheck
	add := func(c instruction.VerifyCheck) {
		cmd := strings.TrimSpace(c.Command)
		if cmd == "" || seen[cmd] {
			return
		}
		seen[cmd] = true
		checks = append(checks, instruction.VerifyCheck{
			Command:    cmd,
			SourcePath: c.SourcePath,
			Line:       c.Line,
			Category:   c.Category,
		})
	}

	for _, cmd := range cfg.AfterWrite {
		add(instruction.VerifyCheck{Command: cmd, SourcePath: "arcdesk.toml", Category: string(CategoryCustom)})
	}
	for _, c := range instruction.ExtractHostChecks(memDocs) {
		add(c)
	}
	if cfg.AutoDiscoverEnabled() {
		for _, c := range Discover(root, cfg) {
			add(c)
		}
	}

	if len(checks) == 0 {
		return nil, policy, false
	}
	return checks, policy, true
}

// ResolvePlan is Resolve plus a structured plan for tools and retry hints.
func ResolvePlan(root string, cfg config.VerificationConfig, memDocs []memory.Source) (Plan, bool) {
	checks, policy, ok := Resolve(root, cfg, memDocs)
	if !ok {
		return Plan{Policy: policy}, false
	}
	return NewPlan(checks, policy), true
}

// DiscoverLegacy is an alias for tests that omit config options.
func DiscoverLegacy(root string) []instruction.VerifyCheck {
	return Discover(root, config.VerificationConfig{})
}
