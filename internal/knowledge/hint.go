package knowledge

import (
	"fmt"
	"strings"
)

const tagOpen = "<knowledge-hint>"
const tagClose = "</knowledge-hint>"

// FormatHint wraps one line of experience text for verify-retry injection.
func FormatHint(line string, maxChars int) string {
	line = strings.TrimSpace(line)
	if line == "" {
		return ""
	}
	if maxChars <= 0 {
		maxChars = 200
	}
	inner := line
	if len(inner) > maxChars {
		inner = inner[:maxChars-1] + "…"
	}
	return fmt.Sprintf("%s\n%s\n%s", tagOpen, inner, tagClose)
}

// IsHint reports whether s contains a knowledge hint block.
func IsHint(s string) bool {
	return strings.Contains(s, tagOpen)
}
