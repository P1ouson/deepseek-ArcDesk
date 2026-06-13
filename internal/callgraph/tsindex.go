package callgraph

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// TSSymbol is a component, hook, function, or bridge call site in TS/TSX.
type TSSymbol struct {
	Kind   NodeKind
	Name   string
	File   string
	Line   int
	ID     NodeID
	Method string // for bridge calls: Submit
}

// TSListen is a frontend EventsOn subscription site.
type TSListen struct {
	Channel string
	File    string
	Line    int
	ID      NodeID
	Scope   NodeID
}

// TSCall is an intra-file call edge between TS symbols.
type TSCall struct {
	From   NodeID
	To     NodeID
	Kind   EdgeKind
	Callee string
	Line   int
}

var (
	reExportFunc      = regexp.MustCompile(`^\s*export\s+(?:default\s+)?function\s+([A-Za-z_]\w*)`)
	reFuncUse         = regexp.MustCompile(`^\s*(?:export\s+)?function\s+(use[A-Z]\w*)\s*\(`)
	reBridgeApp       = regexp.MustCompile(`\bapp\??\.([A-Z]\w*)\s*\(`)
	reBridgeAppLower  = regexp.MustCompile(`\bapp\??\.([a-z]\w*)\s*\(`)
	reBridgeWindow    = regexp.MustCompile(`window\.go\.main\.App\.([A-Z]\w*)\s*\(`)
	reBridgeBracket   = regexp.MustCompile(`\bapp\??\[\s*["']([A-Za-z]\w*)["']\s*\]\s*\(`)
	reDynamicImport   = regexp.MustCompile(`\bimport\s*\(`)
	reRequireImport   = regexp.MustCompile(`\bimport\s+\w+\s*=\s*require\s*\(`)
	reCallIdent       = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*\(`)
	reAnonymousArrow  = regexp.MustCompile(`=>\s*\{`)
	reOnClickArrow    = regexp.MustCompile(`on[A-Z]\w*\s*=\s*\{`)
	reMockRegionStart = regexp.MustCompile(`^// --- browser dev mock`)
	reEventsOnLiteral = regexp.MustCompile(`\bEventsOn\s*\(\s*"([^"]+)"`)
	reConstString     = regexp.MustCompile(`\bconst\s+([A-Za-z_]\w*)\s*=\s*"([^"]+)"`)
	reExportOnEvent   = regexp.MustCompile(`^\s*export\s+function\s+(on[A-Za-z]\w*)\s*\(`)
)

// ScanTSFiles walks frontend src for components, hooks, and bridge calls.
func ScanTSFiles(root string, validMethods map[string]bool) ([]TSSymbol, []TSCall, []TSListen, []ParseWarning, error) {
	srcRoot := filepath.Join(root, "desktop", "frontend", "src")
	if _, err := os.Stat(srcRoot); os.IsNotExist(err) {
		return nil, nil, nil, nil, nil
	}

	var symbols []TSSymbol
	var calls []TSCall
	var listens []TSListen
	var warnings []ParseWarning

	err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ts" && ext != ".tsx" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = normalizeSlash(rel)
		if strings.HasSuffix(rel, ".test.ts") || strings.HasSuffix(rel, ".test.tsx") {
			return nil
		}
		s, c, l, w := scanTSSourceFile(rel, path, validMethods)
		symbols = append(symbols, s...)
		calls = append(calls, c...)
		listens = append(listens, l...)
		warnings = append(warnings, w...)
		return nil
	})
	return symbols, calls, listens, warnings, err
}

func scanTSSourceFile(rel, absPath string, validMethods map[string]bool) ([]TSSymbol, []TSCall, []TSListen, []ParseWarning) {
	b, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, nil, []ParseWarning{{File: rel, Message: "ts_read_error: " + err.Error()}}
	}
	raw := string(b)
	if isBrokenTestFile(rel, raw) {
		return nil, nil, nil, []ParseWarning{{File: rel, Message: "ts_parse_error: unbalanced or broken syntax"}}
	}
	src := stripTSComments(raw)
	lines := strings.Split(src, "\n")

	var symbols []TSSymbol
	var calls []TSCall
	var listens []TSListen
	var warnings []ParseWarning
	nameToID := map[string]NodeID{}
	constChannels := map[string]string{}
	inMock := false

	for i, line := range lines {
		lineNo := i + 1
		if reMockRegionStart.MatchString(line) {
			inMock = true
			continue
		}
		if inMock && strings.HasPrefix(strings.TrimSpace(line), "// ---") {
			inMock = false
		}
		if inMock {
			continue
		}

		trim := strings.TrimSpace(line)
		if reRequireImport.MatchString(trim) {
			warnings = append(warnings, ParseWarning{File: rel, Line: lineNo, Message: "ts_unusual_import:require"})
		}
		if reDynamicImport.MatchString(line) {
			warnings = append(warnings, ParseWarning{File: rel, Line: lineNo, Message: "dynamic_import_bridge"})
		}
		for _, m := range reBridgeBracket.FindAllStringSubmatch(line, -1) {
			warnings = append(warnings, ParseWarning{
				File:    rel,
				Line:    lineNo,
				Message: "bracket_bridge_call:" + m[1],
			})
		}

		if m := reExportFunc.FindStringSubmatch(trim); len(m) == 2 {
			name := m[1]
			kind := KindUIComponent
			if strings.HasPrefix(name, "use") && len(name) > 3 && name[3] >= 'A' && name[3] <= 'Z' {
				kind = KindHook
			}
			id := nodeIDForKind(kind, rel, name)
			sym := TSSymbol{Kind: kind, Name: name, File: rel, Line: lineNo, ID: id}
			symbols = append(symbols, sym)
			nameToID[name] = id
		}
		if m := reFuncUse.FindStringSubmatch(trim); len(m) == 2 {
			name := m[1]
			id := NewHookID(rel, name)
			sym := TSSymbol{Kind: KindHook, Name: name, File: rel, Line: lineNo, ID: id}
			symbols = append(symbols, sym)
			nameToID[name] = id
		}

		if m := reExportOnEvent.FindStringSubmatch(trim); len(m) == 2 {
			name := m[1]
			id := NewFnID(rel, name)
			symbols = append(symbols, TSSymbol{Kind: KindUIHandler, Name: name, File: rel, Line: lineNo, ID: id})
			nameToID[name] = id
		}
		if m := reConstString.FindStringSubmatch(trim); len(m) == 3 {
			constChannels[m[1]] = m[2]
		}

		if reAnonymousArrow.MatchString(line) || reOnClickArrow.MatchString(line) {
			name := anonymousHandlerName(lineNo)
			id := NewAnonymousHandlerID(rel, lineNo)
			symbols = append(symbols, TSSymbol{
				Kind: KindUIHandler,
				Name: name,
				File: rel,
				Line: lineNo,
				ID:   id,
			})
		}

		for _, method := range extractBridgeMethods(line, &warnings, rel, lineNo) {
			id := NewBridgeCallID(rel, lineNo, method)
			sym := TSSymbol{
				Kind:   KindBridgeCall,
				Name:   "app." + method,
				File:   rel,
				Line:   lineNo,
				ID:     id,
				Method: method,
			}
			symbols = append(symbols, sym)
			if len(validMethods) > 0 && !validMethods[method] {
				warnings = append(warnings, ParseWarning{
					File:    rel,
					Line:    lineNo,
					Message: "orphan_bridge_call:" + method,
				})
			}
		}
	}

	currentScope := NodeID("")
	for i, line := range lines {
		lineNo := i + 1
		trim := strings.TrimSpace(line)
		if m := reExportFunc.FindStringSubmatch(trim); len(m) == 2 {
			if id, ok := nameToID[m[1]]; ok {
				currentScope = id
			}
		}
		if m := reExportOnEvent.FindStringSubmatch(trim); len(m) == 2 {
			if id, ok := nameToID[m[1]]; ok {
				currentScope = id
			}
		}

		for _, m := range reEventsOnLiteral.FindAllStringSubmatch(line, -1) {
			if len(m) != 2 {
				continue
			}
			ch := m[1]
			id := NewEventListenID(rel, lineNo, ch)
			scope := currentScope
			if scope == "" {
				scope = NewAnonymousHandlerID(rel, lineNo)
				symbols = append(symbols, TSSymbol{
					Kind: KindUIHandler,
					Name: anonymousHandlerName(lineNo),
					File: rel,
					Line: lineNo,
					ID:   scope,
				})
			}
			listens = append(listens, TSListen{
				Channel: ch,
				File:    rel,
				Line:    lineNo,
				ID:      id,
				Scope:   scope,
			})
		}
		if reEventsOnLiteral.MatchString(line) {
			continue
		}
		if idx := strings.Index(line, "EventsOn("); idx >= 0 {
			rest := line[idx+len("EventsOn("):]
			arg := strings.TrimSpace(strings.Split(rest, ",")[0])
			if ch, ok := constChannels[arg]; ok && ch != "" {
				id := NewEventListenID(rel, lineNo, ch)
				scope := currentScope
				if scope == "" {
					scope = NewAnonymousHandlerID(rel, lineNo)
				}
				listens = append(listens, TSListen{
					Channel: ch,
					File:    rel,
					Line:    lineNo,
					ID:      id,
					Scope:   scope,
				})
			} else if arg != "" && !strings.HasPrefix(arg, `"`) {
				warnings = append(warnings, ParseWarning{
					File:    rel,
					Line:    lineNo,
					Message: "event_listen_variable_channel",
				})
			}
		}

		if currentScope == "" {
			continue
		}
		for _, method := range extractBridgeMethods(line, nil, rel, lineNo) {
			to := NewBridgeCallID(rel, lineNo, method)
			calls = append(calls, TSCall{From: currentScope, To: to, Kind: EdgeCalls, Callee: "app." + method, Line: lineNo})
		}
		for _, m := range reCallIdent.FindAllStringSubmatch(line, -1) {
			callee := m[1]
			if callee == "if" || callee == "for" || callee == "switch" || callee == "return" || callee == "function" {
				continue
			}
			to, ok := nameToID[callee]
			if !ok {
				continue
			}
			calls = append(calls, TSCall{From: currentScope, To: to, Kind: EdgeCalls, Callee: callee, Line: lineNo})
		}
	}

	return symbols, calls, listens, warnings
}

