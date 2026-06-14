package control

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"arcdesk/internal/agent"
	"arcdesk/internal/diff"
	"arcdesk/internal/event"
	"arcdesk/internal/instruction"
	"arcdesk/internal/provider"
	"arcdesk/internal/rollback"
	"arcdesk/internal/tool"
)

type e2eWriteTool struct {
	root string
}

func (w e2eWriteTool) Name() string            { return "write_file" }
func (w e2eWriteTool) Description() string     { return "" }
func (w e2eWriteTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (w e2eWriteTool) ReadOnly() bool          { return false }

func (w e2eWriteTool) Preview(args json.RawMessage) (diff.Change, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	_ = json.Unmarshal(args, &in)
	oldText, _ := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(in.Path)))
	return diff.Change{Path: in.Path, Kind: diff.Modify, OldText: string(oldText), NewText: in.Content}, nil
}

func (w e2eWriteTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var in struct {
		Path    string `json:"path"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(args, &in); err != nil {
		return "", err
	}
	abs := filepath.Join(w.root, filepath.FromSlash(in.Path))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	return "written", os.WriteFile(abs, []byte(in.Content), 0o644)
}

type e2eScriptedProvider struct {
	turns [][]provider.Chunk
	call  int
}

func (p *e2eScriptedProvider) Name() string { return "p" }

func (p *e2eScriptedProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	i := p.call
	if i >= len(p.turns) {
		i = len(p.turns) - 1
	}
	p.call++
	ch := make(chan provider.Chunk, len(p.turns[i]))
	for _, c := range p.turns[i] {
		ch <- c
	}
	close(ch)
	return ch, nil
}

type noticeSink struct {
	mu     sync.Mutex
	events []event.Event
}

func (s *noticeSink) Emit(e event.Event) {
	s.mu.Lock()
	s.events = append(s.events, e)
	s.mu.Unlock()
}

func (s *noticeSink) texts() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []string
	for _, e := range s.events {
		if e.Text != "" {
			out = append(out, e.Text)
		}
	}
	return out
}

func TestControllerVerifyExhaustedAutoRollbackWithDiff(t *testing.T) {
	root := t.TempDir()
	rel := "main.go"
	good := "package main\n\nfunc main() {}\n"
	if err := os.WriteFile(filepath.Join(root, rel), []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}

	prov := &e2eScriptedProvider{turns: [][]provider.Chunk{
		{
			{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
				ID: "w1", Name: "write_file",
				Arguments: `{"path":"main.go","content":"package main\n\nfunc main() { panic(\"broken\") }\n"}`,
			}},
			{Type: provider.ChunkDone},
		},
		{{Type: provider.ChunkText, Text: "ship it 1"}, {Type: provider.ChunkDone}},
		{{Type: provider.ChunkText, Text: "ship it 2"}, {Type: provider.ChunkDone}},
	}}

	reg := tool.NewRegistry()
	reg.Add(e2eWriteTool{root: root})
	exec := agent.New(prov, reg, agent.NewSession(""), agent.Options{
		ProjectChecks:            []instruction.VerifyCheck{{Command: "go test ./..."}},
		VerifyMaxRetries:         2,
		VerifyEnforceFinalAnswer: true,
		VerifyOnFailure:          "rollback",
	}, event.Discard)

	sink := &noticeSink{}
	c := New(Options{
		Runner:          exec,
		Executor:        exec,
		Sink:            sink,
		WorkspaceRoot:   root,
		VerifyOnFailure: "rollback",
		Registry:        reg,
	})

	err := c.runTurn(context.Background(), "break main")
	if !errors.Is(err, agent.ErrVerifyExhausted) {
		t.Fatalf("runTurn err = %v, want ErrVerifyExhausted", err)
	}

	c.mu.Lock()
	turn := c.cpTurn - 1
	var report rollback.Report
	if c.cp != nil {
		report = rollback.BuildReport(c.cpRoot, c.cp.RestorePlan(turn))
	}
	c.mu.Unlock()

	if err := c.Rewind(turn, RewindBoth); err != nil {
		t.Fatalf("Rewind: %v", err)
	}
	sink.Emit(event.Event{Kind: event.Notice, Level: event.LevelWarn, Text: rollback.FormatAutoNotice(report)})

	got, err := os.ReadFile(filepath.Join(root, rel))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != good {
		t.Fatalf("restored file = %q, want %q", got, good)
	}

	joined := strings.Join(sink.texts(), "\n")
	if !strings.Contains(joined, "Reverted changes") || !strings.Contains(joined, "main.go") {
		t.Fatalf("notices=%q", joined)
	}
}
