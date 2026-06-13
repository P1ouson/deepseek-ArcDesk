package guardian

import (
	"fmt"
	"strings"

	"arcdesk/internal/constraint"
)

// BuildRetryContext adds SPEC-aware architecture reminders to verify retries.
func BuildRetryContext(g *Guardian, writtenPaths []string) string {
	if g == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Architecture Guardian\n")
	b.WriteString("- consult SPEC/architecture docs before moving logic across layers\n")
	b.WriteString("- keep business logic in internal/; Wails binds in desktop/\n")
	if len(g.rules) > 0 {
		b.WriteString(fmt.Sprintf("- %d SPEC rule(s) loaded from architecture docs\n", len(g.rules)))
		for i, rule := range g.rules {
			if i >= 3 {
				b.WriteString(fmt.Sprintf("- … and %d more rule(s)\n", len(g.rules)-3))
				break
			}
			b.WriteString("- ")
			b.WriteString(rule.Text)
			b.WriteByte('\n')
		}
	}
	if g.eng != nil {
		if extra := constraint.BuildRetryContext(g.eng, writtenPaths); extra != "" {
			b.WriteString("\n")
			b.WriteString(extra)
		}
	}
	return strings.TrimSpace(b.String())
}
