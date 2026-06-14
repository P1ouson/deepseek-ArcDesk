package agent

import (
	"context"
	"errors"
	"strings"
	"sync/atomic"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/constraint"
	"arcdesk/internal/diff"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/runtime"
	"arcdesk/internal/selfdebug"
	"arcdesk/internal/tool"
	"arcdesk/internal/verification"
)

func TestRepeatSuccessSignatureAndBlock(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	tt, ok := reg.Get("write_file")
	if !ok {
		t.Fatal("missing tool")
	}
	call := provider.ToolCall{Name: "write_file", Arguments: `{"path":"a.txt","content":"b"}`}
	sig, ok := repeatSuccessSignature(call, tt)
	if !ok || sig == "" {
		t.Fatalf("sig=%q ok=%v", sig, ok)
	}
	a := &Agent{repeatSuccessCounts: map[string]int{sig: repeatSuccessBreakThreshold}}
	msg, block := a.repeatedSuccessBlock(call, tt)
	if !block || msg == "" {
		t.Fatalf("block=%v msg=%q", block, msg)
	}
	a.recordRepeatSuccess(call, tt)
	if a.repeatSuccessCounts[sig] != repeatSuccessBreakThreshold+1 {
		t.Fatalf("counts=%v", a.repeatSuccessCounts)
	}
}

func TestAgentUsageAndShapeGetters(t *testing.T) {
	a := New(nil, tool.NewRegistry(), NewSession("sys"), Options{
		ContextWindow: 8192,
		CompactRatio:  0.8,
	}, event.Discard)
	a.lastUsage.Store(&provider.Usage{TotalTokens: 10})
	if a.LastUsage() == nil {
		t.Fatal("expected usage")
	}
	a.sessCacheHit.Store(3)
	a.sessCacheMiss.Store(2)
	hit, miss := a.SessionCache()
	if hit != 3 || miss != 2 {
		t.Fatalf("hit=%d miss=%d", hit, miss)
	}
	if a.ContextWindow() != 8192 || a.CompactRatio() != 0.8 {
		t.Fatalf("window=%d ratio=%f", a.ContextWindow(), a.CompactRatio())
	}
	if shape := a.capturePrefixShape(nil); shape.SystemHash == "" {
		t.Fatalf("shape=%+v", shape)
	}
}

func TestIsBackgroundTaskCallAndToolReadOnly(t *testing.T) {
	if !isBackgroundTaskCall(`{"run_in_background":true}`) {
		t.Fatal("expected background task")
	}
	if isBackgroundTaskCall(`{"run_in_background":false}`) {
		t.Fatal("expected foreground task")
	}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "ro", readOnly: true})
	a := New(nil, reg, NewSession(""), Options{}, event.Discard)
	if !a.toolReadOnly("ro") {
		t.Fatal("expected read-only")
	}
}

func TestSubagentGateForContext(t *testing.T) {
	a := &Agent{}
	if got := a.subagentGateForContext(); got != nil {
		t.Fatal("expected nil without inherit")
	}
	a.SetInheritSubagentGate(true)
	if got := a.subagentGateForContext(); got != nil {
		t.Fatal("expected nil without interactive gate")
	}
}

func TestWithCallContextAddsProgress(t *testing.T) {
	a := New(nil, tool.NewRegistry(), NewSession(""), Options{}, event.Discard)
	ctx := withCallContext(context.Background(), "id1", a.sink, nil, nil)
	if ctx == context.Background() {
		t.Fatal("expected derived context")
	}
}

func TestNormalizeShellCommand(t *testing.T) {
	if got := normalizeShellCommand("  echo   hi  "); got != "echo hi" {
		t.Fatalf("got %q", got)
	}
}

func TestShellPythonOpenWrites(t *testing.T) {
	if !shellPythonOpenWrites(`python -c "open('x','w').write('y')"`) {
		t.Fatal("expected python open write")
	}
	if shellPythonOpenWrites("python -c \"print('hi')\"") {
		t.Fatal("expected read-only python")
	}
}

