package callgraph

import (
	"encoding/json"
	"fmt"
	"strings"
)

// FormatPathsSummary returns a one-line summary for agent tools.
func FormatPathsSummary(label string, paths []CallPath) string {
	if len(paths) == 0 {
		return label + ": 0 paths"
	}
	hops := len(paths[0].Segments)
	if hops > 0 {
		hops--
	}
	return fmt.Sprintf("%s: %d path(s), best %d hop(s)", label, len(paths), hops)
}

// FormatPathsJSON marshals paths for tool output.
func FormatPathsJSON(paths []CallPath) string {
	b, _ := json.Marshal(paths)
	return string(b)
}

// FormatLLMContext returns ≤8 lines of markdown for verify/edit prompts.
func FormatLLMContext(paths []CallPath, goMethod string) string {
	if len(paths) == 0 {
		return ""
	}
	return FormatCrossRealmContext(paths, goMethod)
}

// FormatCrossRealmContext splits RPC and event paths for LLM context.
func FormatCrossRealmContext(paths []CallPath, goMethod string) string {
	if len(paths) == 0 {
		return ""
	}
	var rpc, event []CallPath
	for _, p := range paths {
		if p.PathKind == "event" || p.EventChannel != "" {
			event = append(event, p)
		} else {
			rpc = append(rpc, p)
		}
	}
	if len(rpc) == 0 && len(event) == 0 {
		rpc = paths
	}
	var lines []string
	lines = append(lines, "## Wails Call Chain")
	if len(rpc) > 0 {
		lines = append(lines, formatPathLine("RPC", rpc[0]))
	}
	if len(event) > 0 {
		lines = append(lines, formatPathLine("Event", event[0]))
	}
	if goMethod != "" {
		lines = append(lines, fmt.Sprintf("- **Go bind**: `App.%s`", strings.TrimPrefix(goMethod, "App.")))
	}
	if len(lines) > 8 {
		lines = lines[:8]
	}
	return strings.Join(lines, "\n")
}

// FormatBreakpointContext renders suggested debug stops for LLM prompts.
func FormatBreakpointContext(bps []Breakpoint) string {
	if len(bps) == 0 {
		return ""
	}
	var lines []string
	lines = append(lines, "## Debug Breakpoints")
	for i, bp := range bps {
		if i >= 6 {
			lines = append(lines, "…")
			break
		}
		loc := bp.File
		if bp.Line > 0 {
			loc = fmt.Sprintf("%s:%d", bp.File, bp.Line)
		}
		lines = append(lines, fmt.Sprintf("- **%s**: `%s` %s — %s", strings.ToUpper(bp.Layer), loc, bp.Symbol, bp.Reason))
	}
	return strings.Join(lines, "\n")
}

func formatPathLine(label string, p CallPath) string {
	names := make([]string, 0, len(p.Segments))
	for i, seg := range p.Segments {
		if i >= 5 {
			names = append(names, "…")
			break
		}
		n := seg.Node.Name
		if seg.Node.Kind == KindEventEmit || seg.Node.Kind == KindEventListen {
			n = "emit " + seg.Node.Name
			if seg.Node.Kind == KindEventListen {
				n = "on " + seg.Node.Name
			}
		}
		names = append(names, n)
	}
	line := fmt.Sprintf("- **%s**: %s", label, strings.Join(names, " → "))
	if p.EventChannel != "" {
		line += fmt.Sprintf(" (via event %q)", p.EventChannel)
	}
	return line
}
