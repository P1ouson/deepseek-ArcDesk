package agent

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"arcdesk/internal/dependency"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

func TestDependencyRetryContextInjected(t *testing.T) {
	root := copyDependencyTestProject(t)
	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	check := instruction.VerifyCheck{Command: "go test ./..."}
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"internal/alpha/alpha.go"}}
	a := &Agent{
		evidence:        readinessLedger(writer),
		projectChecks:   []instruction.VerifyCheck{check},
		dependencyIndex: idx,
	}

	block := a.dependencyRetryContext()
	if !strings.Contains(block, "## Dependency Impact") {
		t.Fatalf("dependencyRetryContext() = %q, want dependency impact heading", block)
	}
}

func TestDependencyRetryContextSkipsNonVerifyCheck(t *testing.T) {
	root := copyDependencyTestProject(t)
	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	check := instruction.VerifyCheck{Command: "git diff --check"}
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:        readinessLedger(writer),
		projectChecks:   []instruction.VerifyCheck{check},
		dependencyIndex: idx,
	}
	if got := a.dependencyRetryContext(); got != "" {
		t.Fatalf("dependencyRetryContext() = %q, want empty for non-verify command", got)
	}
}

// sequencedBashTool fails the first verify invocation, then succeeds.
type sequencedBashTool struct {
	calls      int
	failOutput string
}

func (s *sequencedBashTool) Name() string            { return "bash" }
func (s *sequencedBashTool) Description() string     { return "" }
func (s *sequencedBashTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (s *sequencedBashTool) ReadOnly() bool          { return false }
func (s *sequencedBashTool) Execute(context.Context, json.RawMessage) (string, error) {
	s.calls++
	if s.calls == 1 {
		return s.failOutput, errors.New("exit status 1")
	}
	return "bash done", nil
}

// TestVerifyRetryIncludesDependencyImpactEndToEnd drives Run(): write, failed verify,
// blocked final answer — the synthetic readiness retry must include dependency impact.
func TestVerifyRetryIncludesDependencyImpactEndToEnd(t *testing.T) {
	root := copyDependencyTestProject(t)
	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}

	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	reg.Add(&sequencedBashTool{
		failOutput: "FAIL example.com/alpha\ninternal/alpha/alpha.go:3:1: expected declaration, found '!'",
	})

	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{
			toolCallChunk("w1", "write_file", `{"path":"internal/alpha/alpha.go","content":"package alpha\n\n!!!"}`),
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
		DependencyIndex:          idx,
	}, event.Discard)

	if err := a.Run(context.Background(), "edit alpha and verify"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !sessionHasUserMessageContaining(a.session, "## Dependency Impact") {
		t.Fatal("missing dependency impact block in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "internal/alpha/alpha.go") {
		t.Fatal("missing changed-path impact in readiness retry")
	}
	if !sessionHasUserMessageContaining(a.session, "go test ./...") {
		t.Fatal("missing failed verify command in readiness retry")
	}
}

func copyDependencyTestProject(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "dependency", "testdata", "go_project")
	src, err := filepath.Abs(src)
	if err != nil {
		t.Fatal(err)
	}
	dst := t.TempDir()
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	return dst
}

func copyTree(src, dst string) error {
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
		return copyFile(path, target)
	})
}

func copyFile(src, dst string) error {
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
