package envaware

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

func TestLooksLikeWails(t *testing.T) {
	root := t.TempDir()
	if looksLikeWails("") {
		t.Fatal("empty workspace")
	}
	if looksLikeWails(root) {
		t.Fatal("plain dir")
	}
	if err := os.WriteFile(filepath.Join(root, "wails.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !looksLikeWails(root) {
		t.Fatal("wails.json")
	}
	modRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(modRoot, "go.mod"), []byte("module x\nrequire wails.io/v2 v2"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !looksLikeWails(modRoot) {
		t.Fatal("go.mod wails")
	}
}

func TestPlatformNotesAndHints(t *testing.T) {
	snap := Snapshot{OS: "windows", Shell: "powershell.exe", GoVersion: "", NodeVersion: ""}
	notes := platformNotes(snap)
	if len(notes) < 2 {
		t.Fatalf("windows notes=%v", notes)
	}
	snap = Snapshot{OS: "darwin", GoVersion: "go1.22", NodeVersion: "v20"}
	notes = platformNotes(snap)
	if len(notes) == 0 {
		t.Fatal("darwin notes")
	}
	snap = Snapshot{OS: "linux", GoVersion: "go1.22"}
	notes = platformNotes(snap)
	if len(notes) == 0 {
		t.Fatal("linux notes")
	}
	h := toolchainHints(Snapshot{GoVersion: "go", NodeVersion: "v1", PnpmVersion: "9", NpmVersion: "10"})
	if h["pnpm"] != "9" {
		t.Fatalf("pnpm hint=%v", h)
	}
	h = toolchainHints(Snapshot{NpmVersion: "10"})
	if h["npm"] != "10" {
		t.Fatalf("npm hint=%v", h)
	}
	h = toolchainHints(Snapshot{WailsVersion: "v2"})
	if h["wails"] != "v2" {
		t.Fatal("wails hint")
	}
}

func TestComposeBlockVariants(t *testing.T) {
	min := ComposeBlock(Snapshot{OS: "linux", Arch: "amd64"})
	if !strings.Contains(min, "linux/amd64") {
		t.Fatalf("min=%q", min)
	}
	full := ComposeBlock(Snapshot{
		OS:           "windows",
		Arch:         "amd64",
		GoVersion:    "go version go1.22",
		NodeVersion:  "v20.0.0",
		WailsVersion: "v2.8",
		GhAvailable:  true,
		PlatformNotes: []string{"note"},
	})
	if !strings.Contains(full, "gh available") || !strings.Contains(full, "note") {
		t.Fatalf("full=%q", full)
	}
}

func TestProbeCIAndRefresh(t *testing.T) {
	t.Setenv("CI", "true")
	t.Setenv("GITHUB_ACTIONS", "")
	snap := Probe(context.Background(), t.TempDir())
	if !snap.CI {
		t.Fatal("CI flag")
	}
	t.Setenv("CI", "")
	reg := tool.NewRegistry()
	RegisterRefreshTool(reg, t.TempDir())
	refresh, ok := reg.Get("environment_refresh")
	if !ok {
		t.Fatal("missing refresh")
	}
	out, err := refresh.Execute(context.Background(), json.RawMessage(`{}`))
	if err != nil || !strings.Contains(out, "Refreshed") {
		t.Fatalf("refresh: %q err=%v", out, err)
	}
}

func TestRegisterToolsNil(t *testing.T) {
	reg := tool.NewRegistry()
	RegisterTools(nil, Snapshot{})
	RegisterRefreshTool(nil, "x")
	RegisterRefreshTool(reg, "")
	if reg.Len() != 0 {
		t.Fatal("no tools")
	}
}

func TestRunVersionWithFakeBinary(t *testing.T) {
	dir := t.TempDir()
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(dir, "fakever.cmd"), []byte("@echo off\necho fake-tool 1.2.3\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		p := filepath.Join(dir, "fakever")
		if err := os.WriteFile(p, []byte("#!/bin/sh\necho fake-tool 1.2.3\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	got := runVersion(context.Background(), "fakever")
	if got == "" || !strings.Contains(got, "fake-tool") {
		t.Fatalf("runVersion=%q", got)
	}
	long := strings.Repeat("x", 200)
	if runtime.GOOS == "windows" {
		if err := os.WriteFile(filepath.Join(dir, "longver.cmd"), []byte("@echo off\necho "+long+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	} else {
		p := filepath.Join(dir, "longver")
		if err := os.WriteFile(p, []byte("#!/bin/sh\necho "+long+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	got = runVersion(context.Background(), "longver")
	if len(got) > 120 {
		t.Fatalf("should truncate: len=%d", len(got))
	}
}

func TestProbeWailsWorkspace(t *testing.T) {
	root := t.TempDir()
	desktop := filepath.Join(root, "desktop")
	if err := os.MkdirAll(desktop, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(desktop, "wails.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !looksLikeWails(root) {
		t.Fatal("desktop wails.json")
	}
	_ = Probe(context.Background(), root)
}
