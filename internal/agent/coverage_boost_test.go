package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
	"arcdesk/internal/event"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/jobs"
	"arcdesk/internal/provider"
	"arcdesk/internal/rollback"
	"arcdesk/internal/tool"
)

func TestAgentSettersAndGetters(t *testing.T) {
	a := &Agent{}
	a.SetAsker(nil)
	a.SetMemoryQueue(nil)
	a.SetPreEditHook(nil)
	a.SetSession(NewSession("s1"))
	a.SetDependencyIndex(nil)
	a.SetCallgraphIndex(nil)
	a.SetRuntimeHub(nil)
	a.SetRollbackHost(rollback.NewHost(nil, "", nil))
	a.SetVerifyOnFailure("rollback")
	a.SetPlanMode(true)
	a.SetInheritSubagentGate(true)
	if !a.InheritsSubagentGate() {
		t.Fatal("expected inherit gate true")
	}
	a.ContextWindow()
	a.CompactRatio()
	if got := a.LastUsage(); got != nil {
		t.Fatalf("LastUsage = %+v", got)
	}
	if a.systemPrompt() != "s1" {
		t.Fatalf("systemPrompt = %q", a.systemPrompt())
	}
}

func TestFinalReadinessCheckSource(t *testing.T) {
	if got := finalReadinessCheckSource(instruction.VerifyCheck{}); got != "project memory" {
		t.Fatalf("got %q", got)
	}
	if got := finalReadinessCheckSource(instruction.VerifyCheck{SourcePath: "AGENTS.md", Line: 3}); got != "AGENTS.md:3" {
		t.Fatalf("got %q", got)
	}
}

func TestShellFileWriteDetection(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{`python -c "open('f','w').write('x')"`, true},
		{"echo hi > out.txt", true},
		{"cat file.txt", false},
		{"powershell Set-Content -Path x -Value y", true},
		{"sed -i 's/a/b/' f.go", true},
	}
	for _, tc := range cases {
		if got := isShellFileWriteCommand(tc.cmd); got != tc.want {
			t.Fatalf("isShellFileWriteCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestHasShellWriteRedirectQuoted(t *testing.T) {
	if !hasShellWriteRedirect(`echo ">" > out.txt`) {
		t.Fatal("expected redirect outside quotes")
	}
	if hasShellWriteRedirect(`echo "a > b"`) {
		t.Fatal("redirect inside quotes should not match")
	}
}

func TestCanonicalToolArgs(t *testing.T) {
	got := canonicalToolArgs(`{"b":2,"a":1}`)
	if !strings.Contains(got, `"a":1`) {
		t.Fatalf("got %q", got)
	}
	if got := canonicalToolArgs("not-json"); got != "not-json" {
		t.Fatalf("got %q", got)
	}
}

func TestFirstLineHelper(t *testing.T) {
	if got := firstLine("a\nb"); got != "a" {
		t.Fatalf("got %q", got)
	}
	if got := firstLine("only"); got != "only" {
		t.Fatalf("got %q", got)
	}
}

func TestCallgraphRetryContextNoIndex(t *testing.T) {
	a := &Agent{
		evidence: readinessLedger(evidence.Receipt{
			Write: true, Success: true,
			Paths: []string{"desktop/app.go", "desktop/frontend/src/x.ts"},
		}),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./..."}},
	}
	if got := a.callgraphRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCallgraphRetryContextNonVerify(t *testing.T) {
	idx, _ := callgraph.Open(t.TempDir(), nil)
	a := &Agent{
		evidence:       readinessLedger(evidence.Receipt{Write: true, Success: true, Paths: []string{"a.go", "b.ts"}}),
		projectChecks:  []instruction.VerifyCheck{{Command: "git status"}},
		callgraphIndex: idx,
	}
	if got := a.callgraphRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestSetDependencyIndexWiring(t *testing.T) {
	root := copyCallgraphTestProject(t)
	depIdx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	a := &Agent{}
	a.SetDependencyIndex(depIdx)
	if a.dependencyIndex == nil {
		t.Fatal("expected dependency index")
	}
}

func TestSchemaTokenCosts(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	costs := SchemaTokenCosts(reg.Schemas())
	if len(costs) == 0 {
		t.Fatal("expected costs")
	}
}

func TestBranchSavePreserveUpdated(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "session.jsonl")
	meta := BranchMeta{ID: "b1", Name: "feature"}
	if err := SaveBranchMetaPreserveUpdated(sessionPath, meta); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadBranchMeta(sessionPath)
	if err != nil || !ok || loaded.Name != "feature" {
		t.Fatalf("loaded = %+v err=%v", loaded, err)
	}
}

func TestCompactEmitAborted(t *testing.T) {
	a := &Agent{sink: event.Discard}
	a.emitCompactionAborted("test")
}

func TestSummarizeFromOutOfRange(t *testing.T) {
	a := &Agent{session: NewSession("x")}
	if err := a.SummarizeFrom(context.Background(), 99); err != nil {
		t.Fatalf("SummarizeFrom: %v", err)
	}
}

func TestTaskToolDescription(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{{{Type: provider.ChunkDone}}}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "sys", nil)
	if tt.Description() == "" {
		t.Fatal("expected description")
	}
}

func TestTaskToolExecuteInvalidArgs(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{{{Type: provider.ChunkDone}}}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "sys", nil)
	if _, err := tt.Execute(context.Background(), []byte(`{`)); err == nil {
		t.Fatal("expected invalid args error")
	}
}

func TestNestedSinkFallback(t *testing.T) {
	sink := NestedSink(context.Background(), event.Discard)
	if sink == nil {
		t.Fatal("expected sink")
	}
}

func TestCoordinatorNew(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{{{Type: provider.ChunkDone}}}}
	exec := New(prov, tool.NewRegistry(), NewSession("exec"), Options{}, event.Discard)
	c := NewCoordinator(prov, NewSession("plan"), nil, exec, 0, event.Discard, nil, nil)
	if c == nil {
		t.Fatal("expected coordinator")
	}
}

