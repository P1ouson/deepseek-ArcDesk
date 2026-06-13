package callgraph

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GoBindMethod is one Wails-exposed Go method on *App.
type GoBindMethod struct {
	Receiver string
	Method   string
	File     string
	Line     int
	ID       NodeID
}

// GoInternalCall is a same-file callee from a bind method body (depth 1).
type GoInternalCall struct {
	FromBind NodeID
	Name     string
	File     string
	Line     int
	ID       NodeID
}

// EventEmitSite records a runtime.EventsEmit call site.
type EventEmitSite struct {
	Channel         string
	File            string
	Line            int
	From            NodeID
	VariableChannel bool
}

var (
	reGoBind       = regexp.MustCompile(`func\s*\(\s*\w+\s+\*App\s*\)\s+([A-Z]\w*)\s*\(`)
	reGoBindOther  = regexp.MustCompile(`func\s*\(\s*\w+\s+\*(\w+)\s*\)\s+([A-Z]\w*)\s*\(`)
	reEventsEmit   = regexp.MustCompile(`EventsEmit\s*\([^,]+,\s*"([^"]+)"`)
)

// ScanGoBinds walks desktop/**/*.go for *App bind methods.
func ScanGoBinds(root string) ([]GoBindMethod, []GoInternalCall, []EventEmitSite, []ParseWarning, error) {
	root = strings.TrimSpace(root)
	desktop := filepath.Join(root, "desktop")
	if _, err := os.Stat(desktop); os.IsNotExist(err) {
		return nil, nil, nil, nil, nil
	}

	var methods []GoBindMethod
	var internals []GoInternalCall
	var emits []EventEmitSite
	var warnings []ParseWarning
	seenMethod := map[string]bool{}

	_ = filepath.WalkDir(desktop, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		rel = normalizeSlash(rel)
		m, in, em, w := scanGoFile(root, path, rel, seenMethod)
		methods = append(methods, m...)
		internals = append(internals, in...)
		emits = append(emits, em...)
		warnings = append(warnings, w...)
		return nil
	})
	return methods, internals, emits, warnings, nil
}

func scanGoFile(root, absPath, rel string, seenMethod map[string]bool) ([]GoBindMethod, []GoInternalCall, []EventEmitSite, []ParseWarning) {
	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, nil, nil, []ParseWarning{{File: rel, Message: "go_read_error: " + err.Error()}}
	}
	text := string(src)

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, absPath, src, parser.ParseComments)
	if err != nil {
		m, em, w := scanGoBindsRegex(rel, text, seenMethod)
		w = append(w, ParseWarning{File: rel, Message: "go_parse_error: " + err.Error()})
		w = append(w, ParseWarning{File: rel, Message: "go/parser fallback used"})
		return m, nil, em, w
	}

	var methods []GoBindMethod
	var internals []GoInternalCall
	var emits []EventEmitSite
	var warnings []ParseWarning

	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			fromID := NodeID("")
			if x.Recv != nil && x.Name != nil {
				recv := receiverTypeName(x.Recv)
				method := x.Name.Name
				if method != "" && method[0] >= 'A' && method[0] <= 'Z' {
					if recv == "App" {
						if seenMethod[method] {
							warnings = append(warnings, ParseWarning{
								File:    rel,
								Line:    fset.Position(x.Pos()).Line,
								Message: "duplicate_bind:" + method,
							})
						} else {
							seenMethod[method] = true
							line := fset.Position(x.Pos()).Line
							id := NewGoBindID(rel, "App."+method)
							methods = append(methods, GoBindMethod{
								Receiver: "App",
								Method:   method,
								File:     rel,
								Line:     line,
								ID:       id,
							})
							fromID = id
							if x.Body != nil {
								internals = append(internals, scanBindBodyCalls(fset, rel, id, x.Body)...)
							}
						}
					} else {
						warnings = append(warnings, ParseWarning{
							File:    rel,
							Line:    fset.Position(x.Pos()).Line,
							Message: "non_app_receiver:" + recv + "." + method,
						})
					}
				}
			}
			if fromID == "" && x.Name != nil && x.Name.Name != "" {
				fromID = NewGoInternalID(rel, x.Name.Name)
			}
			if x.Body != nil {
				ast.Inspect(x.Body, func(inner ast.Node) bool {
					call, ok := inner.(*ast.CallExpr)
					if !ok {
						return true
					}
					if emit := parseEventsEmit(fset, rel, call, fromID); emit != nil {
						emits = append(emits, *emit)
					}
					return true
				})
			}
		}
		return true
	})
	return methods, internals, emits, warnings
}

func scanGoBindsRegex(rel, text string, seenMethod map[string]bool) ([]GoBindMethod, []EventEmitSite, []ParseWarning) {
	var methods []GoBindMethod
	var emits []EventEmitSite
	var warnings []ParseWarning
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if m := reGoBind.FindStringSubmatch(line); len(m) == 2 {
			method := m[1]
			if seenMethod[method] {
				warnings = append(warnings, ParseWarning{File: rel, Line: i + 1, Message: "duplicate_bind:" + method})
				continue
			}
			seenMethod[method] = true
			id := NewGoBindID(rel, "App."+method)
			methods = append(methods, GoBindMethod{
				Receiver: "App",
				Method:   method,
				File:     rel,
				Line:    i + 1,
				ID:       id,
			})
		}
		if m := reGoBindOther.FindStringSubmatch(line); len(m) == 3 && m[1] != "App" {
			warnings = append(warnings, ParseWarning{
				File:    rel,
				Line:    i + 1,
				Message: "non_app_receiver:" + m[1] + "." + m[2],
			})
		}
		if em := reEventsEmit.FindStringSubmatch(line); len(em) == 2 {
			emits = append(emits, EventEmitSite{Channel: em[1], File: rel, Line: i + 1})
		}
	}
	return methods, emits, warnings
}

func receiverTypeName(fl *ast.FieldList) string {
	if fl == nil || len(fl.List) == 0 {
		return ""
	}
	expr := fl.List[0].Type
	switch t := expr.(type) {
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.Ident:
		return t.Name
	}
	return ""
}

func parseEventsEmit(fset *token.FileSet, rel string, call *ast.CallExpr, from NodeID) *EventEmitSite {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel == nil || sel.Sel.Name != "EventsEmit" {
		return nil
	}
	if len(call.Args) < 2 {
		return nil
	}
	line := fset.Position(call.Pos()).Line
	lit, ok := call.Args[1].(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return &EventEmitSite{
			File:            rel,
			Line:            line,
			From:            from,
			VariableChannel: true,
		}
	}
	ch := strings.Trim(lit.Value, `"`)
	if ch == "" {
		return nil
	}
	return &EventEmitSite{
		Channel: ch,
		File:    rel,
		Line:    line,
		From:    from,
	}
}

func scanBindBodyCalls(fset *token.FileSet, rel string, bindID NodeID, body *ast.BlockStmt) []GoInternalCall {
	if body == nil {
		return nil
	}
	seen := map[string]bool{}
	var out []GoInternalCall
	ast.Inspect(body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		name := callName(call)
		if name == "" || seen[name] {
			return true
		}
		seen[name] = true
		line := fset.Position(call.Pos()).Line
		out = append(out, GoInternalCall{
			FromBind: bindID,
			Name:     name,
			File:     rel,
			Line:     line,
			ID:       NewGoInternalID(rel, name),
		})
		return true
	})
	return out
}

func callName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		if fn.Sel != nil {
			return fn.Sel.Name
		}
	}
	return ""
}
