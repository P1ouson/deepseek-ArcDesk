package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const defaultGoListTimeout = 30 * time.Second

// goListRunner runs go list; tests may replace it to force parser fallback.
var goListRunner = runGoListJSON

// GoBuilder indexes Go module dependencies for a workspace root.
type GoBuilder struct {
	Root       string
	ModulePath string
	GoModFile  string
	Timeout    time.Duration
	goroot     string
	modInfo    *goModFile
}

// NewGoBuilder returns a builder when root contains go.mod.
func NewGoBuilder(root string) (*GoBuilder, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("workspace root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	goMod := filepath.Join(abs, "go.mod")
	if _, err := os.Stat(goMod); err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("go.mod not found in %s", abs)
		}
		return nil, err
	}
	modInfo, err := parseGoMod(goMod)
	if err != nil {
		return nil, fmt.Errorf("parse go.mod: %w", err)
	}
	if modInfo.Module == "" {
		return nil, fmt.Errorf("go.mod missing module directive")
	}
	b := &GoBuilder{
		Root:       abs,
		ModulePath: modInfo.Module,
		GoModFile:  goMod,
		Timeout:    defaultGoListTimeout,
		modInfo:    modInfo,
	}
	b.goroot = resolveGOROOT(abs)
	return b, nil
}

// Build returns nodes, edges, and non-fatal parse errors for the Go module.
func (b *GoBuilder) Build() (nodes []*Node, edges []Edge, parseErrors []ParseError, method BuildMethod, err error) {
	if b == nil {
		return nil, nil, nil, "", fmt.Errorf("nil GoBuilder")
	}
	nodes, edges, parseErrors, method, listErr := b.buildWithGoList()
	if listErr == nil {
		manifestNodes, manifestEdges, warns := b.manifestNodesAndEdges(method)
		nodes = append(nodes, manifestNodes...)
		edges = append(edges, manifestEdges...)
		b.attachReplaceWarnings(nodes, warns)
		return nodes, edges, parseErrors, method, nil
	}
	slog.Debug("dependency: go list failed, using parser fallback", "root", b.Root, "err", listErr)
	nodes, edges, parseErrors, method, err = b.buildWithParser()
	if err != nil {
		return nil, nil, parseErrors, method, fmt.Errorf("go list: %v; parser fallback: %w", listErr, err)
	}
	manifestNodes, manifestEdges, warns := b.manifestNodesAndEdges(method)
	nodes = append(nodes, manifestNodes...)
	edges = append(edges, manifestEdges...)
	b.attachReplaceWarnings(nodes, warns)
	for _, n := range nodes {
		if n.Meta.BuildMethod == "" {
			n.Meta.BuildMethod = string(method)
		}
	}
	return nodes, edges, parseErrors, method, nil
}

func (b *GoBuilder) buildWithGoList() ([]*Node, []Edge, []ParseError, BuildMethod, error) {
	ctx, cancel := context.WithTimeout(context.Background(), b.timeout())
	defer cancel()

	pkgs, err := goListRunner(ctx, b.Root)
	if err != nil {
		return nil, nil, nil, "", err
	}

	method := BuildGoList
	nodeByID := map[NodeID]*Node{}
	var edges []Edge
	var parseErrors []ParseError

	for _, pkg := range pkgs {
		if pkg.ImportPath == "" || pkg.ImportPath == "command-line-arguments" {
			continue
		}
		parseErrors = append(parseErrors, goListPackageErrors(b.Root, pkg)...)
		parseErrors = append(parseErrors, goListFileParseErrors(b.Root, pkg)...)
		id := NewGoID(pkg.ImportPath)
		dir := relPath(b.Root, pkg.Dir)
		node := &Node{
			ID:   id,
			Kind: KindInternalGo,
			Name: pkg.ImportPath,
			Dir:  dir,
			Manifest: ManifestRef{
				Path:    relPath(b.Root, b.GoModFile),
				Section: "module",
			},
			Meta: NodeMeta{BuildMethod: string(method)},
		}
		nodeByID[id] = node

		for _, imp := range pkg.Imports {
			if imp == "" || imp == "C" {
				continue
			}
			toID, kind := classifyImport(b.ModulePath, imp, b.goroot, false)
			b.ensureNode(nodeByID, toID, kind, nodeDisplayName(toID, kind, imp), method)
			file := firstGoFileInDir(pkg.Dir)
			if file != "" {
				file = relPath(b.Root, filepath.Join(pkg.Dir, file))
			}
			edges = append(edges, Edge{From: id, To: toID, Kind: EdgeSourceImport, Files: fileList(file)})
		}
	}

	nodes := make([]*Node, 0, len(nodeByID))
	for _, n := range nodeByID {
		nodes = append(nodes, n)
	}
	return nodes, edges, parseErrors, method, nil
}