func TestSessionSaveRoundTrip(t *testing.T) {
	dir := t.TempDir()
	s := NewSession("save-test")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hi"})
	path := filepath.Join(dir, "s.jsonl")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
}

func TestBranchMetaDefaultScope(t *testing.T) {
	if got := (BranchMeta{Scope: "project"}).DefaultScope(); got != "project" {
		t.Fatalf("scope = %q", got)
	}
	if got := (BranchMeta{}).DefaultScope(); got != "global" {
		t.Fatalf("scope = %q", got)
	}
}

func TestBranchMetaPath(t *testing.T) {
	if got := BranchMetaPath("/tmp/session.jsonl"); !strings.Contains(got, ".meta") {
		t.Fatalf("path = %q", got)
	}
}

func TestNormalizeToolSchemasEmpty(t *testing.T) {
	got := normalizeToolSchemas(nil)
	if len(got) != 0 {
		t.Fatalf("got %v", got)
	}
}

func TestEstimateTokensZero(t *testing.T) {
	if estimateTokens("") != 0 {
		t.Fatal("expected zero")
	}
}

func TestShellPythonOpenWritesVariants(t *testing.T) {
	if !shellPythonOpenWrites(`python -c "open('f', 'w')"`) {
		t.Fatal("expected write mode")
	}
	if shellPythonOpenWrites(`python -c "open('f','r')"`) {
		t.Fatal("read mode should not match")
	}
}

func TestSummarizeFromCompactsRegion(t *testing.T) {
	fp := &fakeProvider{reply: "summary text"}
	s := NewSession("sys")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "world"})
	a := &Agent{
		prov:    fp,
		session: s,
		sink:    event.Discard,
	}
	if err := a.SummarizeFrom(context.Background(), 0); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 1 {
		t.Fatalf("messages = %d, want 1 summary", len(s.Messages))
	}
}

