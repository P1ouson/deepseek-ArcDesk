package builtin

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReadFileConfinement(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "in.txt")
	if err := os.WriteFile(inside, []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "secret.txt")
	if err := os.WriteFile(outside, []byte("secret\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rf := readFile{workDir: root, roots: realRoots([]string{root})}

	inArgs, _ := json.Marshal(map[string]string{"path": "in.txt"})
	if out, err := rf.Execute(context.Background(), inArgs); err != nil || !strings.Contains(out, "hello") {
		t.Fatalf("in-workspace read: out=%q err=%v", out, err)
	}

	outArgs, _ := json.Marshal(map[string]string{"path": outside})
	if _, err := rf.Execute(context.Background(), outArgs); err == nil {
		t.Fatal("absolute path outside workspace should be refused")
	}
}

func TestReadFileSymlinkEscapeBlocked(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, "out")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	rf := readFile{workDir: root, roots: realRoots([]string{root})}
	args, _ := json.Marshal(map[string]string{"path": filepath.Join(link, "secret.txt")})
	if _, err := rf.Execute(context.Background(), args); err == nil {
		t.Fatal("read through symlink escape should be refused")
	}
}

func TestGlobOutsideWorkspaceBlocked(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "away.go")
	if err := os.WriteFile(outside, []byte("package away\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := globTool{workDir: root, roots: realRoots([]string{root})}
	args, _ := json.Marshal(map[string]string{"pattern": outside})
	if _, err := g.Execute(context.Background(), args); err == nil {
		t.Fatal("glob outside workspace should be refused")
	}
}

func TestGrepOutsideWorkspaceBlocked(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "away.go")
	if err := os.WriteFile(outside, []byte("needle\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	g := grepTool{workDir: root, roots: realRoots([]string{root})}
	args, _ := json.Marshal(map[string]string{"pattern": "needle", "path": outside})
	if _, err := g.Execute(context.Background(), args); err == nil {
		t.Fatal("grep outside workspace should be refused")
	}
}

func TestWorkspaceReadConfinement(t *testing.T) {
	dir := t.TempDir()
	outside := filepath.Join(t.TempDir(), "evil.txt")
	ws := Workspace{Dir: dir}
	rf := byName(ws.Tools())["read_file"]
	args, _ := json.Marshal(map[string]string{"path": outside})
	if _, err := rf.Execute(context.Background(), args); err == nil {
		t.Error("workspace-bound read_file should refuse path outside workspace")
	}
}
