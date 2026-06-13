package dependency

import (
	"bufio"
	"os"
	"regexp"
	"strings"
)

// JSImportKind classifies a JavaScript/TypeScript import statement.
type JSImportKind string

const (
	JSImportRelative JSImportKind = "relative"
	JSImportPackage  JSImportKind = "package"
	JSImportDynamic  JSImportKind = "dynamic"
)

// JSImport is one import/require extracted from source (no path resolution).
type JSImport struct {
	Spec string
	Line int
	Kind JSImportKind
}

var (
	reImportFrom = regexp.MustCompile(`\bimport\s+(?:type\s+)?(?:[^'";]+\s+from\s+)?['"]([^'"]+)['"]`)
	reExportFrom = regexp.MustCompile(`\bexport\s+(?:type\s+)?(?:[^'";]+\s+from\s+)['"]([^'"]+)['"]`)
	reRequire    = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reDynamic    = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	reDynamicVar = regexp.MustCompile(`\bimport\s*\(\s*([^'"\s)])`)
)

// ParseJSImports extracts import specs from a JS/TS file. Path resolution is done
// by NodeBuilder.resolveJSImport; this function only performs text extraction.
func ParseJSImports(filePath string) ([]JSImport, error) {
	b, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	return ParseJSImportsSource(string(b)), nil
}

// ParseJSImportsSource extracts imports from source text (tests).
func ParseJSImportsSource(src string) []JSImport {
	src = stripJSComments(src)
	var out []JSImport
	sc := bufio.NewScanner(strings.NewReader(src))
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		out = append(out, extractJSImportsFromLine(line, lineNo)...)
	}
	return out
}

func extractJSImportsFromLine(line string, lineNo int) []JSImport {
	var out []JSImport
	seen := map[string]bool{}
	add := func(spec string, kind JSImportKind) {
		spec = strings.TrimSpace(spec)
		if spec == "" {
			if kind == JSImportDynamic {
				out = append(out, JSImport{Spec: "", Line: lineNo, Kind: kind})
			}
			return
		}
		if seen[spec] {
			return
		}
		seen[spec] = true
		out = append(out, JSImport{Spec: spec, Line: lineNo, Kind: kind})
	}

	for _, m := range reImportFrom.FindAllStringSubmatch(line, -1) {
		add(m[1], classifyImportSpec(m[1]))
	}
	for _, m := range reExportFrom.FindAllStringSubmatch(line, -1) {
		add(m[1], classifyImportSpec(m[1]))
	}
	for _, m := range reRequire.FindAllStringSubmatch(line, -1) {
		add(m[1], classifyImportSpec(m[1]))
	}
	for _, m := range reDynamic.FindAllStringSubmatch(line, -1) {
		add(m[1], JSImportDynamic)
	}
	if reDynamicVar.MatchString(line) && len(reDynamic.FindAllStringSubmatch(line, -1)) == 0 {
		add("", JSImportDynamic)
	}
	return out
}

func classifyImportSpec(spec string) JSImportKind {
	if strings.HasPrefix(spec, ".") {
		return JSImportRelative
	}
	return JSImportPackage
}

func stripJSComments(src string) string {
	var b strings.Builder
	b.Grow(len(src))
	inLine := false
	inBlock := false
	inStr := false
	strQuote := byte(0)
	escaped := false
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inLine {
			if c == '\n' {
				inLine = false
				b.WriteByte(c)
			}
			continue
		}
		if inBlock {
			if c == '*' && i+1 < len(src) && src[i+1] == '/' {
				inBlock = false
				i++
			}
			continue
		}
		if inStr {
			b.WriteByte(c)
			if escaped {
				escaped = false
				continue
			}
			if c == '\\' {
				escaped = true
				continue
			}
			if c == strQuote {
				inStr = false
			}
			continue
		}
		if c == '"' || c == '\'' || c == '`' {
			inStr = true
			strQuote = c
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(src) {
			switch src[i+1] {
			case '/':
				inLine = true
				i++
				continue
			case '*':
				inBlock = true
				i++
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}