func TestCompactNowManual(t *testing.T) {
	fp := &fakeProvider{reply: "compact summary"}
	s := NewSession("sys")
	for i := 0; i < 6; i++ {
		s.Add(provider.Message{Role: provider.RoleUser, Content: strings.Repeat("x", 50)})
		s.Add(provider.Message{Role: provider.RoleAssistant, Content: strings.Repeat("y", 50)})
	}
	a := New(fp, tool.NewRegistry(), s, Options{
		ContextWindow: 500,
		CompactRatio:  0.5,
	}, event.Discard)
	if err := a.CompactNow(context.Background(), "focus on tests"); err != nil {
		t.Fatal(err)
	}
}

func TestTaskToolMissingPrompt(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{{{Type: provider.ChunkDone}}}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "sys", nil)
	if _, err := tt.Execute(context.Background(), []byte(`{}`)); err == nil {
		t.Fatal("expected prompt required error")
	}
}

func TestTaskToolBackgroundWithoutJobs(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{{{Type: provider.ChunkDone}}}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "sys", nil)
	_, err := tt.Execute(context.Background(), []byte(`{"prompt":"x","run_in_background":true}`))
	if err == nil || !strings.Contains(err.Error(), "background execution") {
		t.Fatalf("err = %v", err)
	}
}

func TestNestedSinkWithCallContext(t *testing.T) {
	parent := &boostRecordSink{}
	ctx := withCallContext(context.Background(), "call-1", parent, nil, nil)
	sink := NestedSink(ctx, event.Discard)
	sink.Emit(event.Event{Kind: event.ToolDispatch, Tool: event.Tool{ID: "t1"}})
	if len(parent.events) == 0 {
		t.Fatal("expected nested event on parent sink")
	}
}

type boostRecordSink struct {
	events []event.Event
}

func (s *boostRecordSink) Emit(e event.Event) { s.events = append(s.events, e) }

func TestListBranchesEmptyDir(t *testing.T) {
	branches, err := ListBranches(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(branches) != 0 {
		t.Fatalf("branches = %d", len(branches))
	}
}

func TestSetCallgraphIndex(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	a := &Agent{}
	a.SetCallgraphIndex(idx)
	if a.callgraphIndex == nil {
		t.Fatal("expected callgraph index")
	}
}

func TestDependencyRetryContextNilIndex(t *testing.T) {
	a := &Agent{
		evidence: readinessLedger(evidence.Receipt{
			Write: true, Success: true, Paths: []string{"internal/a.go"},
		}),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./..."}},
	}
	if got := a.dependencyRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestArchiveMessages(t *testing.T) {
	dir := t.TempDir()
	msgs := []provider.Message{{Role: provider.RoleUser, Content: "hi"}}
	path, err := archiveMessages(dir, msgs)
	if err != nil || path == "" {
		t.Fatalf("archiveMessages: path=%q err=%v", path, err)
	}
}

func TestBranchIDFromPath(t *testing.T) {
	if got := BranchID("/tmp/foo.jsonl"); got != "foo" {
		t.Fatalf("BranchID = %q", got)
	}
}

func TestSessionSaveEmptyPath(t *testing.T) {
	s := NewSession("x")
	if err := s.Save(""); err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestCoordinatorPlanSkip(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "plan step 1"}, {Type: provider.ChunkDone}},
	}}
	exec := New(prov, tool.NewRegistry(), NewSession("exec"), Options{}, event.Discard)
	c := NewCoordinator(prov, NewSession("plan"), nil, exec, 0, event.Discard, func(string) bool { return false }, nil)
	if err := c.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestSubSinkForDirect(t *testing.T) {
	parent := &boostRecordSink{}
	sink := subSinkFor("pid", parent)
	sink.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{ID: "t1"}})
	if len(parent.events) == 0 {
		t.Fatal("expected forwarded event")
	}
}

