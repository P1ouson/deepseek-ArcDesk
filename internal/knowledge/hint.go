package knowledge

import (
	"fmt"
	"strings"
)

const tagOpen = "<knowledge-hint>"
const tagClose = "</knowledge-hint>"

// FormatHint wraps one line of experience text for verify-retry injection.
func FormatHint(line string, maxChars int) string {
	return FormatHintWithMeta(line, maxChars, "", "", "")
}

// FormatHintWithMeta wraps a lesson with provenance metadata for auditability.
func FormatHintWithMeta(line string, maxChars int, confidence, head, note string) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 200
	}
	metaParts := []string{"source=knowledge-hint"}
	if conf := strings.TrimSpace(confidence); conf != "" {
		metaParts = append(metaParts, "confidence="+conf)
	}
	if h := strings.TrimSpace(head); h != "" {
		metaParts = append(metaParts, "head="+h)
	}
	if n := strings.TrimSpace(note); n != "" {
		metaParts = append(metaParts, n)
	}
	prefix := "[" + strings.Join(metaParts, " ") + "] "
	inner := prefix + line
	if len(inner) > maxChars {
		inner = inner[:maxChars-1] + "…"
	}
	return fmt.Sprintf("%s\n%s\n%s", tagOpen, inner, tagClose)
}

// IsHint reports whether s contains a knowledge hint block.
func IsHint(s string) bool {
	return strings.Contains(s, tagOpen)
}