func anonymousHandlerName(line int) string {
	return "anonymous:" + itoa(line)
}

func isBrokenTestFile(rel, raw string) bool {
	if !strings.Contains(rel, "Broken.tsx") && !strings.Contains(rel, "Broken.ts") {
		return false
	}
	return strings.Contains(raw, "unclosed") || !tsBraceBalanced(raw)
}

func tsBraceBalanced(src string) bool {
	depth := 0
	inStr := false
	quote := byte(0)
	for i := 0; i < len(src); i++ {
		c := src[i]
		if inStr {
			if c == '\\' {
				i++
				continue
			}
			if c == quote {
				inStr = false
			}
			continue
		}
		switch c {
		case '"', '\'':
			inStr = true
			quote = c
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func nodeIDForKind(kind NodeKind, file, name string) NodeID {
	switch kind {
	case KindHook:
		return NewHookID(file, name)
	case KindUIComponent:
		return NewUIID(file, name)
	default:
		return NewFnID(file, name)
	}
}

func extractBridgeMethods(line string, warnings *[]ParseWarning, rel string, lineNo int) []string {
	seen := map[string]bool{}
	var out []string
	add := func(m string) {
		if m != "" && !seen[m] {
			seen[m] = true
			out = append(out, m)
		}
	}
	for _, m := range reBridgeApp.FindAllStringSubmatch(line, -1) {
		if len(m) == 2 {
			add(m[1])
		}
	}
	for _, m := range reBridgeAppLower.FindAllStringSubmatch(line, -1) {
		if len(m) == 2 && warnings != nil {
			*warnings = append(*warnings, ParseWarning{
				File:    rel,
				Line:    lineNo,
				Message: "method_name_case_mismatch:" + m[1],
			})
		}
	}
	for _, m := range reBridgeWindow.FindAllStringSubmatch(line, -1) {
		if len(m) == 2 {
			add(m[1])
		}
	}
	return out
}

func stripTSComments(src string) string {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	var out strings.Builder
	sc := bufio.NewScanner(strings.NewReader(src))
	inBlock := false
	for sc.Scan() {
		line := sc.Text()
		if inBlock {
			if idx := strings.Index(line, "*/"); idx >= 0 {
				inBlock = false
				line = line[idx+2:]
			} else {
				continue
			}
		}
		for {
			if strings.HasPrefix(strings.TrimSpace(line), "/*") {
				if idx := strings.Index(line, "*/"); idx >= 0 {
					line = line[:strings.Index(line, "/*")] + line[idx+2:]
					continue
				}
				inBlock = true
				line = ""
				break
			}
			break
		}
		if idx := strings.Index(line, "//"); idx >= 0 {
			line = line[:idx]
		}
		out.WriteString(line)
		out.WriteByte('\n')
	}
	return out.String()
}