func TestClearVerifyFailure(t *testing.T) {
	a := &Agent{}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test"}`}, errors.New("fail"), "output")
	a.clearVerifyFailure()
	cmd, stderr := a.latestVerifyFailureForRetry(1)
	if cmd != "" || stderr != "" {
		t.Fatalf("cmd=%q stderr=%q", cmd, stderr)
	}
}

func TestNoteVerifyFailureSkipsNonVerify(t *testing.T) {
	a := &Agent{}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"git status"}`}, errors.New("fail"), "output")
	cmd, _ := a.latestVerifyFailureForRetry(1)
	if cmd != "" {
		t.Fatalf("cmd = %q", cmd)
	}
}

func TestCompareShapeDiagnostics(t *testing.T) {
	old := PrefixShape{SystemHash: "a", ToolsHash: "b"}
	newS := PrefixShape{SystemHash: "c", ToolsHash: "b"}
	d := CompareShape(old, newS, nil)
	if !d.PrefixChanged {
		t.Fatal("expected prefix changed")
	}
}

func TestSummarizeUpToCompactsHead(t *testing.T) {
	fp := &fakeProvider{reply: "head summary"}
	s := NewSession("sys")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "old turn"})
	s.Add(provider.Message{Role: provider.RoleAssistant, Content: "old reply"})
	s.Add(provider.Message{Role: provider.RoleUser, Content: "recent"})
	a := &Agent{prov: fp, session: s, sink: event.Discard, archiveDir: t.TempDir()}
	if err := a.SummarizeUpTo(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
	if len(s.Messages) != 3 {
		t.Fatalf("messages = %d, want system + summary + recent", len(s.Messages))
	}
	if !strings.Contains(s.Messages[0].Content, "sys") {
		t.Fatalf("system lost: %q", s.Messages[0].Content)
	}
	if !strings.Contains(s.Messages[1].Content, "head summary") {
		t.Fatalf("summary = %q", s.Messages[1].Content)
	}
	if !strings.Contains(s.Messages[2].Content, "recent") {
		t.Fatalf("tail = %q", s.Messages[2].Content)
	}
}

func TestRunSubAgentFinalAnswer(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "done"}, {Type: provider.ChunkDone}},
	}}
	reg := tool.NewRegistry()
	out, err := RunSubAgent(context.Background(), sub, reg, "sys", "go", Options{}, event.Discard)
	if err != nil || out != "done" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestFilterRegistryExcludesMeta(t *testing.T) {
	parent := tool.NewRegistry()
	parent.Add(fakeTool{name: "read_file", readOnly: true})
	parent.Add(fakeTool{name: "task", readOnly: false})
	sub := FilterRegistry(parent, nil, "task")
	if _, ok := sub.Get("task"); ok {
		t.Fatal("task should be excluded")
	}
	if _, ok := sub.Get("read_file"); !ok {
		t.Fatal("read_file should remain")
	}
}

func TestLoadSessionRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	s := NewSession("sys")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "persist"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadSession(path)
	if err != nil {
		t.Fatalf("LoadSession: %v", err)
	}
	if len(loaded.Messages) != 2 {
		t.Fatalf("messages = %d, want system + user", len(loaded.Messages))
	}
	if loaded.Messages[1].Content != "persist" {
		t.Fatalf("user content = %q", loaded.Messages[1].Content)
	}
}

func TestMigrateLegacySessions(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()
	if err := os.WriteFile(filepath.Join(src, "old.events.jsonl"), []byte(legacyEventLog), 0o644); err != nil {
		t.Fatal(err)
	}
	n, err := migrateLegacySessions(src, dst, "test-marker", false)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
}

func TestTouchBranchMetaCreates(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	s := NewSession("sys")
	_ = s.Save(path)
	if err := TouchBranchMeta(path); err != nil {
		t.Fatal(err)
	}
}

func TestEnsureBranchMeta(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	s := NewSession("sys")
	_ = s.Save(path)
	meta, err := EnsureBranchMeta(path)
	if err != nil || meta.ID == "" {
		t.Fatalf("meta=%+v err=%v", meta, err)
	}
}

func TestCaptureShape(t *testing.T) {
	shape := CaptureShape("sys", nil, 0)
	if shape.SystemHash == "" {
		t.Fatal("expected system hash")
	}
}

func TestTaskToolMaxStepsHalving(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "ok"}, {Type: provider.ChunkDone}},
	}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 20, 5000, 0.7, 0.8, 0.9, 0, "", "sys", nil)
	out, err := tt.Execute(context.Background(), []byte(`{"prompt":"analyze","max_steps":4}`))
	if err != nil || !strings.Contains(out, "ok") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCallgraphRetryContextWithVerifyFailure(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = idx.EnsureReady(context.Background())
	a := &Agent{
		evidence: readinessLedger(evidence.Receipt{
			Write: true, Success: true,
			Paths: []string{"desktop/app.go", "desktop/frontend/src/lib/useSubmit.ts"},
		}),
		projectChecks:  []instruction.VerifyCheck{{Command: "go test ./..."}},
		callgraphIndex: idx,
	}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), "broken")
	if got := a.callgraphRetryContext(); got == "" {
		t.Fatal("expected callgraph retry context")
	}
}

func TestCoordinatorRunsPlanner(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "plan: inspect files"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "executed"}, {Type: provider.ChunkDone}},
	}}
	exec := New(prov, tool.NewRegistry(), NewSession("exec"), Options{}, event.Discard)
	c := NewCoordinator(prov, NewSession("plan"), nil, exec, 0, event.Discard, nil, nil)
	if err := c.Run(context.Background(), "hello"); err != nil {
		t.Fatal(err)
	}
}

func TestTaskToolBackgroundWithJobs(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "bg done"}, {Type: provider.ChunkDone}},
	}}
	reg := tool.NewRegistry()
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "", nil)
	jm := jobs.NewManager(event.Discard)
	defer jm.Close()
	parent := &boostRecordSink{}
	ctx := jobs.WithManager(context.Background(), jm)
	ctx = withCallContext(ctx, "call-1", parent, nil, nil)
	out, err := tt.Execute(ctx, []byte(`{"prompt":"work","description":"bg task","run_in_background":true}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "Started background task") {
		t.Fatalf("out=%q", out)
	}
}

