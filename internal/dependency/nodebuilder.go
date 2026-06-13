package dependency

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type jsPackage struct {
	RootRel  string
	Dir      string
	Manifest jsPackageJSON
}

type jsPackageJSON struct {
	Name             string            `json:"name"`
	Dependencies     map[string]string `json:"dependencies"`
	DevDependencies  map[string]string `json:"devDependencies"`
	PeerDependencies map[string]string `json:"peerDependencies"`
}

// NodeBuilder indexes frontend package dependencies under a workspace root.
type NodeBuilder struct {
	Root     string
	packages []jsPackage
	byName   map[string]jsPackage
}

// NewNodeBuilder returns a builder when at least one package.json exists under root.
func NewNodeBuilder(root string) (*NodeBuilder, error) {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil, fmt.Errorf("workspace root required")
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	pkgs, err := discoverJSPackages(abs)
	if err != nil {
		return nil, err
	}
	if len(pkgs) == 0 {
		return nil, fmt.Errorf("no package.json found under %s", abs)
	}
	byName := make(map[string]jsPackage, len(pkgs))
	for _, p := range pkgs {
		if p.Manifest.Name != "" {
			byName[p.Manifest.Name] = p
		}
	}
	return &NodeBuilder{Root: abs, packages: pkgs, byName: byName}, nil
}

// Build returns frontend nodes and edges.
func (b *NodeBuilder) Build() ([]*Node, []Edge, []ParseError, error) {
	if b == nil {
		return nil, nil, nil, fmt.Errorf("nil NodeBuilder")
	}

	nodeByID := map[NodeID]*Node{}
	var edges []Edge
	var parseErrors []ParseError
	warnings := map[NodeID][]string{}

	for _, pkg := range b.packages {
		b.addManifestNodes(nodeByID, pkg, &edges)
	}

	for _, pkg := range b.packages {
		srcRoot := filepath.Join(pkg.Dir, "src")
		if st, err := os.Stat(srcRoot); err != nil || !st.IsDir() {
			continue
		}
		err := filepath.WalkDir(srcRoot, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				if jsWalkSkipDirs[d.Name()] {
					return filepath.SkipDir
				}
				return nil
			}
			if !isJSFile(d.Name()) {
				return nil
			}
			imports, err := ParseJSImports(path)
			if err != nil {
				parseErrors = append(parseErrors, ParseError{
					File:    relPath(b.Root, path),
					Message: err.Error(),
				})
				return nil
			}
			fromDir := filepath.Dir(path)
			fromID := jsDirPackageID(b.Root, fromDir)
			b.ensureJSNode(nodeByID, fromID, fromDir, pkg)
			fileRel := relPath(b.Root, path)
			nodeByID[fromID].Files = appendUniqueNodeIDStrings(nodeByID[fromID].Files, []string{fileRel})

			for _, imp := range imports {
				toID, edgeKind, warn := b.resolveJSImport(fromDir, pkg, imp)
				if warn != "" {
					warnings[fromID] = appendUniqueString(warnings[fromID], warn)
				}
				if toID == "" {
					continue
				}
				b.ensureTargetNode(nodeByID, toID, imp)
				edges = append(edges, Edge{
					From:  fromID,
					To:    toID,
					Kind:  edgeKind,
					Files: []string{fileRel},
				})
			}
			return nil
		})
		if err != nil {
			return nil, nil, parseErrors, err
		}
	}

	nodes := make([]*Node, 0, len(nodeByID))
	for id, n := range nodeByID {
		n.ID = id
		for _, w := range warnings[id] {
			n.Meta.Warnings = appendUniqueString(n.Meta.Warnings, w)
		}
		nodes = append(nodes, n)
	}
	return nodes, edges, parseErrors, nil
}

