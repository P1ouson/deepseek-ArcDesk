package constraint

import (
	"fmt"
	"strings"
	"sync"

	"arcdesk/internal/diff"
)

// Engine runs pre-edit constraint checks for writer tools.
type Engine struct {
	host     *Host
	settings Settings

	mu       sync.RWMutex
	last     Result
	checks   int
	blocks   int
	warnings int
}

// NewEngine constructs a constraint engine. Pass DefaultSettings() for production rules.
func NewEngine(host *Host, settings Settings) *Engine {
	return &Engine{host: host, settings: settings}
}

// CheckEdit evaluates one pending file change from a writer tool preview.
func (e *Engine) CheckEdit(toolName string, ch diff.Change) Result {
	_ = toolName
	res := Result{Path: normalizePath(ch.Path)}
	if e == nil || e.host == nil || ch.Binary || strings.TrimSpace(ch.Path) == "" {
		e.record(res)
		return res
	}
	added := addedLines(ch.OldText, ch.NewText)
	kind := string(ch.Kind)

	var violations []Violation
	if e.settings.BlockDuplicate {
		violations = append(violations, checkDuplicate(e.host, ch.Path, added)...)
	}
	if e.settings.AdviseReuse {
		violations = append(violations, checkReuse(e.host, ch.Path, added, kind)...)
	}
	if e.settings.BlockFakeUI {
		violations = append(violations, checkFakeUI(ch.Path, ch.OldText, ch.NewText, added)...)
	}
	if e.settings.BlockArch {
		violations = append(violations, checkArchitecture(ch.Path, added)...)
	}

	for _, v := range violations {
		if v.Severity == SeverityBlock {
			res.Blocked = true
		}
	}
	res.Violations = violations
	e.record(res)
	return res
}

func (e *Engine) record(res Result) {
	if e == nil {
		return
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	e.last = res
	e.checks++
	if res.Blocked {
		e.blocks++
	}
	for _, v := range res.Violations {
		if v.Severity == SeverityWarn {
			e.warnings++
		}
	}
}

// LastResult returns the most recent check outcome.
func (e *Engine) LastResult() Result {
	if e == nil {
		return Result{}
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.last
}

// Stats returns aggregate counters for status tools.
func (e *Engine) Stats() (checks, blocks, warnings int) {
	if e == nil {
		return 0, 0, 0
	}
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.checks, e.blocks, e.warnings
}

// FormatBlockMessage renders a user-facing block reason.
func (r Result) FormatBlockMessage() string {
	var blocks []string
	for _, v := range r.Violations {
		if v.Severity != SeverityBlock {
			continue
		}
		line := v.Message
		if v.Hint != "" {
			line += " — " + v.Hint
		}
		blocks = append(blocks, line)
	}
	if len(blocks) == 0 {
		return "edit blocked by constraint policy"
	}
	return strings.Join(blocks, "; ")
}

// FormatWarnHint renders non-blocking guidance appended after successful writes.
func (r Result) FormatWarnHint() string {
	var warns []string
	for _, v := range r.Violations {
		if v.Severity != SeverityWarn {
			continue
		}
		line := "[constraint] " + v.Message
		if v.Hint != "" {
			line += " — " + v.Hint
		}
		warns = append(warns, line)
	}
	return strings.Join(warns, "\n")
}

// CheckPath runs constraint heuristics against arbitrary path/content (tool API).
func (e *Engine) CheckPath(path, oldText, newText string) Result {
	if e == nil {
		return Result{}
	}
	kind := diff.Modify
	if strings.TrimSpace(oldText) == "" && strings.TrimSpace(newText) != "" {
		kind = diff.Create
	}
	return e.CheckEdit("constraint_check", diff.Change{
		Path:    path,
		Kind:    kind,
		OldText: oldText,
		NewText: newText,
	})
}

// BuildRetryContext adds constraint reminders to self-debug retries after writes.
func BuildRetryContext(e *Engine, writtenPaths []string) string {
	if e == nil || len(writtenPaths) == 0 {
		return ""
	}
	last := e.LastResult()
	if len(last.Violations) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Constraint System\n")
	b.WriteString("- no duplicate implementations; prefer reusing existing modules\n")
	b.WriteString("- no fake UI fixes (mock data, CSS-only functional hacks)\n")
	b.WriteString("- keep Wails layering: UI → bridge → Go bind\n")
	if msg := last.FormatBlockMessage(); msg != "" && last.Blocked {
		b.WriteString("- last blocked edit: ")
		b.WriteString(msg)
		b.WriteByte('\n')
	}
	if hint := last.FormatWarnHint(); hint != "" {
		b.WriteString(hint)
		b.WriteByte('\n')
	}
	if len(writtenPaths) > 0 {
		b.WriteString(fmt.Sprintf("- recent writes: %s\n", strings.Join(limitPaths(writtenPaths, 6), ", ")))
	}
	return strings.TrimSpace(b.String())
}

func limitPaths(paths []string, max int) []string {
	if max <= 0 || len(paths) <= max {
		return append([]string(nil), paths...)
	}
	return append([]string(nil), paths[:max]...)
}