func TestTaskToolWithToolWhitelist(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{
		{{Type: provider.ChunkText, Text: "scoped"}, {Type: provider.ChunkDone}},
	}}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	reg.Add(fakeTool{name: "write_file", readOnly: false})
	tt := NewTaskTool(sub, nil, reg, 5, 0, 0, 0, 0, 0, "", "sys", nil)
	out, err := tt.Execute(context.Background(), []byte(`{"prompt":"read","tools":["read_file"]}`))
	if err != nil || !strings.Contains(out, "scoped") {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCaptureShapeWithSchemas(t *testing.T) {
	schemas := []provider.ToolSchema{{Name: "b"}, {Name: "a"}}
	shape := CaptureShape("sys", schemas, 3)
	if shape.ToolsHash == "" || shape.LogRewriteVersion != 3 {
		t.Fatalf("shape=%+v", shape)
	}
}

func TestCompareShapeLogRewrite(t *testing.T) {
	prev := PrefixShape{LogRewriteVersion: 1}
	cur := PrefixShape{LogRewriteVersion: 2}
	d := CompareShape(prev, cur, &provider.Usage{TotalTokens: 10, PromptTokens: 5, CompletionTokens: 5})
	if !d.PrefixChanged || len(d.PrefixChangeReasons) == 0 {
		t.Fatalf("diag=%+v", d)
	}
}

func TestTextSinkCompactionAndErrors(t *testing.T) {
	var b strings.Builder
	s := NewTextSink(&b, nil, 80)
	s.Emit(event.Event{Kind: event.CompactionStarted})
	s.Emit(event.Event{Kind: event.CompactionDone, Compaction: event.Compaction{
		Messages: 3, Trigger: "auto", Summary: "line1\nline2",
	}})
	s.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: "warn"})
	s.Emit(event.Event{Kind: event.ToolResult, Tool: event.Tool{Name: "bash", Err: "denied"}})
	if b.Len() == 0 {
		t.Fatal("expected output")
	}
}

