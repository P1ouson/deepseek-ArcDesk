package control

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/event"
	"arcdesk/internal/provider"
	"arcdesk/internal/tool"
)

type reuseTurnProvider struct {
	call int
}

func (p *reuseTurnProvider) Name() string { return "reuse-test" }

func (p *reuseTurnProvider) Stream(_ context.Context, _ provider.Request) (<-chan provider.Chunk, error) {
	ch := make(chan provider.Chunk, 4)
	if p.call == 0 {
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "1", Name: "read_file", Arguments: `{"path":"a.go"}`,
		}}
		ch <- provider.Chunk{Type: provider.ChunkToolCall, ToolCall: &provider.ToolCall{
			ID: "2", Name: "read_file", Arguments: `{"path":"a.go"}`,
		}}
	} else {
		ch <- provider.Chunk{Type: provider.ChunkText, Text: "ok"}
	}
	ch <- provider.Chunk{Type: provider.ChunkUsage, Usage: &provider.Usage{
		PromptTokens: 100, CompletionTokens: 10, TotalTokens: 110,
	}}
	ch <- provider.Chunk{Type: provider.ChunkDone}
	p.call++
	close(ch)
	return ch, nil
}

type fakeReadTool struct{}

func (fakeReadTool) Name() string            { return "read_file" }
func (fakeReadTool) Description() string     { return "read" }
func (fakeReadTool) Schema() json.RawMessage { return json.RawMessage(`{"type":"object"}`) }
func (fakeReadTool) ReadOnly() bool          { return true }
func (fakeReadTool) Execute(_ context.Context, _ json.RawMessage) (string, error) {
	return "file contents", nil
}

func TestControllerToolReuseTurnDoneAndGetter(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeReadTool{})

	prov := &reuseTurnProvider{}
	sess := agent.NewSession("sys")
	exec := agent.New(prov, reg, sess, agent.Options{}, event.Discard)

	done := make(chan event.Event, 1)
	c := New(Options{
		Runner:   exec,
		Executor: exec,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.TurnDone {
				done <- e
			}
		}),
	})

	c.Send("go")
	select {
	case td := <-done:
		if td.ToolReuse == nil {
			t.Fatal("TurnDone missing ToolReuse")
		}
		if td.ToolReuse.SessionCalls != 2 || td.ToolReuse.SessionDuplicates != 1 {
			t.Fatalf("TurnDone ToolReuse = %+v", td.ToolReuse)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for TurnDone")
	}

	got := c.ToolReuseStats()
	if got.SessionCalls != 2 || got.SessionDuplicates != 1 {
		t.Fatalf("ToolReuseStats() = %+v", got)
	}
}
