package rollback

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatAutoNotice renders a user-facing notice after verification rollback.
func FormatAutoNotice(report Report) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("verification failed after retries; rewound turn %d", report.Turn))
	if report.Summary != "" {
		b.WriteString(" — ")
		b.WriteString(report.Summary)
	}
	if block := FormatDiffBlock(report, 6); block != "" {
		b.WriteString("\n\n")
		b.WriteString(block)
	}
	return b.String()
}

// BuildRetryContext adds rollback preview guidance on the final verify retry.
func BuildRetryContext(report Report) string {
	if len(report.Files) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("## Rollback Preview\n")
	b.WriteString("Verification is about to exhaust; the host will rewind this turn on the next failure.\n")
	if report.Summary != "" {
		b.WriteString("- ")
		b.WriteString(report.Summary)
		b.WriteByte('\n')
	}
	if block := FormatDiffBlock(report, 4); block != "" {
		b.WriteString("\n")
		b.WriteString(block)
	}
	b.WriteString("\nFix the failing checks or accept that these edits will be reverted.")
	return strings.TrimSpace(b.String())
}

// FormatDiffBlock renders per-file unified diffs for notices and retry prompts.
func FormatDiffBlock(report Report, maxFiles int) string {
	if len(report.Files) == 0 {
		return ""
	}
	if maxFiles <= 0 || len(report.Files) <= maxFiles {
		maxFiles = len(report.Files)
	}
	var b strings.Builder
	b.WriteString("### Reverted changes\n")
	for i := 0; i < maxFiles; i++ {
		f := report.Files[i]
		b.WriteString(fmt.Sprintf("- `%s` (%s", f.Path, f.Action))
		if f.Added > 0 || f.Removed > 0 {
			b.WriteString(fmt.Sprintf(", -%d/+%d lines", f.Removed, f.Added))
		}
		b.WriteString(")\n")
		if strings.TrimSpace(f.Diff) != "" {
			b.WriteString("```diff\n")
			b.WriteString(strings.TrimRight(f.Diff, "\n"))
			b.WriteString("\n```\n")
		}
	}
	if len(report.Files) > maxFiles {
		b.WriteString(fmt.Sprintf("- … and %d more file(s)\n", len(report.Files)-maxFiles))
	}
	return strings.TrimSpace(b.String())
}

// FormatJSON returns the report as compact JSON for tools.
func FormatJSON(report Report) string {
	b, _ := json.Marshal(report)
	return string(b)
}
