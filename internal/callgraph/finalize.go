package callgraph

import "strings"

// finalizeWarnings aggregates unused bind and App.d.ts drift warnings.
func finalizeWarnings(g *CallGraph, binds []GoBindMethod, catalog map[string]bool, dtsMethods map[string]bool) []ParseWarning {
	if g == nil {
		return nil
	}
	var out []ParseWarning

	usedBind := map[string]bool{}
	for method := range g.BridgeByMethod {
		usedBind[method] = true
	}

	for _, b := range binds {
		if !usedBind[b.Method] {
			out = append(out, ParseWarning{
				File:    b.File,
				Message: "unused_bind:" + b.Method,
			})
		}
	}

	if len(dtsMethods) > 0 {
		for method := range dtsMethods {
			found := false
			for _, b := range binds {
				if b.Method == method {
					found = true
					break
				}
			}
			if !found {
				out = append(out, ParseWarning{Message: "app_dts_drift:extra:" + method})
			}
		}
		for _, b := range binds {
			if !dtsMethods[b.Method] {
				out = append(out, ParseWarning{
					File:    b.File,
					Message: "app_dts_drift:missing:" + b.Method,
				})
			}
		}
	}

	if len(catalog) == 0 && len(binds) == 0 {
		out = append(out, ParseWarning{Message: "method_catalog_empty"})
	}
	return out
}

func countParseErrors(warnings []ParseWarning) int {
	n := 0
	for _, w := range warnings {
		msg := strings.ToLower(w.Message)
		if strings.Contains(msg, "parse_error") ||
			strings.Contains(msg, "go_parse_error") ||
			strings.Contains(msg, "ts_parse_error") ||
			strings.Contains(msg, "go/parser fallback") {
			n++
		}
	}
	return n
}

func hasWarningMessage(warnings []ParseWarning, needle string) bool {
	for _, w := range warnings {
		if strings.Contains(w.Message, needle) {
			return true
		}
	}
	return false
}
