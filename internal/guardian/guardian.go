package guardian

import (
	"strings"
	"sync"

	"arcdesk/internal/archrag"
	"arcdesk/internal/constraint"
	"arcdesk/internal/diff"
)

// Severity mirrors constraint severities for guardian findings.
type Severity string

const (
	SeverityBlock Severity = "block"
	SeverityWarn  Severity = "warn"
)

// Violation is one SPEC-aware architecture finding.
type Violation struct {
	Rule     string   `json:"rule"`
	Severity Severity `json:"severity"`
	Message  string   `json:"message"`
	Hint     string   `json:"hint,omitempty"`
	Source   string   `json:"source,omitempty"`
}

// Guardian layers SPEC rules from architecture RAG on the constraint engine.
type Guardian struct {
	idx   *archrag.Index
	eng   *constraint.Engine
	rules []SpecRule

	mu     sync.RWMutex
	checks int
	blocks int
	warns  int
	last   constraint.Result
}

// New builds a guardian from an architecture index and optional constraint engine.
func New(idx *archrag.Index, eng *constraint.Engine) *Guardian {
	return &Guardian{
		idx:   idx,
		eng:   eng,
		rules: CompileRules(idx),
	}
}

// Rules returns compiled SPEC rules.
func (g *Guardian) Rules() []SpecRule {
	if g == nil {
		return nil
	}
	return append([]SpecRule(nil), g.rules...)
}

// CheckEdit evaluates a pending writer-tool change.
func (g *Guardian) CheckEdit(toolName string, ch diff.Change) constraint.Result {
	if g == nil {
		return constraint.Result{}
	}
	res := g.checkPath(ch.Path, ch.OldText, ch.NewText)
	g.record(res)
	_ = toolName
	return res
}

// CheckPath dry-runs checks for tools and previews.
func (g *Guardian) CheckPath(path, oldText, newText string) constraint.Result {
	if g == nil {
		return constraint.Result{}
	}
	res := g.checkPath(path, oldText, newText)
	g.record(res)
	return res
}

func (g *Guardian) checkPath(path, oldText, newText string) constraint.Result {
	var res constraint.Result
	if g.eng != nil {
		res = g.eng.CheckPath(path, oldText, newText)
	} else {
		res.Path = normalizePath(path)
	}
	added := addedLines(oldText, newText)
	for _, v := range checkSpecRules(g.rules, path, added) {
		cv := constraint.Violation{
			Rule:     constraint.RuleArch,
			Severity: constraint.Severity(v.Severity),
			Message:  v.Message,
			Hint:     v.Hint,
		}
		if v.Severity == SeverityBlock {
			res.Blocked = true
		}
		res.Violations = append(res.Violations, cv)
	}
	if res.Path == "" {
		res.Path = normalizePath(path)
	}
	return res
}

func (g *Guardian) record(res constraint.Result) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.last = res
	g.checks++
	if res.Blocked {
		g.blocks++
	}
	for _, v := range res.Violations {
		if v.Severity == constraint.SeverityWarn {
			g.warns++
		}
	}
}

// Stats returns aggregate counters.
func (g *Guardian) Stats() (checks, blocks, warnings int) {
	if g == nil {
		return 0, 0, 0
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.checks, g.blocks, g.warns
}

// LastResult returns the most recent check outcome.
func (g *Guardian) LastResult() constraint.Result {
	if g == nil {
		return constraint.Result{}
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.last
}

// SummaryLine returns a one-line status for tools.
func (g *Guardian) SummaryLine() string {
	if g == nil {
		return "Architecture guardian: unavailable"
	}
	checks, blocks, warns := g.Stats()
	return strings.TrimSpace(
		"Architecture guardian: rules=" + itoa(len(g.rules)) +
			" checks=" + itoa(checks) +
			" blocks=" + itoa(blocks) +
			" warnings=" + itoa(warns),
	)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [12]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
