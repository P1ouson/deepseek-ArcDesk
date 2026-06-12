package apikey

import "strings"

// Normalize trims whitespace and strips a leading "Bearer " prefix users often
// paste from docs, so Authorization does not become "Bearer Bearer sk-…".
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	for {
		lower := strings.ToLower(raw)
		if strings.HasPrefix(lower, "bearer ") {
			raw = strings.TrimSpace(raw[7:])
			continue
		}
		break
	}
	return raw
}