func goListPackageErrors(root string, pkg goListPackage) []ParseError {
	var out []ParseError
	if pkg.Error != nil && strings.TrimSpace(pkg.Error.Err) != "" {
		out = append(out, parseErrorFromGoList(root, pkg.Dir, *pkg.Error))
	}
	for _, depErr := range pkg.DepsErrors {
		if strings.TrimSpace(depErr.Err) == "" {
			continue
		}
		out = append(out, parseErrorFromGoList(root, pkg.Dir, depErr))
	}
	return out
}

func goListFileParseErrors(root string, pkg goListPackage) []ParseError {
	if strings.TrimSpace(pkg.Dir) == "" || len(pkg.GoFiles) == 0 {
		return nil
	}
	var out []ParseError
	for _, name := range pkg.GoFiles {
		abs := filepath.Join(pkg.Dir, name)
		if err := checkGoFileSyntax(abs); err != nil {
			pe := ParseError{
				File:    relPath(root, abs),
				Message: err.Error(),
			}
			if _, line := splitGoListPos(err.Error()); line > 0 {
				pe.Line = line
			}
			out = append(out, pe)
		}
	}
	return out
}

func parseErrorFromGoList(root, pkgDir string, e goListError) ParseError {
	pe := ParseError{Message: strings.TrimSpace(e.Err)}
	if pe.Message == "" {
		return pe
	}
	file, line := splitGoListPos(e.Pos)
	switch {
	case file != "":
		if filepath.IsAbs(file) {
			pe.File = relPath(root, file)
		} else if strings.Contains(file, "/") || strings.Contains(file, `\`) {
			pe.File = normalizeSlash(file)
		} else {
			pe.File = relPath(root, filepath.Join(pkgDir, file))
		}
		pe.Line = line
	case pkgDir != "":
		pe.File = relPath(root, pkgDir)
	}
	return pe
}

// splitGoListPos parses go list error positions like "internal/bad/bad.go:2:8".
func splitGoListPos(pos string) (file string, line int) {
	pos = strings.TrimSpace(pos)
	if pos == "" {
		return "", 0
	}
	parts := strings.Split(pos, ":")
	if len(parts) < 2 {
		return normalizeSlash(pos), 0
	}
	// file:line:col — path may contain ":" on Windows drive letters, so take last two as line/col.
	if len(parts) >= 3 {
		file = strings.Join(parts[:len(parts)-2], ":")
	} else {
		file = parts[0]
	}
	if n, err := parseIntDecimal(parts[len(parts)-2]); err == nil {
		line = n
	}
	return normalizeSlash(file), line
}

func parseIntDecimal(s string) (int, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty")
	}
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(c-'0')
	}
	return n, nil
}

func (b *GoBuilder) buildWithParser() ([]*Node, []Edge, []ParseError, BuildMethod, error) {
	method := BuildParserFallback
	nodeByID := map[NodeID]*Node{}
	pkgFiles := map[string][]string{}
	var parseErrors []ParseError

	err := walkGoSources(b.Root, func(path string) {
		if err := checkGoFileSyntax(path); err != nil {
			parseErrors = append(parseErrors, ParseError{
				File:    relPath(b.Root, path),
				Message: err.Error(),
			})
			return
		}
		imports, impErr := ParseGoImports(path)
		if impErr != nil {
			parseErrors = append(parseErrors, ParseError{
				File:    relPath(b.Root, path),
				Message: impErr.Error(),
			})
			return
		}
		dir := filepath.Dir(path)
		importPath := inferImportPathFromDir(b.ModulePath, b.Root, dir)
		if importPath == "" {
			return
		}
		pkgFiles[importPath] = append(pkgFiles[importPath], relPath(b.Root, path))
		fromID := NewGoID(importPath)
		b.ensureNode(nodeByID, fromID, KindInternalGo, importPath, method)
		nodeByID[fromID].Dir = relPath(b.Root, dir)
		_ = imports
	})
	if err != nil {
		return nil, nil, parseErrors, method, err
	}

	var edges []Edge
	for importPath, files := range pkgFiles {
		fromID := NewGoID(importPath)
		seenImp := map[string]bool{}
		for _, file := range files {
			abs := filepath.Join(b.Root, filepath.FromSlash(file))
			imports, err := ParseGoImports(abs)
			if err != nil {
				continue
			}
			for _, imp := range imports {
				if imp == "" || seenImp[imp] {
					continue
				}
				seenImp[imp] = true
				toID, kind := classifyImport(b.ModulePath, imp, b.goroot, false)
				b.ensureNode(nodeByID, toID, kind, nodeDisplayName(toID, kind, imp), method)
				edges = append(edges, Edge{From: fromID, To: toID, Kind: EdgeSourceImport, Files: []string{file}})
			}
		}
	}

	nodes := make([]*Node, 0, len(nodeByID))
	for _, n := range nodeByID {
		n.Files = pkgFiles[n.Name]
		n.Meta.BuildMethod = string(method)
		nodes = append(nodes, n)
	}
	return nodes, edges, parseErrors, method, nil
}

func (b *GoBuilder) manifestNodesAndEdges(method BuildMethod) ([]*Node, []Edge, []string) {
	if b.modInfo == nil {
		return nil, nil, nil
	}
	modRel := relPath(b.Root, b.GoModFile)
	rootID := NewGoID(b.ModulePath)
	rootNode := &Node{
		ID:   rootID,
		Kind: KindInternalGo,
		Name: b.ModulePath,
		Dir:  ".",
		Manifest: ManifestRef{Path: modRel, Section: "module"},
		Meta: NodeMeta{BuildMethod: string(method)},
	}
	var nodes []*Node
	var edges []Edge
	var warnings []string

	nodes = append(nodes, rootNode)
	for _, req := range b.modInfo.Requires {
		id := NewGoModID(req.Path)
		nodes = append(nodes, &Node{
			ID:   id,
			Kind: KindExternalGo,
			Name: req.Path,
			Manifest: ManifestRef{Path: modRel, Section: "require"},
			Meta: NodeMeta{Version: req.Version, BuildMethod: string(method)},
		})
		edges = append(edges, Edge{From: rootID, To: id, Kind: EdgeManifestRequire})
	}
	for _, rep := range b.modInfo.Replaces {
		warnings = append(warnings, fmt.Sprintf("go.mod replace %s => %s (not fully resolved)", rep.OldPath, rep.NewPath))
	}
	return nodes, edges, warnings
}

func (b *GoBuilder) ensureNode(m map[NodeID]*Node, id NodeID, kind Kind, name string, method BuildMethod) {
	if id == "" {
		return
	}
	if _, ok := m[id]; ok {
		return
	}
	m[id] = &Node{
		ID:   id,
		Kind: kind,
		Name: name,
		Meta: NodeMeta{BuildMethod: string(method)},
	}
}

func (b *GoBuilder) timeout() time.Duration {
	if b.Timeout > 0 {
		return b.Timeout
	}
	return defaultGoListTimeout
}

type goListPackage struct {
	ImportPath string   `json:"ImportPath"`
	Imports    []string `json:"Imports"`
	Dir        string   `json:"Dir"`
	GoFiles    []string `json:"GoFiles"`
	Standard   bool     `json:"Standard"`
	Error      *goListError `json:"Error"`
	DepsErrors []goListError `json:"DepsErrors"`
}

type goListError struct {
	ImportStack []string `json:"ImportStack"`
	Pos         string   `json:"Pos"`
	Err         string   `json:"Err"`
}

func runGoListJSON(ctx context.Context, root string) ([]goListPackage, error) {
	cmd := exec.CommandContext(ctx, "go", "list", "-json", "-e", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return nil, fmt.Errorf("go command not found")
		}
		if ee, ok := err.(*exec.ExitError); ok && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("go list: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("go list: %w", err)
	}
	return decodeGoListJSON(out)
}

func decodeGoListJSON(out []byte) ([]goListPackage, error) {
	dec := json.NewDecoder(strings.NewReader(string(out)))
	var pkgs []goListPackage
	for {
		var pkg goListPackage
		if err := dec.Decode(&pkg); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("decode go list json: %w", err)
		}
		pkgs = append(pkgs, pkg)
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("go list returned no packages")
	}
	return pkgs, nil
}

func resolveGOROOT(root string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "env", "GOROOT")
	cmd.Dir = root
	out, err := cmd.Output()
	if err == nil {
		if v := strings.TrimSpace(string(out)); v != "" {
			return v
		}
	}
	return strings.TrimSpace(os.Getenv("GOROOT"))
}

// TODO(D5): share skip rules with repomap/glob via internal/walk.
var goWalkSkipDirs = map[string]bool{
	".git": true, ".arcdesk": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".wails": true,
}

func walkGoSources(root string, visit func(path string)) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if goWalkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(d.Name(), ".go") && !strings.HasSuffix(d.Name(), "_test.go") {
			visit(path)
		}
		return nil
	})
}

func relPath(root, path string) string {
	if path == "" {
		return ""
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return normalizeSlash(path)
	}
	return normalizeSlash(rel)
}

func firstGoFileInDir(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		name := e.Name()
		if strings.HasSuffix(name, ".go") && !strings.HasPrefix(name, ".") {
			return name
		}
	}
	return ""
}

func fileList(name string) []string {
	if name == "" {
		return nil
	}
	return []string{name}
}

func nodeDisplayName(id NodeID, kind Kind, importPath string) string {
	if kind == KindStdlib {
		return id.Path()
	}
	return importPath
}

func appendUniqueString(list []string, s string) []string {
	for _, v := range list {
		if v == s {
			return list
		}
	}
	return append(list, s)
}

func (b *GoBuilder) attachReplaceWarnings(nodes []*Node, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	rootID := NewGoID(b.ModulePath)
	for _, n := range nodes {
		if n.ID == rootID {
			for _, w := range warnings {
				n.Meta.Warnings = appendUniqueString(n.Meta.Warnings, w)
			}
			return
		}
	}
}
