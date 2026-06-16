package agent

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

func TestCallgraphRetryContextNoWriter(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := &Agent{
		evidence:       evidence.NewLedger(),
		projectChecks:  []instruction.VerifyCheck{{Command: "go test ./..."}},
		callgraphIndex: idx,
	}
	if got := a.callgraphRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCallgraphRetryContextInjected(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	check := instruction.VerifyCheck{Command: "go test ./..."}
	writer := evidence.Receipt{
		ToolName: "write_file",
		Success:  true,
		Write:    true,
		Paths: []string{
			"desktop/app.go",
			"desktop/frontend/src/lib/useSubmit.ts",
		},
	}
	a := &Agent{
		evidence:        readinessLedger(writer),
		projectChecks:   []instruction.VerifyCheck{check},
		callgraphIndex:  idx,
	}

	block := a.callgraphRetryContext()
	if !strings.Contains(block, "## Wails Call Chain") {
		t.Fatalf("callgraphRetryContext() = %q, want Wails call chain heading", block)
	}
}

func TestCallgraphRetryContextSkipsSingleRealmChange(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	check := instruction.VerifyCheck{Command: "go test ./..."}
	writer := evidence.Receipt{
		ToolName: "write_file",
		Success:  true,
		Write:    true,
		Paths:    []string{"desktop/app.go"},
	}
	a := &Agent{
		evidence:       readinessLedger(writer),
		projectChecks:  []instruction.VerifyCheck{check},
		callgraphIndex: idx,
	}
	if got := a.callgraphRetryContext(); got != "" {
		t.Fatalf("callgraphRetryContext() = %q, want empty for single-realm change", got)
	}
}

func TestVerifyRetryIncludesCallgraphEndToEnd(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(&sequencedBashTool{failOutput: "FAIL boot.test/desktop\nSubmit broken"})

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("w1", "write_file", `{"path":"desktop/app.go","content":"package main\n\ntype App struct{}\n\nfunc (a *App) Submit(msg string) error { return nil }\n"}`),
			toolCallChunk("w2", "write_file", `{"path":"desktop/frontend/src/lib/useSubmit.ts","content":"import { app } from \"./bridge\";\n\nexport function useSubmit() {\n  return async (msg: string) => { await app.Submit(msg); };\n}\n"}`),
			toolCallChunk("b1", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "premature"}, {Type: provider.ChunkDone}},
		{
			toolCallChunk("b2", "bash", `{"command":"go test ./..."}`),
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "verified done"}, {Type: provider.ChunkDone}},
	}}

	a := New(prov, reg, NewSession(""), Options{
		ProjectChecks:            []instruction.VerifyCheck{{Command: "go test ./...", SourcePath: "AGENTS.md", Line: 3}},
		VerifyEnforceFinalAnswer: true,
		CallgraphIndex:           idx,
	}, event.Discard)

	if err := a.Run(context.Background(), "edit submit flow and verify"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sessionHasUserMessageContaining(a.session, "## Wails Call Chain") {
		t.Fatal("missing Wails call chain block in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "Submit") {
		t.Fatal("missing Submit method in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "## Debug Breakpoints") {
		t.Fatal("missing auto breakpoint block in readiness retry")
	}
}

func copyCallgraphTestProject(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "callgraph", "testdata", "wails_project")
	src, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyCallgraphTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyCallgraphTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyCallgraphFile(path, target)
	})
}

func copyCallgraphFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, in)
	return err
}
