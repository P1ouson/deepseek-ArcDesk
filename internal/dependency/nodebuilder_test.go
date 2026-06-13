package dependency

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseJSImportsSource(t *testing.T) {
	src := `
import React from 'react';
import { x } from './lib/foo';
const y = require('../hooks/useUser');
// import fake from 'ignored-in-comment';
import('dynamic/path');
`
	imports := ParseJSImportsSource(src)
	specs := map[string]JSImportKind{}
	for _, imp := range imports {
		specs[imp.Spec] = imp.Kind
	}
	for spec, kind := range map[string]JSImportKind{
		"react":              JSImportPackage,
		"./lib/foo":          JSImportRelative,
		"../hooks/useUser":   JSImportRelative,
		"dynamic/path":       JSImportDynamic,
	} {
		if got, ok := specs[spec]; !ok || got != kind {
			t.Fatalf("spec %q = (%v, %v), want kind %q", spec, ok, got, kind)
		}
	}
}

func TestParseJSImportsIgnoresCommentImports(t *testing.T) {
	src := `// import hidden from 'secret';`
	imports := ParseJSImportsSource(src)
	if len(imports) != 0 {
		t.Fatalf("imports = %+v, want none", imports)
	}
}

func TestResolveRelativeJSDir(t *testing.T) {
	root := t.TempDir()
	libDir := filepath.Join(root, "frontend", "src", "lib")
	hooksDir := filepath.Join(root, "frontend", "src", "hooks")
	if err := mkdirAll(libDir, hooksDir); err != nil {
		t.Fatal(err)
	}
	hookFile := filepath.Join(hooksDir, "useUser.ts")
	if err := os.WriteFile(hookFile, []byte("export {};\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, ok := resolveRelativeJSDir(libDir, "../hooks/useUser")
	if !ok {
		t.Fatal("expected resolution")
	}
	want := hooksDir
	if filepath.Clean(got) != filepath.Clean(want) {
		t.Fatalf("resolve = %q, want %q", got, want)
	}
}

func TestNodeBuilderJSProject(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "js_project"))
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewNodeBuilder(root)
	if err != nil {
		t.Fatal(err)
	}
	nodes, edges, _, err := b.Build()
	if err != nil {
		t.Fatal(err)
	}

	kinds := kindCounts(nodes)
	if kinds[KindInternalJS] < 2 {
		t.Fatalf("internal js = %d, want >= 2: %#v", kinds[KindInternalJS], kinds)
	}
	if kinds[KindExternalNPM] < 1 {
		t.Fatalf("external npm = %d, want >= 1", kinds[KindExternalNPM])
	}

	libID := NewJSID("frontend/src/lib")
	hooksID := NewJSID("frontend/src/hooks")
	sharedID := NewJSID("packages/shared/src")
	reactID := NewNpmID("react")

	if !hasEdge(edges, libID, hooksID) {
		t.Fatalf("missing lib -> hooks edge in %#v", edges)
	}
	if !hasEdge(edges, hooksID, sharedID) {
		t.Fatalf("missing hooks -> shared edge")
	}
	if !hasEdge(edges, libID, reactID) {
		t.Fatalf("missing lib -> react edge")
	}
}

func TestBuildGraphJSProjectOnly(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("testdata", "js_project"))
	if err != nil {
		t.Fatal(err)
	}
	g, _, err := BuildGraph(BuildOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if g.BuildMethod != BuildMerged && g.Stats.JSPackages == 0 {
		t.Fatalf("graph = %+v", g.Stats)
	}
	if g.Stats.JSPackages == 0 {
		t.Fatal("expected js packages in stats")
	}
}

func TestWorkspaceProtocolOnly(t *testing.T) {
	b := &NodeBuilder{
		byName: map[string]jsPackage{
			"@demo/shared": {Manifest: jsPackageJSON{Name: "@demo/shared"}},
		},
	}
	owner := jsPackage{
		Manifest: jsPackageJSON{
			Dependencies: map[string]string{
				"@demo/shared": "workspace:*",
				"react":        "^18.0.0",
			},
		},
	}
	if !b.isWorkspaceDependency(owner, "@demo/shared") {
		t.Fatal("workspace:* should match")
	}
	if b.isWorkspaceDependency(owner, "react") {
		t.Fatal("react should not be workspace")
	}
}

func mkdirAll(dirs ...string) error {
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0o755); err != nil {
			return err
		}
	}
	return nil
}
