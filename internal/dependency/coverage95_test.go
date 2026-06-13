package dependency

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func copyJSTestProject(t *testing.T) string {
	t.Helper()
	src, err := filepath.Abs(filepath.Join("testdata", "js_project"))
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyDir(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestFindPackageJSONNested(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "apps", "web")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "package.json"), []byte(`{"name":"web"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !findPackageJSON(root, discoverMaxDepth) {
		t.Fatal("expected nested package.json")
	}
	if findPackageJSON(root, 0) {
		t.Fatal("depth 0 should not find nested")
	}
}

func TestDiscoverableNestedJS(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "frontend")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nested, "package.json"), []byte(`{"name":"fe"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if !Discoverable(root) {
		t.Fatal("expected discoverable nested js project")
	}
}

func TestParseJSImportsFileAndExportFrom(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mod.ts")
	src := `export { x } from './peer';
import dyn from './x';
const v = import(variable);
`
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	imports, err := ParseJSImports(path)
	if err != nil {
		t.Fatal(err)
	}
	found := map[string]bool{}
	for _, imp := range imports {
		found[imp.Spec+"|"+string(imp.Kind)] = true
	}
	if !found["./peer|relative"] || !found["|dynamic"] {
		t.Fatalf("imports = %+v", imports)
	}
}

func TestComputeImpactNilGraph(t *testing.T) {
	computeImpact(nil)
}

func TestAffectedByWithAnalyzerUnavailable(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a", Meta: NodeMeta{BridgeMethod: "Submit"}})
	res, err := AffectedByWithAnalyzer(g, id, mockBridgeAnalyzer{ok: false})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.CrossRealm) != 0 {
		t.Fatalf("CrossRealm = %+v", res.CrossRealm)
	}
}

func TestCrossRealmEntriesAnalyzerError(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a", Meta: NodeMeta{BridgeMethod: "Submit"}})
	entries := crossRealmEntries(g, id, mockBridgeAnalyzer{ok: true, err: errors.New("fail")})
	if len(entries) != 0 {
		t.Fatalf("entries = %+v", entries)
	}
}

func TestBridgeMethodsForNodeNil(t *testing.T) {
	if methods := bridgeMethodsForNode(NewGraph("/tmp"), NewGoID("missing")); methods != nil {
		t.Fatalf("methods = %v", methods)
	}
}

func TestResolveIDPrefixMatch(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/pkg")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "pkg", Dir: "internal/pkg"})
	got, err := resolveID(g, "internal/pkg/extra")
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIndexNilRootAndNotReadyHelpers(t *testing.T) {
	var idx *Index
	if idx.Root() != "" {
		t.Fatal("nil Root")
	}
	idx = &Index{root: t.TempDir()}
	if name := idx.NodeName(NewGoID("missing")); name == "" {
		t.Fatal("expected fallback name")
	}
	if kind := idx.NodeKind(NewGoID("missing")); kind != "" {
		t.Fatalf("kind = %q", kind)
	}
}

func TestBuildNodeSectionSuccess(t *testing.T) {
	root := copyJSTestProject(t)
	nodes, edges, errs, err := buildNodeSection(root)
	if err != nil {
		t.Fatal(err)
	}
	if nodes == nil && edges == nil && len(errs) == 0 {
		t.Fatal("expected node section output")
	}
}

func TestMergeBuildResultNilGraph(t *testing.T) {
	mergeBuildResult(nil, []*Node{{ID: NewGoID("a")}}, nil, nil)
}

func TestFindEdgeHelper(t *testing.T) {
	a, b := NewGoID("a"), NewGoID("b")
	edges := []Edge{{From: a, To: b, Kind: EdgeSourceImport}}
	if _, ok := findEdge(edges, a, b, EdgeSourceImport); !ok {
		t.Fatal("expected edge")
	}
	if _, ok := findEdge(edges, a, b, EdgeManifestRequire); ok {
		t.Fatal("unexpected edge kind match")
	}
}

func TestAtomicWriteSuccessAndRenameFailure(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ok.json")
	if err := atomicWrite(path, []byte("{}")); err != nil {
		t.Fatal(err)
	}
	block := filepath.Join(dir, "block.json")
	if err := os.Mkdir(block, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := atomicWrite(block, []byte("{}")); err == nil {
		t.Fatal("expected rename error")
	}
}

func TestLoadIndexMissingAndCorrupt(t *testing.T) {
	if _, err := LoadIndex(t.TempDir()); !errors.Is(err, ErrIndexNotFound) {
		t.Fatalf("err = %v", err)
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndex(dir); !errors.Is(err, ErrIndexCorrupt) {
		t.Fatalf("err = %v", err)
	}
}

func TestGraphRemoveNodeFull(t *testing.T) {
	g := NewGraph("/tmp")
	a, b, c := NewGoID("a"), NewGoID("b"), NewGoID("c")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddNode(&Node{ID: c, Kind: KindInternalGo, Name: "c"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(b, c, EdgeSourceImport, "b.go")
	g.RemoveNode(b)
	if _, ok := g.Nodes[b]; ok {
		t.Fatal("node b should be removed")
	}
	if len(g.ImportedBy(c)) != 0 {
		t.Fatal("edge to c should be removed")
	}
}

func TestGraphAddNodeRecreatesNodesMap(t *testing.T) {
	g := &Graph{}
	g.AddNode(&Node{ID: NewGoID("a"), Kind: KindInternalGo, Name: "a"})
	if g.Nodes == nil {
		t.Fatal("expected nodes map")
	}
}

func TestParseRequireLineEdgeCases(t *testing.T) {
	req := parseRequireLine("github.com/foo/bar v1.2.3 // indirect")
	if req.Path != "github.com/foo/bar" || req.Version != "v1.2.3" {
		t.Fatalf("req = %+v", req)
	}
	if rep := parseRequireLine(""); rep.Path != "" {
		t.Fatalf("empty require = %+v", rep)
	}
}

func TestParseReplaceLineWithPath(t *testing.T) {
	rep := parseReplaceLine("old/path => new/path v1.0.0")
	if rep.OldPath != "old/path" || !strings.Contains(rep.NewPath, "new/path") {
		t.Fatalf("rep = %+v", rep)
	}
}

func TestDepImportsDefaultDirection(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	importsTool, _ := reg.Get("dependency_imports")
	out, err := importsTool.Execute(context.Background(), json.RawMessage(`{"path":"internal/gamma/gamma.go"}`))
	if err != nil || !strings.Contains(out, "imports") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestDepAffectedByToolSuccess(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	affectedTool, _ := reg.Get("dependency_affected_by")
	out, err := affectedTool.Execute(context.Background(), json.RawMessage(`{"path":"internal/gamma/gamma.go"}`))
	if err != nil || !strings.Contains(out, "Impact for") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestNewNodeBuilderEmptyRoot(t *testing.T) {
	if _, err := NewNodeBuilder(""); err == nil {
		t.Fatal("expected error")
	}
}

func TestNewGoBuilderEmptyRoot(t *testing.T) {
	if _, err := NewGoBuilder("  "); err == nil {
		t.Fatal("expected error")
	}
}

func TestEntriesForSkipsMissingNode(t *testing.T) {
	g := NewGraph("/tmp")
	got := entriesFor(g, []NodeID{NewGoID("missing")}, 1)
	if len(got) != 0 {
		t.Fatalf("got = %+v", got)
	}
}

func TestComputeImpactPopulatesImpactMap(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/internal")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "internal"})
	computeImpact(g)
	if _, ok := g.Impact[id]; !ok {
		t.Fatal("expected impact entry")
	}
}

func TestIndexAffectedByPathNotFound(t *testing.T) {
	idx := readyTestIndex(t)
	if _, err := idx.AffectedByPath("no/such/path.go"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestIndexImportsOfNotFound(t *testing.T) {
	idx := readyTestIndex(t)
	if _, err := idx.ImportsOf(NewGoID("missing")); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveIDByNodeIDString(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/single")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "single"})
	got, err := resolveID(g, string(id))
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestBuildGraphEmptyNodesReturnsMeta(t *testing.T) {
	root := t.TempDir()
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if g == nil || meta == nil {
		t.Fatal("expected graph and meta")
	}
}

func TestSourceImportImportsExternal(t *testing.T) {
	g := NewGraph("/tmp")
	intID := NewGoID("example.com/internal")
	extID := NewGoModID("github.com/x")
	g.AddNode(&Node{ID: intID, Kind: KindInternalGo, Name: "internal"})
	g.AddNode(&Node{ID: extID, Kind: KindExternalGo, Name: "github.com/x"})
	g.AddEdge(intID, extID, EdgeSourceImport, "a.go")
	imports := sourceImportImports(g, intID)
	if len(imports) != 1 || imports[0] != extID {
		t.Fatalf("imports = %v", imports)
	}
}

func TestRemoveIDHelper(t *testing.T) {
	list := []NodeID{"a", "b", "c"}
	got := removeID(list, "b")
	if len(got) != 2 || got[0] != "a" || got[1] != "c" {
		t.Fatalf("got = %v", got)
	}
}

func TestIsWorkspaceDependency(t *testing.T) {
	b := &NodeBuilder{Root: t.TempDir()}
	owner := jsPackage{Manifest: jsPackageJSON{
		Dependencies: map[string]string{"@scope/pkg": "workspace:*"},
	}}
	if !b.isWorkspaceDependency(owner, "@scope/pkg") {
		t.Fatal("expected workspace dependency")
	}
	if b.isWorkspaceDependency(owner, "lodash") {
		t.Fatal("lodash should not be workspace")
	}
}

func TestNpmPackageNameScoped(t *testing.T) {
	if got := npmPackageName("@scope/pkg/extra"); got != "@scope/pkg" {
		t.Fatalf("got=%q", got)
	}
	if got := npmPackageName("lodash/deep"); got != "lodash" {
		t.Fatalf("got=%q", got)
	}
}

func TestRunGoListJSONOnProject(t *testing.T) {
	root := copyGoTestProject(t)
	pkgs, err := runGoListJSON(context.Background(), root)
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatal("expected packages from go list")
	}
}

func TestGoListRunnerInjectedSuccess(t *testing.T) {
	root := copyGoTestProject(t)
	orig := goListRunner
	goListRunner = func(context.Context, string) ([]goListPackage, error) {
		return []goListPackage{{
			ImportPath: "example.com/testproj",
			Dir:        root,
			GoFiles:    []string{"internal/alpha/alpha.go"},
		}}, nil
	}
	t.Cleanup(func() { goListRunner = orig })
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, method, err := b.Build(); err != nil || method != BuildGoList {
		t.Fatalf("method=%q err=%v", method, err)
	}
}

func TestBuildNodeSectionNoPackageJSON(t *testing.T) {
	nodes, edges, errs, err := buildNodeSection(t.TempDir())
	if err == nil {
		t.Fatal("expected error without package.json")
	}
	if nodes != nil || edges != nil || errs != nil {
		t.Fatalf("nodes=%v edges=%v errs=%v", nodes, edges, errs)
	}
}

func TestMergeNodeFields(t *testing.T) {
	dst := &Node{ID: NewGoID("a"), Kind: KindInternalGo, Name: "a"}
	src := &Node{
		ID: NewGoID("a"), Kind: KindInternalGo, Name: "merged",
		Dir: "pkg/a", Files: []string{"a.go"},
		Meta: NodeMeta{Version: "1.0", BuildMethod: "test", Warnings: []string{"w"}},
	}
	mergeNode(dst, src)
	if dst.Dir != "pkg/a" || dst.Name != "merged" || dst.Meta.Version != "1.0" {
		t.Fatalf("dst=%+v", dst)
	}
	mergeNode(nil, src)
	mergeNode(dst, nil)
}

func TestComputeImpactPreexistingMap(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a"})
	g.Impact = map[NodeID]ImpactLayers{}
	computeImpact(g)
	if _, ok := g.Impact[id]; !ok {
		t.Fatal("expected impact entry")
	}
}

func TestAffectedByWithoutPrecomputedImpact(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("leaf")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "leaf"})
	res, err := AffectedBy(g, id)
	if err != nil || res.Hint == "" {
		t.Fatalf("res=%+v err=%v", res, err)
	}
}

func TestModuleCatalogResolveGoFile(t *testing.T) {
	root := copyGoTestProject(t)
	idx, _ := Open(root)
	_ = idx.EnsureReady(context.Background())
	cat := idx.ModuleCatalog()
	mod, ok := cat.ResolveFile("internal/alpha/alpha.go")
	if !ok {
		t.Fatal("expected resolve")
	}
	kind, ok := cat.ModuleKind(mod)
	if !ok || kind != "go" {
		t.Fatalf("kind=%q ok=%v", kind, ok)
	}
}

func TestInferImportPathFromDirCoverage(t *testing.T) {
	root := t.TempDir()
	if got := inferImportPathFromDir("example.com/mod", root, root); got != "example.com/mod" {
		t.Fatalf("root import = %q", got)
	}
	sub := filepath.Join(root, "internal", "pkg")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if got := inferImportPathFromDir("example.com/mod", root, sub); got != "example.com/mod/internal/pkg" {
		t.Fatalf("sub import = %q", got)
	}
}

func TestParseGoImportsFile(t *testing.T) {
	root := copyGoTestProject(t)
	path := filepath.Join(root, "internal", "alpha", "alpha.go")
	imports, err := ParseGoImports(path)
	if err != nil || len(imports) == 0 {
		t.Fatalf("imports=%v err=%v", imports, err)
	}
}

func TestSaveMetaLoadMetaRoundTrip(t *testing.T) {
	dir := t.TempDir()
	meta := &Meta{IndexVersion: IndexVersion, Fingerprint: "abc"}
	if err := SaveMeta(meta, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadMeta(dir)
	if err != nil || loaded.Fingerprint != "abc" {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestLoadMetaMissing(t *testing.T) {
	if _, err := LoadMeta(t.TempDir()); !errors.Is(err, ErrIndexNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestResolveIDMultiplePrefixAmbiguous(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a", Dir: "internal/a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b", Dir: "internal/b"})
	if _, err := resolveID(g, "internal"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestNodeBuilderResolvePackageSpec(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	id, kind, warn := b.resolvePackageSpec(owner, "lodash/map")
	if id == "" || kind != EdgeSourceImport || warn != "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestNodeBuilderResolveImportSpec(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	libDir := filepath.Join(owner.Dir, "src", "lib")
	id, kind, warn := b.resolveImportSpec(libDir, owner, "./app")
	if id == "" || kind != EdgeSourceImport || warn != "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestComputeFingerprintDependency(t *testing.T) {
	root := copyGoTestProject(t)
	if fp := ComputeFingerprint(root); fp == "" {
		t.Fatal("expected fingerprint")
	}
}

func TestMetaCheckStaleGitHead(t *testing.T) {
	root := copyGoTestProject(t)
	_, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	meta.GitHead = "deadbeef"
	if gitHead(root) != "" && !CheckStale(root, meta) {
		t.Fatal("expected stale when git head differs")
	}
}

func TestDepCyclesToolEmpty(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	cyclesTool, _ := reg.Get("dependency_cycles")
	out, err := cyclesTool.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency cycles") {
		t.Fatalf("out=%q", out)
	}
}

func TestImpactBridgeMethodsInternalGo(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/desktop")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "desktop", Meta: NodeMeta{BridgeMethod: "Submit"}})
	methods := bridgeMethodsForNode(g, id)
	if len(methods) != 1 || methods[0] != "Submit" {
		t.Fatalf("methods=%v", methods)
	}
}

func TestOpenIndexWithoutMeta(t *testing.T) {
	root := copyGoTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	_ = os.Remove(filepath.Join(dir, metaFileName))
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestResolveJSImportDynamicEmptySpec(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	libDir := filepath.Join(owner.Dir, "src", "lib")
	id, kind, warn := b.resolveJSImport(libDir, owner, JSImport{Kind: JSImportDynamic, Spec: ""})
	if id != "" || kind != EdgeDynamicImport || warn == "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestResolvePackageSpecWorkspaceMissing(t *testing.T) {
	b := &NodeBuilder{
		Root:   t.TempDir(),
		byName: map[string]jsPackage{},
	}
	owner := jsPackage{Manifest: jsPackageJSON{
		Dependencies: map[string]string{"@scope/missing": "workspace:*"},
	}}
	id, _, warn := b.resolvePackageSpec(owner, "@scope/missing")
	if id != "" || !strings.Contains(warn, "workspace dependency") {
		t.Fatalf("id=%q warn=%q", id, warn)
	}
}

func TestResolvePackageSpecEmptyName(t *testing.T) {
	b := &NodeBuilder{Root: t.TempDir()}
	id, _, warn := b.resolvePackageSpec(jsPackage{}, "  ")
	if id != "" || warn != "" {
		t.Fatalf("id=%q warn=%q", id, warn)
	}
}

func TestLoadJSPackageCorrupt(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join(dir, "package.json")
	if err := os.WriteFile(manifest, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadJSPackage(dir, manifest); err == nil {
		t.Fatal("expected corrupt package error")
	}
}

func TestJsDirPackageIDOutsideRoot(t *testing.T) {
	id := jsDirPackageID(t.TempDir(), filepath.Join(t.TempDir(), "..", "outside"))
	if id == "" {
		t.Fatal("expected fallback id")
	}
}

func TestRunGoListJSONInvalidModule(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module broken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := runGoListJSON(context.Background(), root); err == nil {
		t.Fatal("expected go list error")
	}
}

func TestIndexSetBridgeAnalyzer(t *testing.T) {
	idx := readyTestIndex(t)
	idx.SetBridgeImpactAnalyzer(mockBridgeAnalyzer{ok: true, entries: []CrossRealmImpactEntry{{Name: "UI"}}})
	res, err := idx.AffectedBy(NewGoID("example.com/testproj/internal/gamma"))
	if err != nil {
		t.Fatal(err)
	}
	_ = res
}

func TestResolveIDFromFilesMap(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a"})
	g.Files["pkg/a.go"] = id
	got, err := resolveID(g, "pkg/a.go")
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestParseJSImportsMissingFile(t *testing.T) {
	if _, err := ParseJSImports(filepath.Join(t.TempDir(), "missing.ts")); err == nil {
		t.Fatal("expected read error")
	}
}

func TestNodeBuilderBuildFullProject(t *testing.T) {
	b, err := NewNodeBuilder(copyJSTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, err := b.Build()
	if err != nil || len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d err=%v", len(nodes), len(edges), err)
	}
}

func TestResolveIDByNameMatch(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/single")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "unique-name"})
	got, err := resolveID(g, "unique-name")
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIndexResolveIDSuccess(t *testing.T) {
	idx := readyTestIndex(t)
	id, err := idx.ResolveID("internal/gamma/gamma.go")
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected id")
	}
}

func TestAffectedByWithAnalyzerNilGraph(t *testing.T) {
	if _, err := AffectedByWithAnalyzer(nil, NewGoID("a"), nil); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("err=%v", err)
	}
}

func TestDepImportsToolDefaultDirection(t *testing.T) {
	idx := readyTestIndex(t)
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	importsTool, _ := reg.Get("dependency_imports")
	out, err := importsTool.Execute(context.Background(), json.RawMessage(`{"path":"internal/gamma/gamma.go","direction":"imports"}`))
	if err != nil || !strings.Contains(out, "imports") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestEnsureTargetNodeNpmAndJS(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	m := map[NodeID]*Node{}
	npmID := NewNpmID("lodash")
	b.ensureTargetNode(m, npmID, JSImport{Spec: "lodash", Kind: JSImportPackage})
	if m[npmID] == nil || m[npmID].Kind != KindExternalNPM {
		t.Fatalf("npm node=%+v", m[npmID])
	}
	jsID := NewJSID("frontend/src")
	b.ensureTargetNode(m, jsID, JSImport{Spec: "./app", Kind: JSImportRelative})
	if m[jsID] == nil || m[jsID].Kind != KindInternalJS {
		t.Fatalf("js node=%+v", m[jsID])
	}
}

func TestBuildFailureContextVerifyCommand(t *testing.T) {
	idx := readyTestIndex(t)
	out := BuildFailureContext(idx, []string{"internal/gamma/gamma.go"}, "go test ./...", "compile error")
	if out == "" || !strings.Contains(out, "## Dependency Impact") {
		t.Fatalf("out=%q", out)
	}
}

func TestBuildFailureContextNilIndexCoverage95(t *testing.T) {
	if got := BuildFailureContext(nil, []string{"a.go"}, "go test", ""); got != "dependency index unavailable" {
		t.Fatalf("got=%q", got)
	}
}

func TestWalkGoSourcesProject(t *testing.T) {
	root := copyGoTestProject(t)
	var count int
	if err := walkGoSources(root, func(string) { count++ }); err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("expected go source files")
	}
}

func TestResolveImportSpecUnresolved(t *testing.T) {
	b, err := NewNodeBuilder(copyJSTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	id, kind, warn := b.resolveImportSpec(filepath.Join(owner.Dir, "src"), owner, "./missing/module")
	if id != "" || kind != EdgeSourceImport || warn == "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestModuleKindFromNodeBridge(t *testing.T) {
	if kind, ok := moduleKindFromNode(&Node{Kind: KindBridge, Name: "Submit"}); !ok || kind != "bridge" {
		t.Fatalf("kind=%q ok=%v", kind, ok)
	}
	if _, ok := moduleKindFromNode(&Node{Kind: Kind("unknown")}); ok {
		t.Fatal("unknown kind should fail")
	}
}

func TestCatalogResolveFileKnownPath(t *testing.T) {
	idx := readyTestIndex(t)
	cat := idx.ModuleCatalog()
	mod, ok := cat.ResolveFile("internal/alpha/alpha.go")
	if !ok || mod == "" {
		t.Fatalf("mod=%q ok=%v", mod, ok)
	}
	kind, ok := cat.ModuleKind(mod)
	if !ok || kind != "go" {
		t.Fatalf("kind=%q ok=%v", kind, ok)
	}
}

func TestGoBuilderBuildWithParserFallback(t *testing.T) {
	root := copyGoTestProject(t)
	orig := goListRunner
	goListRunner = func(context.Context, string) ([]goListPackage, error) {
		return nil, errors.New("forced fallback")
	}
	t.Cleanup(func() { goListRunner = orig })
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, method, err := b.Build()
	if err != nil || method != BuildParserFallback || len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d method=%q err=%v", len(nodes), len(edges), method, err)
	}
}

func TestSaveLoadIndexWithImpact(t *testing.T) {
	root := copyGoTestProject(t)
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil || len(loaded.Impact) == 0 {
		t.Fatalf("impact=%d err=%v", len(loaded.Impact), err)
	}
}

func TestResolveJSImportPackageKind(t *testing.T) {
	b, err := NewNodeBuilder(copyJSTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	libDir := filepath.Join(owner.Dir, "src", "lib")
	id, kind, warn := b.resolveJSImport(libDir, owner, JSImport{Spec: "react", Kind: JSImportPackage})
	if id == "" || kind != EdgeSourceImport || warn != "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestIndexMethodsNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	if _, err := idx.Status(); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("Status err=%v", err)
	}
	if _, err := idx.AffectedByPath("a.go"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("AffectedByPath err=%v", err)
	}
	if _, err := idx.ImportsOf(NewGoID("a")); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("ImportsOf err=%v", err)
	}
	if _, err := idx.ImportedBy(NewGoID("a")); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("ImportedBy err=%v", err)
	}
	if _, err := idx.FindCycles(); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("FindCycles err=%v", err)
	}
	if _, err := idx.VersionConflicts(); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("VersionConflicts err=%v", err)
	}
	if _, err := idx.ResolveID("a.go"); !errors.Is(err, ErrIndexNotReady) {
		t.Fatalf("ResolveID err=%v", err)
	}
}

func TestCatalogAdapterGraphNil(t *testing.T) {
	cat := (&Index{root: t.TempDir()}).ModuleCatalog()
	if mod, ok := cat.ResolveFile("internal/alpha/alpha.go"); ok || mod != "" {
		t.Fatalf("mod=%q ok=%v", mod, ok)
	}
	if kind, ok := cat.ModuleKind("go:x"); ok || kind != "" {
		t.Fatalf("kind=%q ok=%v", kind, ok)
	}
	if n, e, m := cat.Status(); n != 0 || e != 0 || m != "" {
		t.Fatalf("status=%d,%d,%q", n, e, m)
	}
}

func TestBuildGraphAbsPathError(t *testing.T) {
	if _, _, err := BuildGraph(BuildOptions{Root: string([]byte{0})}); err == nil {
		t.Fatal("expected abs path error")
	}
}

func TestDiscoverReadDirError(t *testing.T) {
	if findPackageJSON(string([]byte{0}), 1) {
		t.Fatal("expected false for invalid dir")
	}
}

func TestImpactCrossRealmSkipDuplicate(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/a")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "a", Meta: NodeMeta{BridgeMethod: "Submit"}})
	entries := crossRealmEntries(g, id, mockBridgeAnalyzer{
		ok: true,
		entries: []CrossRealmImpactEntry{
			{ID: "ui:1", Name: "A", Kind: "ui_component"},
			{ID: "ui:1", Name: "A", Kind: "ui_component"},
		},
	})
	if len(entries) != 1 {
		t.Fatalf("entries=%v", entries)
	}
}

func TestComputeImpactNilImpactMapInit(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("leaf")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "leaf"})
	computeImpact(g)
	if g.Impact == nil {
		t.Fatal("expected impact map init")
	}
}

func TestResolveIDPrefixAmbiguousMultiple(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a", Dir: "internal/a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b", Dir: "internal/b"})
	if _, err := resolveID(g, "internal/shared/extra"); !errors.Is(err, ErrNodeNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestNodeBuilderResolvePackageWorkspaceHit(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	var owner jsPackage
	for _, p := range b.packages {
		if strings.Contains(p.RootRel, "frontend") {
			owner = p
			break
		}
	}
	owner.Manifest.Dependencies = map[string]string{"@shared/pkg": "workspace:*"}
	b.byName["@shared/pkg"] = jsPackage{
		RootRel:  "packages/shared",
		Dir:      filepath.Join(root, "packages", "shared"),
		Manifest: jsPackageJSON{Name: "@shared/pkg"},
	}
	id, kind, warn := b.resolvePackageSpec(owner, "@shared/pkg")
	if id == "" || kind != EdgeSourceImport || warn != "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestIndexRefreshInvalidateCycle(t *testing.T) {
	idx := readyTestIndex(t)
	_ = idx.InvalidateFiles([]string{"internal/alpha/alpha.go"})
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	stats, err := idx.Status()
	if err != nil || stats.Stale {
		t.Fatalf("stats=%+v err=%v", stats, err)
	}
}

func TestBuildGraphEmptyRootDependency(t *testing.T) {
	if _, _, err := BuildGraph(BuildOptions{Root: ""}); err == nil {
		t.Fatal("expected error")
	}
}

func TestParseGoModFile(t *testing.T) {
	root := copyGoTestProject(t)
	info, err := parseGoMod(filepath.Join(root, "go.mod"))
	if err != nil || info.Module == "" {
		t.Fatalf("info=%+v err=%v", info, err)
	}
}

func TestGoBuilderBuildWithGoListDirect(t *testing.T) {
	root := copyGoTestProject(t)
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, method, err := b.buildWithGoList()
	if err != nil || method != BuildGoList || len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d method=%q err=%v", len(nodes), len(edges), method, err)
	}
}

func TestGoBuilderManifestNodesAndEdges(t *testing.T) {
	root := copyGoTestProject(t)
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _, method, err := b.buildWithGoList()
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, warns := b.manifestNodesAndEdges(method)
	if len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d warns=%v", len(nodes), len(edges), warns)
	}
}

func TestGoBuilderRelPathAndFirstGoFile(t *testing.T) {
	root := copyGoTestProject(t)
	if got := relPath(root, filepath.Join(root, "internal", "alpha", "alpha.go")); got != "internal/alpha/alpha.go" {
		t.Fatalf("relPath=%q", got)
	}
	if got := firstGoFileInDir(filepath.Join(root, "internal", "alpha")); got == "" {
		t.Fatal("expected first go file")
	}
}

func TestClassifyImportExternal(t *testing.T) {
	root := copyGoTestProject(t)
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	id, kind := classifyImport(b.ModulePath, "github.com/stretchr/testify/assert", resolveGOROOT(root), false)
	if kind != KindExternalGo || id == "" {
		t.Fatalf("kind=%q id=%q", kind, id)
	}
}

func TestParseJSImportsDynamicVariable(t *testing.T) {
	imports := ParseJSImportsSource(`const m = import(variable);`)
	if len(imports) != 1 || imports[0].Kind != JSImportDynamic || imports[0].Spec != "" {
		t.Fatalf("imports=%+v", imports)
	}
}

func TestResolveIDByFileMapAndGoID(t *testing.T) {
	g := NewGraph("/tmp")
	id := NewGoID("example.com/testproj/internal/gamma")
	g.AddNode(&Node{ID: id, Kind: KindInternalGo, Name: "gamma", Dir: "internal/gamma"})
	g.Files["internal/gamma/gamma.go"] = id
	got, err := resolveID(g, "example.com/testproj/internal/gamma")
	if err != nil || got != id {
		t.Fatalf("got=%q err=%v", got, err)
	}
}

func TestIndexAffectedByDirect(t *testing.T) {
	idx := readyTestIndex(t)
	res, err := idx.AffectedBy(NewGoID("example.com/testproj/internal/gamma"))
	if err != nil {
		t.Fatal(err)
	}
	if res.Source == "" {
		t.Fatalf("res=%+v", res)
	}
}

func TestIndexImportedByDirect(t *testing.T) {
	idx := readyTestIndex(t)
	id, _ := idx.ResolveID("internal/gamma/gamma.go")
	neighbors, err := idx.ImportedBy(id)
	if err != nil {
		t.Fatal(err)
	}
	_ = neighbors
}

func TestComputeFingerprintDependencyProject(t *testing.T) {
	root := copyGoTestProject(t)
	if fp := ComputeFingerprint(root); fp == "" {
		t.Fatal("expected fingerprint")
	}
}

func TestCheckStaleMatchingFingerprint(t *testing.T) {
	root := copyGoTestProject(t)
	_, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if CheckStale(root, meta) {
		t.Fatal("fresh meta should not be stale")
	}
}

func TestForwardExternalImportsChain(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	ext := NewGoModID("github.com/x")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddNode(&Node{ID: ext, Kind: KindExternalGo, Name: "github.com/x"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(b, ext, EdgeSourceImport, "b.go")
	external := forwardExternalImports(g, a)
	if len(external) != 1 || external[0] != ext {
		t.Fatalf("external=%v", external)
	}
}

func TestBuildNodeSectionSuccessPath(t *testing.T) {
	nodes, edges, errs, err := buildNodeSection(copyJSTestProject(t))
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d errs=%v", len(nodes), len(edges), errs)
	}
}

func TestDepStatusToolNotReady(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(reg, &Index{root: t.TempDir()})
	statusTool, _ := reg.Get("dependency_status")
	if _, err := statusTool.Execute(context.Background(), nil); err == nil {
		t.Fatal("expected not ready error")
	}
}

func TestComputeImpactTransitiveLayers(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	c := NewGoID("example.com/c")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddNode(&Node{ID: c, Kind: KindInternalGo, Name: "c"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(b, c, EdgeSourceImport, "b.go")
	computeImpact(g)
	layers := g.Impact[c]
	if len(layers.Direct) != 1 || len(layers.Transitive) == 0 || layers.Transitive[0][0] != a {
		t.Fatalf("layers=%+v", layers)
	}
}

func TestReverseInternalImportersSkipsExternal(t *testing.T) {
	g := NewGraph("/tmp")
	intID := NewGoID("example.com/internal")
	extID := NewGoModID("github.com/x")
	g.AddNode(&Node{ID: intID, Kind: KindInternalGo, Name: "internal"})
	g.AddNode(&Node{ID: extID, Kind: KindExternalGo, Name: "github.com/x"})
	g.AddEdge(extID, intID, EdgeSourceImport, "x.go")
	direct, trans := reverseInternalImporters(g, intID)
	if len(direct) != 0 || len(trans) != 0 {
		t.Fatalf("direct=%v trans=%v", direct, trans)
	}
}

func TestOpenCorruptIndexDependency(t *testing.T) {
	root := copyGoTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("expected corrupt index error")
	}
}

func TestLoadIndexMissingMetaFields(t *testing.T) {
	dir := t.TempDir()
	g := NewGraph(dir)
	g.AddNode(&Node{ID: NewGoID("a"), Kind: KindInternalGo, Name: "a"})
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadIndex(dir)
	if err != nil || len(loaded.Nodes) != 1 {
		t.Fatalf("loaded=%+v err=%v", loaded, err)
	}
}

func TestResolveIDMultipleNameMatches(t *testing.T) {
	g := NewGraph("/tmp")
	a := NewGoID("example.com/a")
	b := NewGoID("example.com/b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "same"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "same"})
	if _, err := resolveID(g, "same"); err == nil {
		t.Fatal("expected ambiguous error")
	}
}

func TestNodeBuilderBuildDynamicImportResolved(t *testing.T) {
	root := copyJSTestProject(t)
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	owner := b.packages[0]
	libDir := filepath.Join(owner.Dir, "src", "lib")
	id, kind, warn := b.resolveJSImport(libDir, owner, JSImport{Spec: "./app", Kind: JSImportDynamic})
	if id == "" || kind != EdgeSourceImport || warn != "" {
		t.Fatalf("id=%q kind=%q warn=%q", id, kind, warn)
	}
}

func TestGoProjectFullSurface(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	gamma, err := idx.ResolveID("internal/gamma/gamma.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, fn := range []func() error{
		func() error { _, err := idx.AffectedBy(gamma); return err },
		func() error { _, err := idx.AffectedByPath("internal/gamma/gamma.go"); return err },
		func() error { _, err := idx.ImportsOf(gamma); return err },
		func() error { _, err := idx.ImportedBy(gamma); return err },
		func() error { _, err := idx.FindCycles(); return err },
		func() error { _, err := idx.VersionConflicts(); return err },
	} {
		if err := fn(); err != nil {
			t.Fatal(err)
		}
	}
	out := BuildFailureContext(idx, []string{"internal/gamma/gamma.go", "internal/alpha/alpha.go"}, "go test ./...", "build failed")
	if out == "" {
		t.Fatal("expected failure context")
	}
	_ = idx.InvalidateFiles([]string{"internal/gamma/gamma.go"})
	if err := idx.EnsureReady(ctx); err != nil {
		t.Fatal(err)
	}
	reg := tool.NewRegistry()
	RegisterTools(reg, idx)
	for _, name := range []string{"dependency_status", "dependency_affected_by", "dependency_imports", "dependency_cycles"} {
		toolDef, ok := reg.Get(name)
		if !ok {
			t.Fatalf("missing %s", name)
		}
		switch name {
		case "dependency_affected_by":
			_, _ = toolDef.Execute(ctx, json.RawMessage(`{"path":"internal/gamma/gamma.go"}`))
		case "dependency_imports":
			_, _ = toolDef.Execute(ctx, json.RawMessage(`{"path":"internal/gamma/gamma.go","direction":"imported_by"}`))
		case "dependency_cycles":
			_, _ = toolDef.Execute(ctx, json.RawMessage(`{"lang":"go"}`))
		default:
			_, _ = toolDef.Execute(ctx, json.RawMessage(`{}`))
		}
	}
	cat := idx.ModuleCatalog()
	if mod, ok := cat.ResolveFile("internal/alpha/alpha.go"); !ok {
		t.Fatalf("resolve failed for %q", mod)
	}
}

func TestNewGoBuilderValidationErrors(t *testing.T) {
	if _, err := NewGoBuilder(""); err == nil {
		t.Fatal("expected empty root error")
	}
	if _, err := NewGoBuilder(t.TempDir()); err == nil {
		t.Fatal("expected missing go.mod error")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("not-a-module-file"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGoBuilder(dir); err == nil {
		t.Fatal("expected parse go.mod error")
	}
}

func TestGoBuilderNilBuild(t *testing.T) {
	var b *GoBuilder
	if _, _, _, _, err := b.Build(); err == nil {
		t.Fatal("expected nil builder error")
	}
}

func TestBuildWithParserSyntaxAndImportErrors(t *testing.T) {
	root := copyGoTestProject(t)
	broken := filepath.Join(root, "internal", "alpha", "broken_syntax.go")
	if err := os.WriteFile(broken, []byte("package alpha\nfunc broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, parseErrors, method, err := b.buildWithParser()
	if err != nil {
		t.Fatal(err)
	}
	if method != BuildParserFallback || len(nodes) == 0 || len(edges) == 0 {
		t.Fatalf("nodes=%d edges=%d method=%q", len(nodes), len(edges), method)
	}
	if len(parseErrors) == 0 {
		t.Fatal("expected syntax parse errors")
	}
}

func TestBuildGraphEmptyRootError(t *testing.T) {
	if _, _, err := BuildGraph(BuildOptions{Root: ""}); err == nil {
		t.Fatal("expected empty root error")
	}
}

func TestDecodeGoListJSONCases(t *testing.T) {
	if _, err := decodeGoListJSON([]byte("")); err == nil {
		t.Fatal("expected empty packages error")
	}
	if _, err := decodeGoListJSON([]byte("{")); err == nil {
		t.Fatal("expected decode error")
	}
	pkgs, err := decodeGoListJSON([]byte(`{"ImportPath":"example.com/x","Dir":"."}` + "\n"))
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("pkgs=%v err=%v", pkgs, err)
	}
}

func TestNewGoBuilderMissingModuleDirective(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("go 1.21\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := NewGoBuilder(dir); err == nil {
		t.Fatal("expected missing module directive error")
	}
}

func TestOpenDependencyEmptyRoot(t *testing.T) {
	if _, err := Open(""); err == nil {
		t.Fatal("expected empty root error")
	}
}

func TestIndexStatusToolNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	toolDef := depStatusTool{idx: idx}
	if _, err := toolDef.Execute(context.Background(), nil); err == nil {
		t.Fatal("expected status error when graph missing")
	}
}

func TestDependencyImportsToolValidation(t *testing.T) {
	idx := readyTestIndex(t)
	importsTool := depImportsTool{idx: idx}
	if _, err := importsTool.Execute(context.Background(), json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected path required")
	}
	if _, err := importsTool.Execute(context.Background(), json.RawMessage(`{"path":"missing/pkg","direction":"imports"}`)); err == nil {
		t.Fatal("expected resolve error")
	}
}

func TestDependencyCyclesToolEmptyFilter(t *testing.T) {
	idx := readyTestIndex(t)
	cyclesTool := depCyclesTool{idx: idx}
	out, err := cyclesTool.Execute(context.Background(), json.RawMessage(`{"lang":"js"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Dependency cycles:") {
		t.Fatalf("out=%q", out)
	}
}

func TestRefreshIfStaleConcurrentSkip(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	mu := refreshLockFor(root)
	mu.Lock()
	defer mu.Unlock()
	if err := idx.RefreshIfStale(context.Background()); err != nil {
		t.Fatalf("expected skip when lock held, got %v", err)
	}
}

func TestRefreshIfStaleContextCanceled(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_ = idx.InvalidateFiles([]string{"internal/alpha/alpha.go"})
	if err := idx.RefreshIfStale(ctx); err == nil {
		t.Fatal("expected context canceled")
	}
}

func TestParseGoImportsSkipsDuplicatePaths(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dup.go")
	src := "package dup\nimport \"fmt\"\nimport \"fmt\"\n"
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	imports, err := ParseGoImports(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(imports) != 1 {
		t.Fatalf("imports=%v", imports)
	}
}

func TestBuildGraphJSProject(t *testing.T) {
	root := copyJSTestProject(t)
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil || g == nil || meta == nil {
		t.Fatalf("g=%v meta=%v err=%v", g, meta, err)
	}
	if len(g.Nodes) == 0 {
		t.Fatal("expected JS nodes")
	}
}

func TestNodeBuilderWalkReadError(t *testing.T) {
	root := copyJSTestProject(t)
	bad := filepath.Join(root, "frontend", "src", "broken.ts")
	if err := os.WriteFile(bad, []byte("import x from './x'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(bad, 0o000); err != nil {
		t.Skip("chmod not supported")
	}
	t.Cleanup(func() { _ = os.Chmod(bad, 0o644) })
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, errs, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(errs) == 0 {
		t.Skip("platform allows reading chmod 000 files")
	}
}

func TestSetBridgeImpactAnalyzerNilIndex(t *testing.T) {
	var idx *Index
	idx.SetBridgeImpactAnalyzer(nil)
}

func TestResolveIDEmptyPath(t *testing.T) {
	idx := readyTestIndex(t)
	if _, err := idx.ResolveID("  "); err == nil {
		t.Fatal("expected path required")
	}
}

func TestOpenDependencyProjectDirFailure(t *testing.T) {
	root := t.TempDir()
	blocker := filepath.Join(root, "block")
	if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(blocker); err == nil {
		t.Fatal("expected ProjectDir failure when workspace is a file")
	}
}

func TestOpenDependencyCorruptMeta(t *testing.T) {
	root := copyGoTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	g, meta, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if err := SaveIndex(g, dir); err != nil {
		t.Fatal(err)
	}
	if err := SaveMeta(meta, dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, metaFileName), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("expected corrupt meta error")
	}
}

func TestDependencyCyclesToolWithMatches(t *testing.T) {
	g := NewGraph(t.TempDir())
	a, b := NewGoID("a"), NewGoID("b")
	g.AddNode(&Node{ID: a, Kind: KindInternalGo, Name: "a"})
	g.AddNode(&Node{ID: b, Kind: KindInternalGo, Name: "b"})
	g.AddEdge(a, b, EdgeSourceImport, "a.go")
	g.AddEdge(b, a, EdgeSourceImport, "b.go")
	g.Cycles = computeCycles(g)
	idx := &Index{root: t.TempDir(), graph: g, meta: &Meta{IndexVersion: IndexVersion}}
	tool := depCyclesTool{idx: idx}
	out, err := tool.Execute(context.Background(), json.RawMessage(`{"lang":"go"}`))
	if err != nil || !strings.Contains(out, "Dependency cycles:") || strings.Contains(out, "[]") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestParseGoImportsSyntaxError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.go")
	if err := os.WriteFile(path, []byte("package bad\nimport (\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ParseGoImports(path); err == nil {
		t.Fatal("expected parse error")
	}
}

func TestDetectVersionConflictsSkipsBadMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("not valid"), 0o644); err != nil {
		t.Fatal(err)
	}
	if conflicts := detectVersionConflicts(root, nil); conflicts != nil {
		t.Fatalf("conflicts=%v", conflicts)
	}
}

func TestRefreshIfStaleSaveIndexFailure(t *testing.T) {
	root := copyGoTestProject(t)
	idx, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Skip("chmod not supported")
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	_ = idx.InvalidateFiles([]string{"internal/alpha/alpha.go"})
	if err := idx.RefreshIfStale(context.Background()); err == nil {
		t.Skip("platform allowed write to read-only directory")
	}
}

func TestDependencyImportsInvalidJSON(t *testing.T) {
	idx := readyTestIndex(t)
	tool := depImportsTool{idx: idx}
	if _, err := tool.Execute(context.Background(), json.RawMessage([]byte("{bad"))); err == nil {
		t.Fatal("expected invalid args error")
	}
}

func TestDependencyAffectedByIndexNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	tool := depAffectedByTool{idx: idx}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"a"}`)); err == nil {
		t.Fatal("expected affected-by error")
	}
}

func TestDetectVersionConflictsMultipleVersions(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module root.test\n\ngo 1.21\n\nrequire example.com/foo v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("module sub.test\n\ngo 1.21\n\nrequire example.com/foo v2.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicts := detectVersionConflicts(root, nil)
	if len(conflicts) == 0 {
		t.Fatal("expected version conflict")
	}
}

func TestDependencyCyclesToolNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	tool := depCyclesTool{idx: idx}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"lang":"go"}`)); err == nil {
		t.Fatal("expected find cycles error")
	}
}

func TestDependencyImportsToolNotReady(t *testing.T) {
	idx := &Index{root: t.TempDir()}
	tool := depImportsTool{idx: idx}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"internal/alpha/alpha.go","direction":"imports"}`)); err == nil {
		t.Fatal("expected imports error")
	}
}

func TestProjectDirDependencyEmptyRoot(t *testing.T) {
	if _, err := ProjectDir("  "); err == nil {
		t.Fatal("expected empty workspace error")
	}
}

func TestLoadIndexDependencyReadFailure(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, indexFileName), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadIndex(dir); err == nil {
		t.Fatal("expected read failure for directory index path")
	}
}

func TestBuildWithParserSyntaxErrorRecordsParseError(t *testing.T) {
	root := copyGoTestProject(t)
	broken := filepath.Join(root, "internal", "alpha", "broken_syntax.go")
	if err := os.WriteFile(broken, []byte("package alpha\nfunc broken( {\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b, err := NewGoBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	_, _, parseErrors, _, err := b.buildWithParser()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, pe := range parseErrors {
		if strings.Contains(pe.File, "broken_syntax.go") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("parseErrors=%v", parseErrors)
	}
}

func TestLoadIndexLegacyAdjacencyOnly(t *testing.T) {
	dir := t.TempDir()
	a, b := NewGoID("a"), NewGoID("b")
	snap := indexSnapshot{
		Version: IndexVersion,
		Root:    dir,
		Nodes: map[string]*Node{
			string(a): {ID: a, Kind: KindInternalGo, Name: "a"},
			string(b): {ID: b, Kind: KindInternalGo, Name: "b"},
		},
		Out: map[string][]string{string(a): {string(b)}},
	}
	blob, err := json.Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, indexFileName), blob, 0o644); err != nil {
		t.Fatal(err)
	}
	g, err := LoadIndex(dir)
	if err != nil || len(g.edges) == 0 {
		t.Fatalf("edges=%d err=%v", len(g.edges), err)
	}
}

func TestDetectVersionConflictsInvalidSubMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module root.test\n\ngo 1.21\n\nrequire example.com/foo v1.0.0\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "go.mod"), []byte("not-a-mod"), 0o644); err != nil {
		t.Fatal(err)
	}
	conflicts := detectVersionConflicts(root, nil)
	if len(conflicts) != 0 {
		t.Fatalf("conflicts=%v", conflicts)
	}
}

func TestDependencyImportsToolNodeRemoved(t *testing.T) {
	idx := readyTestIndex(t)
	id, err := idx.ResolveID("internal/gamma/gamma.go")
	if err != nil {
		t.Fatal(err)
	}
	idx.mu.Lock()
	delete(idx.graph.Nodes, id)
	idx.mu.Unlock()
	tool := depImportsTool{idx: idx}
	if _, err := tool.Execute(context.Background(), json.RawMessage(`{"path":"internal/gamma/gamma.go","direction":"imports"}`)); err == nil {
		t.Fatal("expected imports neighbor error")
	}
}

func TestOpenDependencyCorruptIndex(t *testing.T) {
	root := copyGoTestProject(t)
	dir, err := ProjectDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, indexFileName), []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(root); err == nil {
		t.Fatal("expected corrupt index error")
	}
}