func TestCompactArgsTruncatesLongJSON(t *testing.T) {
	long := strings.Repeat("x", 150)
	if got := CompactArgs(long); len([]rune(got)) > 123 {
		t.Fatalf("got len %d", len([]rune(got)))
	}
}

func TestLoadBranchMetaCorruptJSON(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "s.jsonl")
	metaPath := sessionPath + ".meta"
	if err := os.WriteFile(metaPath, []byte("{bad"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, ok, err := LoadBranchMeta(sessionPath)
	if err == nil || ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
}

func TestBranchMetaPathEmptySession(t *testing.T) {
	if got := BranchMetaPath(""); got != "" {
		t.Fatalf("got %q", got)
	}
	if got := BranchID(""); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestRunSubAgentNoFinalAnswer(t *testing.T) {
	sub := &scriptedProvider{name: "sub", turns: [][]provider.Chunk{
		{toolCallChunk("c1", "read_file", `{}`), {Type: provider.ChunkDone}},
	}}
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	_, err := RunSubAgent(context.Background(), sub, reg, "sys", "go", Options{MaxSteps: 1}, event.Discard)
	if err == nil {
		t.Fatal("expected error for missing final answer")
	}
}

func TestSubagentMetaToolsCopy(t *testing.T) {
	a := SubagentMetaTools()
	a[0] = "mutated"
	b := SubagentMetaTools()
	if b[0] == "mutated" {
		t.Fatal("expected independent copy")
	}
}

func TestListSessionsWithUserTurn(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sess.jsonl")
	s := NewSession("sys")
	s.Add(provider.Message{Role: provider.RoleUser, Content: "hello picker"})
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	sessions, err := ListSessions(dir)
	if err != nil || len(sessions) != 1 {
		t.Fatalf("sessions=%+v err=%v", sessions, err)
	}
	if sessions[0].Preview != "hello picker" {
		t.Fatalf("preview=%q", sessions[0].Preview)
	}
}

func TestNoteVerifyFailureTruncatesLongOutput(t *testing.T) {
	a := &Agent{}
	long := strings.Repeat("x", 3000)
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), long)
	_, stderr := a.latestVerifyFailureForRetry(0)
	if len(stderr) != 2000 {
		t.Fatalf("stderr len = %d", len(stderr))
	}
}

func TestNewTaskToolDefaultSystemPrompt(t *testing.T) {
	tt := NewTaskTool(nil, nil, tool.NewRegistry(), 0, 0, 0, 0, 0, 0, "", "", nil)
	if tt.sysPrompt != DefaultTaskSystemPrompt {
		t.Fatalf("sysPrompt = %q", tt.sysPrompt)
	}
}

func TestLoadSessionMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.jsonl")
	if err := os.WriteFile(path, []byte("{not-json\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadSession(path); err == nil {
		t.Fatal("expected decode error")
	}
}

func TestImportMarkerHelpers(t *testing.T) {
	dir := t.TempDir()
	if importMarkerExists("", "x") || importMarkerExists(dir, "") {
		t.Fatal("empty args should be false")
	}
	writeImportMarkers(dir, "m1", "m1", "")
	if !importMarkerExists(dir, "m1") {
		t.Fatal("expected marker file")
	}
}

func TestSaveBranchMetaTouchesUpdated(t *testing.T) {
	dir := t.TempDir()
	sessionPath := filepath.Join(dir, "s.jsonl")
	if err := SaveBranchMeta(sessionPath, BranchMeta{Name: "main"}); err != nil {
		t.Fatal(err)
	}
	loaded, ok, err := LoadBranchMeta(sessionPath)
	if err != nil || !ok || loaded.Name != "main" {
		t.Fatalf("loaded=%+v ok=%v err=%v", loaded, ok, err)
	}
}

