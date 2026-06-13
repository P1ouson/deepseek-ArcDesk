package selfdebug

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	stdruntime "runtime"
	"strings"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/instruction"
	arcruntime "arcdesk/internal/runtime"
	"arcdesk/internal/tool"
	"arcdesk/internal/verification"
)

func newTestRegistry(t *testing.T) *tool.Registry {
	t.Helper()
	return tool.NewRegistry()
}

func TestBuildImmediateHintNoFailedCmd(t *testing.T) {
	in := Input{
		HasWriter: true,
		Checks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		HasSuccessfulCommandAfter: func(string, int) bool { return true },
	}
	if got := BuildImmediateHint(in); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPendingAndPassedCommands(t *testing.T) {
	in := Input{
		Checks: []instruction.VerifyCheck{
			{Command: "go build ./..."},
			{Command: "go test ./..."},
			{Command: ""},
		},
		WriterIndex: 0,
		HasSuccessfulCommandAfter: func(cmd string, _ int) bool {
			return cmd == "go build ./..."
		},
	}
	pending := pendingCommands(in)
	if len(pending) != 1 || pending[0] != "go test ./..." {
		t.Fatalf("pending = %v", pending)
	}
	passed := passedCommands(in)
	if len(passed) != 1 || passed[0] != "go build ./..." {
		t.Fatalf("passed = %v", passed)
	}
}

func TestResolveFailedCmdFromChecks(t *testing.T) {
	in := Input{
		Checks: []instruction.VerifyCheck{{Command: "go vet ./..."}},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	if got := resolveFailedCmd(in); got != "go vet ./..." {
		t.Fatalf("got %q", got)
	}
}

func TestLimitPaths(t *testing.T) {
	paths := []string{"a", "b", "c", "d"}
	got := limitPaths(paths, 2)
	if len(got) != 2 || got[0] != "a" {
		t.Fatalf("got %v", got)
	}
}

func TestSdStatusToolMetadata(t *testing.T) {
	st := sdStatusTool{tracker: NewTracker(Plan{})}
	if st.Description() == "" || !st.ReadOnly() || st.Name() != "selfdebug_status" {
		t.Fatal("metadata")
	}
}

func TestAnalyzeSnapshotVerifyPhase(t *testing.T) {
	in := Input{
		HasWriter: true,
		Checks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	snap := AnalyzeSnapshot(in)
	if snap.Phase != PhaseVerify {
		t.Fatalf("phase = %q", snap.Phase)
	}
}

func TestAnalyzeSnapshotExplicitFailure(t *testing.T) {
	in := Input{
		HasWriter: true,
		FailedCmd: "go build ./...",
		Checks:    []instruction.VerifyCheck{{Command: "go build ./..."}},
	}
	snap := AnalyzeSnapshot(in)
	if snap.Phase != PhaseFailed {
		t.Fatalf("phase = %q", snap.Phase)
	}
}

func TestBuildRetryContextWithVerificationOnly(t *testing.T) {
	in := Input{
		HasWriter: true,
		FailedCmd: "vitest run",
		Checks: []instruction.VerifyCheck{
			{Command: "vitest run", Category: string(verification.CategoryUnit)},
		},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	got := BuildRetryContext(in)
	if !strings.Contains(got, "Unit tests failed") {
		t.Fatalf("got %q", got)
	}
}

func TestBuildRetryContextMinimal(t *testing.T) {
	got := BuildRetryContext(Input{
		HasWriter: true,
		Checks:    []instruction.VerifyCheck{{Command: "go build ./..."}},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	})
	if !strings.Contains(got, "## Self-debug Loop") {
		t.Fatalf("got %q", got)
	}
}

func TestBuildImmediateHintNoWriter(t *testing.T) {
	if got := BuildImmediateHint(Input{FailedCmd: "go test ./..."}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestPendingCommandsExcluding(t *testing.T) {
	got := pendingCommandsExcluding(Input{
		Checks: []instruction.VerifyCheck{{Command: "a"}, {Command: "b"}},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}, "a")
	if len(got) != 1 || got[0] != "b" {
		t.Fatalf("got %v", got)
	}
	if pendingCommandsExcluding(Input{}, "") != nil {
		t.Fatal("expected nil")
	}
}

func TestResolveFailedCmdEmpty(t *testing.T) {
	if got := resolveFailedCmd(Input{}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSdStatusToolWithPending(t *testing.T) {
	tracker := NewTracker(Plan{
		Checks: []instruction.VerifyCheck{{Command: "go test ./..."}, {Command: "go vet ./..."}},
		Policy: verification.Policy{MaxRetries: 2, OnFailure: "rollback"},
	})
	tracker.Update(Snapshot{
		Phase: PhaseFailed, FailedCmd: "go test ./...",
		PendingChecks: []string{"go vet ./..."}, Attempt: 1, MaxRetries: 2,
	})
	st := sdStatusTool{tracker: tracker}
	out, err := st.Execute(t.Context(), nil)
	if err != nil || !strings.Contains(out, "Pending checks") || !strings.Contains(out, "on_failure=rollback") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestTrackerNilPlan(t *testing.T) {
	var tr *Tracker
	if tr.Plan().Policy.MaxRetries != 0 {
		t.Fatal("nil plan")
	}
}

func copyWailsProject(t *testing.T) string {
	t.Helper()
	_, file, _, ok := stdruntime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "callgraph", "testdata", "wails_project")
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

func TestBuildRetryContextFullSignals(t *testing.T) {
	root := copyWailsProject(t)
	dep, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := dep.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	hub := arcruntime.NewHub(arcruntime.DefaultLimits())
	hub.Ingest(arcruntime.KindConsole, arcruntime.LevelError, "console", "TypeError: boom", nil)

	in := Input{
		HasWriter:    true,
		FailedCmd:    "go test ./...",
		Stderr:       "FAIL pkg",
		WrittenPaths: []string{"desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"},
		Checks: []instruction.VerifyCheck{
			{Command: "go test ./...", Category: string(verification.CategoryUnit)},
		},
		Attempt:    1,
		MaxRetries: 3,
		DepIndex:   dep,
		CGIndex:    cg,
		RuntimeHub: hub,
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}
	got := BuildRetryContext(in)
	for _, want := range []string{
		"## Self-debug Loop",
		"## Dependency Impact",
		"## Debug Breakpoints",
		"## Runtime Observation",
		"## Verification Engine",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
}

func TestBuildRetryContextPendingOnly(t *testing.T) {
	got := BuildRetryContext(Input{
		HasWriter:    true,
		WrittenPaths: []string{"a.go", "b.go"},
		Checks: []instruction.VerifyCheck{
			{Command: "git status"},
		},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	})
	if !strings.Contains(got, "still required") || !strings.Contains(got, "git status") {
		t.Fatalf("got %q", got)
	}
	if strings.Contains(got, "failed check:") {
		t.Fatalf("unexpected failed check: %q", got)
	}
}

func TestResolveFailedCmdSkipsNonVerify(t *testing.T) {
	if got := resolveFailedCmd(Input{
		Checks:                    []instruction.VerifyCheck{{Command: "echo hi"}},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	}); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestLimitPathsNoCap(t *testing.T) {
	got := limitPaths([]string{"a", "b"}, 0)
	if len(got) != 2 || got[0] != "a" {
		t.Fatalf("got %v", got)
	}
}

func TestAnalyzeSnapshotManyPaths(t *testing.T) {
	paths := make([]string, 20)
	for i := range paths {
		paths[i] = fmt.Sprintf("f%d.go", i)
	}
	snap := AnalyzeSnapshot(Input{WrittenPaths: paths})
	if len(snap.WrittenPaths) != 12 {
		t.Fatalf("got %d paths", len(snap.WrittenPaths))
	}
}

func TestBuildRetryContextCallgraphPathsFallback(t *testing.T) {
	root := copyWailsProject(t)
	cg, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := cg.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	got := BuildRetryContext(Input{
		HasWriter:  true,
		FailedCmd:  "go test ./...",
		CGIndex:    cg,
		CallgraphPaths: []string{
			"desktop/app.go",
			"desktop/frontend/src/lib/useSubmit.ts",
		},
		Checks: []instruction.VerifyCheck{
			{Command: "go test ./...", Category: string(verification.CategoryUnit)},
		},
		HasSuccessfulCommandAfter: func(string, int) bool { return false },
	})
	if !strings.Contains(got, "## Debug Breakpoints") {
		t.Fatalf("got %q", got)
	}
}
