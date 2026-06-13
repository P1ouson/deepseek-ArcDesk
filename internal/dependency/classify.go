package dependency

import (
	"os"
	"path/filepath"
	"strings"
)

// classifyImport resolves an import path to a graph node id and kind.
//
// Primary path (go list): use pkg.Standard from go list JSON.
//
// Parser fallback (no go list), in order:
//  1. Module prefix match → InternalGo
//  2. GOROOT probe: $GOROOT/src/<importPath> exists as .go file or directory with .go files
//  3. Heuristic: import path contains no '.' → Stdlib (matches cmd/go module-mode rule)
//  4. Otherwise → ExternalGo (module path via modulePathForImport)
func classifyImport(modulePath, importPath, goroot string, fromGoListStandard bool) (NodeID, Kind) {
	importPath = strings.TrimSpace(importPath)
	if importPath == "" {
		return "", ""
	}
	if fromGoListStandard {
		return NewStdlibID(importPath), KindStdlib
	}
	if modulePath != "" && (importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")) {
		return NewGoID(importPath), KindInternalGo
	}
	if isStdlibImport(importPath, goroot) {
		return NewStdlibID(importPath), KindStdlib
	}
	mod := modulePathForImport(importPath)
	return NewGoModID(mod), KindExternalGo
}

func isStdlibImport(importPath, goroot string) bool {
	if goroot != "" && stdlibExistsUnderGOROOT(goroot, importPath) {
		return true
	}
	// cmd/go: in module mode, paths without a dot are standard library packages.
	return !strings.Contains(importPath, ".")
}

func stdlibExistsUnderGOROOT(goroot, importPath string) bool {
	base := filepath.Join(goroot, "src", filepath.FromSlash(importPath))
	if st, err := os.Stat(base); err == nil {
		if st.IsDir() {
			entries, err := os.ReadDir(base)
			if err != nil {
				return false
			}
			for _, e := range entries {
				name := e.Name()
				if strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".") {
					return true
				}
			}
			return false
		}
		return strings.HasSuffix(base, ".go")
	}
	return false
}

// modulePathForImport extracts the module path from a package import path.
func modulePathForImport(importPath string) string {
	parts := strings.Split(importPath, "/")
	if len(parts) == 0 || !strings.Contains(parts[0], ".") {
		return importPath
	}
	end := 2
	if len(parts) >= 3 {
		end = 3
	}
	if end < len(parts) && isMajorVersionSuffix(parts[end]) {
		end++
	}
	if end > len(parts) {
		end = len(parts)
	}
	return strings.Join(parts[:end], "/")
}

func isMajorVersionSuffix(s string) bool {
	if len(s) < 2 || s[0] != 'v' {
		return false
	}
	for _, c := range s[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