func TestCoordinatorPlanError(t *testing.T) {
	prov := &scriptedProvider{name: "p", turns: [][]provider.Chunk{
		{{Type: provider.ChunkError, Err: errors.New("plan fail")}},
	}}
	exec := New(prov, tool.NewRegistry(), NewSession("exec"), Options{}, event.Discard)
	c := NewCoordinator(prov, NewSession("plan"), nil, exec, 0, event.Discard, nil, nil)
	if err := c.Run(context.Background(), "hello"); err == nil {
		t.Fatal("expected planner error")
	}
}

func TestFormatUsageLineWithPricingAndChurn(t *testing.T) {
	line := FormatUsageLine(
		&provider.Usage{TotalTokens: 100, PromptTokens: 80, CacheHitTokens: 60, CompletionTokens: 20, ReasoningTokens: 5},
		&provider.Pricing{Input: 1, Output: 2, Currency: "USD"},
		&CacheDiagnostics{PrefixChanged: true, PrefixChangeReasons: []string{}},
	)
	if !strings.Contains(line, "cache prefix changed") {
		t.Fatalf("line=%q", line)
	}
}

func TestFinalReadinessIncompleteTodosEmptyLabel(t *testing.T) {
	got := finalReadinessIncompleteTodos([]evidence.TodoStepMatch{
		{Index: 2, Status: "pending", Content: "  "},
	})
	if !strings.Contains(got, "todo 2: pending") {
		t.Fatalf("got %q", got)
	}
}

func TestVerificationRetryContextPendingCheck(t *testing.T) {
	writer := evidence.Receipt{ToolName: "write_file", Success: true, Write: true, Paths: []string{"a.go"}}
	a := &Agent{
		evidence:      readinessLedger(writer),
		projectChecks: []instruction.VerifyCheck{{Command: "go test ./...", Category: "unit"}},
	}
	got := a.verificationRetryContext()
	if !strings.Contains(got, "## Verification Engine") {
		t.Fatalf("got %q", got)
	}
}

func TestNoteVerifyFailureBadJSON(t *testing.T) {
	a := &Agent{}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{`}, errors.New("fail"), "output")
	cmd, stderr := a.latestVerifyFailureForRetry(0)
	if cmd != "" || stderr != "" {
		t.Fatalf("cmd=%q stderr=%q", cmd, stderr)
	}
}

func TestDependencyRetryContextNoWriter(t *testing.T) {
	root := copyDependencyTestProject(t)
	idx, err := dependency.Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	a := &Agent{
		evidence:        evidence.NewLedger(),
		projectChecks:   []instruction.VerifyCheck{{Command: "go test ./..."}},
		dependencyIndex: idx,
	}
	if got := a.dependencyRetryContext(); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestCallgraphRetryContextMergesLaterWritePaths(t *testing.T) {
	root := copyCallgraphTestProject(t)
	idx, err := callgraph.Open(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := idx.EnsureReady(context.Background()); err != nil {
		t.Fatal(err)
	}
	first := evidence.Receipt{
		ToolName: "write_file", Success: true, Write: true,
		Paths: []string{"desktop/app.go"},
	}
	second := evidence.Receipt{
		ToolName: "write_file", Success: true, Write: true,
		Paths: []string{"desktop/frontend/src/lib/useSubmit.ts", ""},
	}
	a := &Agent{
		evidence:       readinessLedger(first, second),
		projectChecks:  []instruction.VerifyCheck{{Command: "go test ./..."}},
		callgraphIndex: idx,
	}
	a.noteVerifyFailure(provider.ToolCall{Arguments: `{"command":"go test ./..."}`}, errors.New("fail"), "FAIL")
	if got := a.callgraphRetryContext(); got == "" {
		t.Fatal("expected callgraph retry context with merged write paths")
	}
}
