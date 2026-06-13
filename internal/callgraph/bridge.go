package callgraph

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	reAppDTSExport     = regexp.MustCompile(`^export\s+function\s+([A-Z]\w*)\s*\(`)
	reAppBindingsIface = regexp.MustCompile(`^\s*([A-Z]\w*)\s*\([^)]*\)\s*:`)
	reAppBindingsMethod = regexp.MustCompile(`^\s*([A-Z]\w*)\s*\(`)
)

// ParseAppDTS reads method names from wailsjs App.d.ts.
func ParseAppDTS(root string) (map[string]bool, error) {
	path := filepath.Join(root, "desktop", "frontend", "wailsjs", "go", "main", "App.d.ts")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseAppDTSLines(string(b)), nil
}

func parseAppDTSLines(text string) map[string]bool {
	methods := map[string]bool{}
	for _, line := range strings.Split(text, "\n") {
		if m := reAppDTSExport.FindStringSubmatch(strings.TrimSpace(line)); len(m) == 2 {
			methods[m[1]] = true
		}
	}
	return methods
}

// ParseAppBindings reads method names from bridge.ts AppBindings interface.
func ParseAppBindings(root string) (map[string]bool, error) {
	path := filepath.Join(root, "desktop", "frontend", "src", "lib", "bridge.ts")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	methods := map[string]bool{}
	inIface := false
	for _, line := range strings.Split(string(b), "\n") {
		trim := strings.TrimSpace(line)
		if strings.Contains(trim, "interface AppBindings") {
			inIface = true
			continue
		}
		if inIface {
			if strings.HasPrefix(trim, "}") {
				break
			}
			if m := reAppBindingsIface.FindStringSubmatch(trim); len(m) == 2 {
				methods[m[1]] = true
			} else if m := reAppBindingsMethod.FindStringSubmatch(trim); len(m) == 2 {
				methods[m[1]] = true
			}
		}
	}
	if len(methods) == 0 {
		return nil, os.ErrNotExist
	}
	return methods, nil
}

// MethodCatalogResult holds authoritative method names and catalog warnings.
type MethodCatalogResult struct {
	Methods  map[string]bool
	DTS      map[string]bool
	Warnings []ParseWarning
}

// BuildMethodCatalog builds the authoritative method catalog. Go bind scan wins over App.d.ts.
func BuildMethodCatalog(root string, binds []GoBindMethod) MethodCatalogResult {
	res := MethodCatalogResult{Methods: map[string]bool{}, DTS: map[string]bool{}}

	for _, b := range binds {
		res.Methods[b.Method] = true
	}

	wailsjsDir := filepath.Join(root, "desktop", "frontend", "wailsjs")
	if _, err := os.Stat(wailsjsDir); os.IsNotExist(err) {
		res.Warnings = append(res.Warnings, ParseWarning{Message: "wailsjs_missing"})
	}

	dts, dtsErr := ParseAppDTS(root)
	if dtsErr == nil {
		res.DTS = dts
		for m := range dts {
			res.Methods[m] = true
		}
	} else {
		if bindings, bindErr := ParseAppBindings(root); bindErr == nil {
			for m := range bindings {
				res.Methods[m] = true
			}
			res.Warnings = append(res.Warnings, ParseWarning{Message: "App.d.ts missing; method catalog from AppBindings + Go bind"})
		} else {
			res.Warnings = append(res.Warnings, ParseWarning{Message: "App.d.ts missing; method catalog from Go bind scan"})
		}
	}

	// App.d.ts present but unparsable — try broken fallback file then regex salvage.
	if dtsErr != nil {
		broken := filepath.Join(root, "desktop", "frontend", "wailsjs", "go", "main", "App_broken.d.ts")
		if b, err := os.ReadFile(broken); err == nil {
			salvaged := parseAppDTSLines(string(b))
			if len(salvaged) > 0 {
				for m := range salvaged {
					res.Methods[m] = true
				}
				res.Warnings = append(res.Warnings, ParseWarning{File: "App_broken.d.ts", Message: "app_dts_parse_fallback"})
			}
		}
	}

	if len(res.Methods) == 0 {
		for _, b := range binds {
			res.Methods[b.Method] = true
		}
	}
	return res
}

// MethodCatalog returns valid Wails bind method names (legacy helper).
func MethodCatalog(root string, binds []GoBindMethod) map[string]bool {
	return BuildMethodCatalog(root, binds).Methods
}

// LinkBridgeEdges connects bridge_call nodes to go_bind nodes via bridge_invoke.
// Returns warnings for orphan bridge calls.
func LinkBridgeEdges(g *CallGraph, binds []GoBindMethod) []ParseWarning {
	if g == nil {
		return nil
	}
	byMethod := map[string]NodeID{}
	for _, b := range binds {
		if _, ok := byMethod[b.Method]; !ok {
			byMethod[b.Method] = b.ID
		}
	}
	var warnings []ParseWarning
	for id, n := range g.Nodes {
		if n == nil || n.Kind != KindBridgeCall {
			continue
		}
		method := bridgeMethodName(n.Name)
		gobind, ok := byMethod[method]
		if !ok {
			warnings = append(warnings, ParseWarning{
				File:    n.File,
				Line:    n.Line,
				Message: "orphan_bridge:" + method,
			})
			continue
		}
		g.AddEdge(id, gobind, EdgeBridgeInvoke)
	}
	g.RebuildIndexes()
	return warnings
}
