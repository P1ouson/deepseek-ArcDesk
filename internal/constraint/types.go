package constraint

import (
	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
)

// Rule identifies a constraint category.
type Rule string

const (
	RuleDuplicate Rule = "duplicate"
	RuleReuse     Rule = "reuse"
	RuleFakeUI    Rule = "fake_ui"
	RuleArch      Rule = "architecture"
)

// Severity decides whether a violation blocks the write or only advises.
type Severity string

const (
	SeverityBlock Severity = "block"
	SeverityWarn  Severity = "warn"
)

// Violation is one constraint finding for a pending edit.
type Violation struct {
	Rule     Rule     `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
}

// Result is the outcome of checking one pending file change.
type Result struct {
	Path       string      `json:"path"`
	Blocked    bool        `json:"blocked"`
	Violations []Violation `json:"violations,omitempty"`
}

// Host wires repo indexes used by constraint checks.
type Host struct {
	Root      string
	Dep       *dependency.Index
	Callgraph *callgraph.Index
}

// Settings toggles individual rules. Zero value enables all rules.
type Settings struct {
	BlockDuplicate bool
	BlockFakeUI    bool
	BlockArch      bool
	AdviseReuse    bool
}

// DefaultSettings returns production defaults: block risky edits, advise reuse.
func DefaultSettings() Settings {
	return Settings{
		BlockDuplicate: true,
		BlockFakeUI:    true,
		BlockArch:      true,
		AdviseReuse:    true,
	}
}
