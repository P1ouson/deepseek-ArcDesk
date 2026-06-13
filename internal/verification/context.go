package verification

import (
	"fmt"
	"sort"
	"strings"

	"arcdesk/internal/instruction"
)

var verifyKeywords = []string{
	"build", "test", "vet", "lint", "compile",
	"vitest", "playwright", "jest", "cypress", "pnpm", "npm", "go ",
}

// IsVerifyCommand reports whether cmd looks like a verification command.
func IsVerifyCommand(cmd string) bool {
	cmd = strings.ToLower(strings.TrimSpace(cmd))
	if cmd == "" {
		return false
	}
	for _, kw := range verifyKeywords {
		if strings.Contains(cmd, kw) {
			return true
		}
	}
	return false
}

// BuildRetryContext formats verification guidance for a failed check.
func BuildRetryContext(checks []instruction.VerifyCheck, failedCmd, stderr string) string {
	failedCmd = strings.TrimSpace(failedCmd)
	if failedCmd == "" || !IsVerifyCommand(failedCmd) {
		return ""
	}
	var cat Category
	for _, c := range checks {
		if strings.TrimSpace(c.Command) == failedCmd {
			cat = categoryOf(c.Category)
			break
		}
	}
	var b strings.Builder
	b.WriteString("## Verification Engine\n")
	switch cat {
	case CategoryBuild:
		b.WriteString("Compile/build failed. Fix compile errors before answering.\n")
	case CategoryUnit:
		b.WriteString("Unit tests failed. Fix failing tests or the code under test.\n")
	case CategoryE2E:
		b.WriteString("Behavioral/E2E tests failed. Reproduce in the UI and fix the broken interaction.\n")
	default:
		b.WriteString("Project verification failed. Re-run the required check successfully.\n")
	}
	b.WriteString("- failed: ")
	b.WriteString(failedCmd)
	b.WriteByte('\n')
	if s := strings.TrimSpace(stderr); s != "" {
		b.WriteString("- output: ")
		b.WriteString(truncateLines(s, 12, 200))
		b.WriteByte('\n')
	}
	if others := pendingByCategory(checks, failedCmd); len(others) > 0 {
		b.WriteString("- also required after writes:\n")
		for _, line := range others {
			b.WriteString("  - ")
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return strings.TrimSpace(b.String())
}

func pendingByCategory(checks []instruction.VerifyCheck, failedCmd string) []string {
	seen := map[string]bool{failedCmd: true}
	var out []string
	for _, c := range checks {
		cmd := strings.TrimSpace(c.Command)
		if cmd == "" || seen[cmd] {
			continue
		}
		seen[cmd] = true
		label := string(categoryOf(c.Category))
		if label == "" || label == string(CategoryCustom) {
			label = "check"
		}
		out = append(out, fmt.Sprintf("[%s] %s", label, cmd))
	}
	sort.Strings(out)
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func truncateLines(s string, maxLines, maxLine int) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	if len(lines) > maxLines {
		lines = lines[len(lines)-maxLines:]
	}
	for i, ln := range lines {
		if len(ln) > maxLine {
			lines[i] = ln[:maxLine] + "…"
		}
	}
	return strings.Join(lines, "\n")
}

// NewPlan wraps resolved checks for tools and diagnostics.
func NewPlan(checks []instruction.VerifyCheck, policy Policy) Plan {
	out := make([]Check, 0, len(checks))
	for _, c := range checks {
		out = append(out, Check{
			Command:    c.Command,
			Category:   categoryOf(c.Category),
			SourcePath: c.SourcePath,
			Line:       c.Line,
		})
	}
	return Plan{Checks: out, Policy: policy}
}
