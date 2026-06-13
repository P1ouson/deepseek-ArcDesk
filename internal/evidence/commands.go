package evidence

import "strings"

// CommandSatisfiesCheck reports whether an executed bash command counts as having
// run a required host verify check. Compound shells (cd …; go test …), extra
// flags, and narrower Go package paths are accepted when they clearly run the
// same check.
func CommandSatisfiesCheck(executed, required string) bool {
	required = trimCommandNoise(required)
	if required == "" {
		return false
	}
	executed = trimCommandNoise(executed)
	if executed == "" {
		return false
	}
	if segmentCoversCheck(executed, required) {
		return true
	}
	for _, seg := range commandSegments(executed) {
		if segmentCoversCheck(seg, required) {
			return true
		}
	}
	return false
}

func commandSegments(cmd string) []string {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return nil
	}
	cmd = strings.ReplaceAll(cmd, "\r\n", "\n")
	var parts []string
	for _, piece := range strings.FieldsFunc(cmd, func(r rune) bool {
		return r == ';' || r == '\n'
	}) {
		for _, sub := range splitAND(piece) {
			if s := trimCommandNoise(sub); s != "" {
				parts = append(parts, s)
			}
		}
	}
	if len(parts) == 0 {
		if s := trimCommandNoise(cmd); s != "" {
			return []string{s}
		}
		return nil
	}
	return parts
}

func splitAND(s string) []string {
	var out []string
	for _, p := range strings.Split(s, "&&") {
		out = append(out, p)
	}
	return out
}

func trimCommandNoise(s string) string {
	s = normalizeCommand(s)
	if s == "" {
		return ""
	}
	lower := strings.ToLower(s)
	if i := strings.Index(s, " 2>&1"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	if i := strings.Index(lower, " if ($?"); i > 0 {
		s = strings.TrimSpace(s[:i])
	}
	return normalizeCommand(s)
}

func normalizeCommand(cmd string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(cmd)), " ")
}

func segmentCoversCheck(executed, required string) bool {
	executed = trimCommandNoise(executed)
	required = trimCommandNoise(required)
	if executed == "" || required == "" {
		return false
	}
	if executed == required {
		return true
	}
	if strings.HasPrefix(executed, required+" ") {
		return true
	}
	// go test ./internal/counter/... satisfies go test ./...
	if strings.HasSuffix(required, "./...") {
		prefix := strings.TrimSuffix(required, "./...")
		if prefix != "" && strings.HasPrefix(executed, prefix) {
			rest := strings.TrimPrefix(executed, prefix)
			if strings.HasPrefix(rest, "./") {
				return true
			}
		}
	}
	return false
}
