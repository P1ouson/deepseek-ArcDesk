package callgraph

import (
	"context"
	"errors"
	"testing"
)

func TestToolResultOrNotReady(t *testing.T) {
	msg, err := toolResultOrNotReady("", ErrIndexNotReady)
	if err != nil || msg == "" {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
	msg, err = toolResultOrNotReady("ok", nil)
	if err != nil || msg != "ok" {
		t.Fatalf("msg=%q err=%v", msg, err)
	}
	_, err = toolResultOrNotReady("", errors.New("boom"))
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCgStatusToolNotReadyMessage(t *testing.T) {
	tool := cgStatusTool{idx: &Index{root: t.TempDir()}}
	out, err := tool.Execute(context.Background(), nil)
	if err != nil || out == "" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}