func (b *NodeBuilder) resolveJSImport(fromDir string, owner jsPackage, imp JSImport) (NodeID, EdgeKind, string) {
	switch imp.Kind {
	case JSImportDynamic:
		if imp.Spec == "" {
			return "", EdgeDynamicImport, "dynamic import with non-literal target"
		}
		return b.resolveImportSpec(fromDir, owner, imp.Spec)
	case JSImportRelative:
		return b.resolveImportSpec(fromDir, owner, imp.Spec)
	case JSImportPackage:
		return b.resolvePackageSpec(owner, imp.Spec)
	default:
		return b.resolveImportSpec(fromDir, owner, imp.Spec)
	}
}

func (b *NodeBuilder) resolveImportSpec(fromDir string, owner jsPackage, spec string) (NodeID, EdgeKind, string) {
	_ = owner
	targetDir, ok := resolveRelativeJSDir(fromDir, spec)
	if !ok {
		return "", EdgeSourceImport, fmt.Sprintf("unresolved relative import %q", spec)
	}
	return jsDirPackageID(b.Root, targetDir), EdgeSourceImport, ""
}

func (b *NodeBuilder) resolvePackageSpec(owner jsPackage, spec string) (NodeID, EdgeKind, string) {
	name := npmPackageName(spec)
	if name == "" {
		return "", EdgeSourceImport, ""
	}
	if b.isWorkspaceDependency(owner, name) {
		if pkg, ok := b.byName[name]; ok {
			return jsDefaultPackageID(b.Root, pkg), EdgeSourceImport, ""
		}
		return "", EdgeSourceImport, fmt.Sprintf("workspace dependency %q has no local package.json", name)
	}
	return NewNpmID(name), EdgeSourceImport, ""
}

func (b *NodeBuilder) isWorkspaceDependency(pkg jsPackage, name string) bool {
	for _, section := range []map[string]string{
		pkg.Manifest.Dependencies,
		pkg.Manifest.DevDependencies,
		pkg.Manifest.PeerDependencies,
	} {
		if v, ok := section[name]; ok && isWorkspaceProtocol(v) {
			return true
		}
	}
	return false
}

func isWorkspaceProtocol(v string) bool {
	return strings.HasPrefix(strings.TrimSpace(v), "workspace:")
}

func (b *NodeBuilder) addManifestNodes(nodeByID map[NodeID]*Node, pkg jsPackage, edges *[]Edge) {
	rootID := jsDefaultPackageID(b.Root, pkg)
	b.ensureJSNode(nodeByID, rootID, filepath.Join(pkg.Dir, "src"), pkg)
	manifestPath := relPath(b.Root, filepath.Join(pkg.Dir, "package.json"))

	for _, section := range []struct {
		name string
		deps map[string]string
	}{
		{"dependencies", pkg.Manifest.Dependencies},
		{"devDependencies", pkg.Manifest.DevDependencies},
		{"peerDependencies", pkg.Manifest.PeerDependencies},
	} {
		for depName, version := range section.deps {
			if isWorkspaceProtocol(version) {
				if target, ok := b.byName[depName]; ok {
					toID := jsDefaultPackageID(b.Root, target)
					b.ensureJSNode(nodeByID, toID, filepath.Join(target.Dir, "src"), target)
					*edges = append(*edges, Edge{From: rootID, To: toID, Kind: EdgeWorkspaceRef})
				}
				continue
			}
			toID := NewNpmID(depName)
			if _, ok := nodeByID[toID]; !ok {
				nodeByID[toID] = &Node{
					ID:   toID,
					Kind: KindExternalNPM,
					Name: depName,
					Manifest: ManifestRef{
						Path:    manifestPath,
						Section: section.name,
					},
					Meta: NodeMeta{Version: version},
				}
			}
			*edges = append(*edges, Edge{From: rootID, To: toID, Kind: EdgeManifestRequire})
		}
	}
}

