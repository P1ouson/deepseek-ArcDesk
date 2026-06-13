package runtime

import "strings"

// normalizeKind upgrades generic go_log rows from Wails/desktop sources.
func normalizeKind(kind Kind, source, message string, meta map[string]string) Kind {
	if kind != "" && kind != KindGoLog {
		return kind
	}
	src := strings.ToLower(strings.TrimSpace(source))
	msg := strings.ToLower(message)
	if strings.Contains(src, "wails") || strings.Contains(msg, "wails") {
		return KindWails
	}
	if meta != nil {
		switch strings.ToLower(meta["origin"]) {
		case "wails", "desktop":
			return KindWails
		}
		switch strings.ToLower(meta["stream"]) {
		case "wails", "wails_stderr":
			return KindWails
		}
	}
	if kind != "" {
		return kind
	}
	return KindGoLog
}
