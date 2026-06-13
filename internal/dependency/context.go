package dependency

import (
	"fmt"
	"strings"
	"time"
)

var verifyKeywords = []string{"build", "test", "vet", "lint", "compile"}

// IsVerifyCommand reports whether cmd looks like a build/test verification command.
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

// BuildFailureContext formats dependency impact for a failed verification command.
// Returns empty string when failedCmd is not a verification command.
func BuildFailureContext(idx *Index, changedPaths []string, failedCmd, stderr string) string {
	if !IsVerifyCommand(failedCmd) {
		return ""
	}
	if idx == nil {
		return "dependency index unavailable"
	}

	seen := map[NodeID]ImpactResult{}
	var blocks []string
	for _, path := range changedPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		res, err := idx.AffectedByPath(path)
		if err != nil {
			continue
		}
		if _, ok := seen[res.Source]; ok {
			continue
		}
		seen[res.Source] = res
		if line := formatChangedImpactLine(path, res); line != "" {
			blocks = append(blocks, line)
		}
	}

	var lines []string
	lines = append(lines, "## Dependency Impact")
	if len(blocks) == 0 {
		lines = append(lines, "- **Changed**: (no mapped packages for written paths)")
	} else {
		for _, b := range blocks {
			lines = append(lines, b)
		}
	}
	if stderr := strings.TrimSpace(stderr); stderr != "" {
		if len(stderr) > 240 {
			stderr = stderr[:237] + "..."
		}
		lines = append(lines, fmt.Sprintf("- **Verify error**: `%s` — %s", failedCmd, stderr))
	}
	if stats, err := idx.Status(); err == nil {
		age := "unknown"
		if !stats.BuiltAt.IsZero() {
			age = fmt.Sprintf("%ds ago", int(time.Since(stats.BuiltAt).Seconds()))
		}
		lines = append(lines, fmt.Sprintf("- **Graph status**: %d nodes, built %s (%s)", stats.NodeCount, age, stats.BuildMethod))
	} else {
		lines = append(lines, "- **Graph status**: unavailable")
	}

	if len(lines) > 8 {
		lines = lines[:8]
	}
	return strings.Join(lines, "\n")
}

func formatChangedImpactLine(path string, res ImpactResult) string {
	direct := formatImpactNames(res.Layers.Direct, 5)
	transitive := len(res.Layers.Transitive)
	external := len(res.Layers.External)
	line := fmt.Sprintf("- **Changed**: `%s`", path)
	if direct != "" {
		line += fmt.Sprintf(" → directly imported by %s", direct)
	}
	if transitive > 0 {
		line += fmt.Sprintf("; %d transitive importer(s)", transitive)
	}
	if external > 0 {
		line += fmt.Sprintf("; %d external dep(s) in chain", external)
	}
	return line
}

func formatImpactNames(entries []ImpactEntry, max int) string {
	if len(entries) == 0 {
		return ""
	}
	if len(entries) > max {
		entries = entries[:max]
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name
		if name == "" {
			name = string(e.ID)
		}
		names = append(names, "`"+name+"`")
	}
	out := strings.Join(names, ", ")
	if len(entries) == max {
		out += ", …"
	}
	return out
}

// RiskBeforeEdit is reserved for Phase 2 pre-edit risk hints.
func RiskBeforeEdit(idx *Index, paths []string) string {
	_ = idx
	_ = paths
	return ""
}