func (b *NodeBuilder) ensureJSNode(m map[NodeID]*Node, id NodeID, dir string, pkg jsPackage) {
	if _, ok := m[id]; ok {
		return
	}
	dirRel := relPath(b.Root, dir)
	m[id] = &Node{
		ID:   id,
		Kind: KindInternalJS,
		Name: pkg.Manifest.Name,
		Dir:  dirRel,
		Manifest: ManifestRef{
			Path:    relPath(b.Root, filepath.Join(pkg.Dir, "package.json")),
			Section: "package",
		},
	}
}

func (b *NodeBuilder) ensureTargetNode(m map[NodeID]*Node, id NodeID, imp JSImport) {
	if _, ok := m[id]; ok {
		return
	}
	switch id.Realm() {
	case realmNpm:
		m[id] = &Node{ID: id, Kind: KindExternalNPM, Name: id.Path()}
	case realmJS:
		name := id.Path()
		for _, pkg := range b.packages {
			if strings.HasPrefix(id.Path(), pkg.RootRel) {
				name = pkg.Manifest.Name
				break
			}
		}
		m[id] = &Node{ID: id, Kind: KindInternalJS, Name: name, Dir: id.Path()}
	default:
		return
	}
}

func discoverJSPackages(root string) ([]jsPackage, error) {
	var pkgs []jsPackage
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if jsWalkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() != "package.json" {
			return nil
		}
		pkg, err := loadJSPackage(root, path)
		if err != nil {
			return nil
		}
		pkgs = append(pkgs, pkg)
		return nil
	})
	return pkgs, err
}

func loadJSPackage(root, manifestPath string) (jsPackage, error) {
	dir := filepath.Dir(manifestPath)
	b, err := os.ReadFile(manifestPath)
	if err != nil {
		return jsPackage{}, err
	}
	var manifest jsPackageJSON
	if err := json.Unmarshal(b, &manifest); err != nil {
		return jsPackage{}, err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		rel = dir
	}
	return jsPackage{
		RootRel:  normalizeSlash(rel),
		Dir:      dir,
		Manifest: manifest,
	}, nil
}

// resolveRelativeJSDir resolves a relative import spec to the target directory package.
func resolveRelativeJSDir(fromDir, spec string) (string, bool) {
	joined := filepath.Clean(filepath.Join(fromDir, filepath.FromSlash(spec)))
	candidates := []string{joined}
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
		candidates = append(candidates, joined+ext)
	}
	for _, indexName := range []string{"index.ts", "index.tsx", "index.js", "index.jsx"} {
		candidates = append(candidates, filepath.Join(joined, indexName))
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil {
			if st.IsDir() {
				return c, true
			}
			return filepath.Dir(c), true
		}
	}
	return "", false
}

func jsDirPackageID(workspaceRoot, absDir string) NodeID {
	rel, err := filepath.Rel(workspaceRoot, absDir)
	if err != nil {
		return NewJSID(normalizeSlash(absDir))
	}
	return NewJSID(normalizeSlash(rel))
}

func jsDefaultPackageID(workspaceRoot string, pkg jsPackage) NodeID {
	src := filepath.Join(pkg.Dir, "src")
	if st, err := os.Stat(src); err == nil && st.IsDir() {
		return jsDirPackageID(workspaceRoot, src)
	}
	return jsDirPackageID(workspaceRoot, pkg.Dir)
}

func npmPackageName(spec string) string {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return ""
	}
	if strings.HasPrefix(spec, "@") {
		parts := strings.Split(spec, "/")
		if len(parts) >= 2 {
			return parts[0] + "/" + parts[1]
		}
		return spec
	}
	if i := strings.IndexByte(spec, '/'); i >= 0 {
		return spec[:i]
	}
	return spec
}

func isJSFile(name string) bool {
	lower := strings.ToLower(name)
	for _, ext := range []string{".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// TODO(D5): share with repomap/glob skip rules.
var jsWalkSkipDirs = map[string]bool{
	".git": true, ".arcdesk": true, "node_modules": true, "vendor": true,
	"dist": true, "build": true, ".wails": true, "coverage": true,
}
