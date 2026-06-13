package codegraph

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func writeLauncher(t *testing.T, dir, name, body string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		p := filepath.Join(dir, name+".cmd")
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		return p
	}
	p := filepath.Join(dir, name)
	writeExec(t, p, body)
	return p
}

func makeZip(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, body := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestResolveOverrideWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows launcher extension")
	}
	dir := t.TempDir()
	bin := writeLauncher(t, dir, "codegraph", "@echo off\nexit /b 0\n")
	got, ok := Resolve(bin)
	if !ok || got != bin {
		t.Fatalf("Resolve(%q) = %q, %v; want %q, true", bin, got, ok, bin)
	}
}

func TestEnsureInitWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows launcher")
	}
	root := t.TempDir()
	bin := writeLauncher(t, t.TempDir(), "fakecg", "@echo off\nmkdir .codegraph\n")
	if err := EnsureInit(context.Background(), bin, root); err != nil {
		t.Fatalf("EnsureInit = %v", err)
	}
	if !Initialized(root) {
		t.Fatal(".codegraph not created")
	}
}

func TestInstallReturnsCachedWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows launcher")
	}
	base := t.TempDir()
	t.Setenv("arcdesk_CACHE_DIR", base)
	launcher := filepath.Join(CacheDir(), "bin", "codegraph.cmd")
	if err := os.MkdirAll(filepath.Dir(launcher), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(launcher, []byte("@echo off\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Install(context.Background(), nil)
	if err != nil || got != launcher {
		t.Fatalf("Install cached = %q, %v; want %q", got, err, launcher)
	}
	if p, ok := Resolve(""); !ok || p != launcher {
		t.Fatalf("Resolve = %q, %v; want %q", p, ok, launcher)
	}
}

func TestExtractZip(t *testing.T) {
	data := makeZip(t, map[string]string{
		"bin/codegraph.cmd": "@echo off\n",
		"lib/app.js":        "console.log(1)",
	})
	dir := t.TempDir()
	if err := extractZip(data, dir); err != nil {
		t.Fatalf("extractZip: %v", err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "lib", "app.js"))
	if err != nil || string(b) != "console.log(1)" {
		t.Fatalf("lib/app.js = %q, %v", b, err)
	}
}

func TestExtractZipRejectsTraversal(t *testing.T) {
	data := makeZip(t, map[string]string{"../evil": "x"})
	if err := extractZip(data, t.TempDir()); err == nil {
		t.Fatal("extractZip should reject ../evil")
	}
}

func TestSingleChild(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "codegraph-x64")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := singleChild(parent)
	if err != nil || got != root {
		t.Fatalf("singleChild = %q, %v; want %q", got, err, root)
	}
	if _, err := singleChild(root); err == nil {
		t.Fatal("expected error for empty dir with no single child")
	}
}

func TestLogfNil(t *testing.T) {
	logf(nil, "noop %s", "x")
}

func TestInitializedEdgeCases(t *testing.T) {
	if Initialized("") {
		t.Fatal("empty root should be false")
	}
	root := t.TempDir()
	if Initialized(root) {
		t.Fatal("missing .codegraph should be false")
	}
}

func TestCachedMiss(t *testing.T) {
	t.Setenv("arcdesk_CACHE_DIR", t.TempDir())
	if _, ok := cached(); ok {
		t.Fatal("empty cache should not resolve")
	}
}

func TestInstallWithClientMockRelease(t *testing.T) {
	base := t.TempDir()
	t.Setenv("arcdesk_CACHE_DIR", base)

	asset := assetName()
	rootName := strings.TrimSuffix(asset, ".zip")
	if !strings.HasSuffix(asset, ".zip") {
		rootName = strings.TrimSuffix(asset, ".tar.gz")
	}
	launcherRel := launcherNames()[0]
	inner := filepath.ToSlash(filepath.Join(rootName, launcherRel))
	var archive []byte
	if strings.HasSuffix(asset, ".zip") {
		archive = makeZip(t, map[string]string{inner: "@echo off\n"})
	} else {
		archive = makeTarGz(t, map[string]struct {
			body string
			mode int64
		}{inner: {"#!/bin/sh\n", 0o755}})
	}
	sum := sha256.Sum256(archive)
	sumsBody := hex.EncodeToString(sum[:]) + "  " + asset + "\n"

	srv := httptestReleaseServer(t, asset, sumsBody, archive)
	t.Setenv("ARCDESK_CODEGRAPH_RELEASE_BASE", srv.URL)

	var lines []string
	got, err := InstallWithClient(context.Background(), srv.Client(), func(s string) { lines = append(lines, s) })
	if err != nil {
		t.Fatalf("InstallWithClient: %v", err)
	}
	if !isExec(got) {
		t.Fatalf("launcher %q not executable", got)
	}
	if len(lines) < 2 {
		t.Fatalf("expected progress logs, got %v", lines)
	}
}

// httptestReleaseServer is defined in install_test.go for shared mock releases.