func TestRepeatSuccessSignatureBash(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "bash", readOnly: false})
	tt, _ := reg.Get("bash")
	call := provider.ToolCall{Name: "bash", Arguments: `{"command":"echo hi > out.txt"}`}
	if _, ok := repeatSuccessSignature(call, tt); !ok {
		t.Fatal("expected bash write signature")
	}
}

func TestRecordRepeatSuccessReadOnlySkipped(t *testing.T) {
	a := &Agent{}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	tt, _ := reg.Get("read_file")
	a.recordRepeatSuccess(provider.ToolCall{Name: "read_file"}, tt)
	if a.repeatSuccessCounts != nil {
		t.Fatal("read-only should not record")
	}
}

func TestRepeatedSuccessBlockNilCounts(t *testing.T) {
	a := &Agent{}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	tt, _ := reg.Get("write_file")
	call := provider.ToolCall{Name: "write_file", Arguments: `{"path":"a"}`}
	if msg, block := a.repeatedSuccessBlock(call, tt); block || msg != "" {
		t.Fatalf("block=%v msg=%q", block, msg)
	}
}

func TestExecuteBatchIncrementsCalls(t *testing.T) {
	var calls int32
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true, calls: &calls})
	a := New(&scriptedProvider{name: "p", turns: nil}, reg, NewSession(""), Options{}, event.Discard)
	out := a.executeBatch(context.Background(), []provider.ToolCall{{ID: "1", Name: "read_file", Arguments: `{}`}})
	if len(out) != 1 || atomic.LoadInt32(&calls) != 1 {
		t.Fatalf("out=%v calls=%d", out, calls)
	}
}

func TestRuntimeRetryContextPendingCheck(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	hub.Ingest(runtime.KindConsole, runtime.LevelError, "console", "TypeError: pending verify", nil)
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:      readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		runtimeHub:    hub,
	}
	if got := a.runtimeRetryContext(); got == "" {
		t.Fatal("expected pending verify runtime context")
	}
}

func TestCallgraphRetryContextPendingVerify(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	writer := evidence.Receipt{
		ToolName: "write_file", Success: true, Write: true,
		Paths: []string{"desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"},
	}
	a := &Agent{
		evidence:       readinessLedger(writer),
		projectChecks:  []instruction.VerifyCheck{{Command: "go test ./..."}},
		callgraphIndex: idx,
	}
	if got := a.callgraphRetryContext(); got == "" {
		t.Fatal("expected callgraph retry context")
	}
}

func TestSelfdebugRetryContextIncludesConstraint(t *testing.T) {
	eng := constraint.NewEngine(&constraint.Host{Root: t.TempDir()}, constraint.DefaultSettings())
	eng.CheckEdit("write_file", diff.Change{
		Path: "desktop/frontend/src/App.css", Kind: diff.Modify,
		OldText: ".x{}", NewText: ".x{display:none}",
	})
	tracker := selfdebug.NewTracker(selfdebug.Plan{
		Checks: []instruction.VerifyCheck{{Command: "go test ./..."}},
		Policy: verification.Policy{MaxRetries: 2},
	})
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"desktop/frontend/src/App.css"}}
	a := &Agent{
		evidence:         readinessLedger(writer),
		projectChecks:    []instruction.VerifyCheck{{Command: "go test ./..."}},
		selfdebugTracker: tracker,
		constraintEngine: eng,
	}
	a.noteVerifyFailure(context.Background(),provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errTest, "FAIL")
	got := a.selfdebugRetryContext(1)
	if !strings.Contains(got, "## Constraint System") {
		t.Fatalf("got %q", got)
	}
}

func TestNilAgentSettersAreSafe(t *testing.T) {
	var a *Agent
	a.SetDependencyIndex(nil)
	a.SetCallgraphIndex(nil)
	a.SetRuntimeHub(nil)
	a.SetRollbackHost(nil)
	a.SetVerifyOnFailure("rollback")
}

var errTest = errors.New("fail")
