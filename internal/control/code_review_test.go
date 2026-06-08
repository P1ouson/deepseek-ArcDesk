package control

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"arcdesk/internal/tool"
)

type stubReviewTool struct {
	name string
	out  string
	err  error
	got  string
}

func (s *stubReviewTool) Name() string        { return s.name }
func (*stubReviewTool) ReadOnly() bool        { return false }
func (*stubReviewTool) Description() string   { return "stub" }
func (*stubReviewTool) Schema() json.RawMessage {
	return json.RawMessage(`{"type":"object","properties":{"task":{"type":"string"}},"required":["task"]}`)
}
func (s *stubReviewTool) Execute(_ context.Context, args json.RawMessage) (string, error) {
	var p struct {
		Task string `json:"task"`
	}
	_ = json.Unmarshal(args, &p)
	s.got = p.Task
	return s.out, s.err
}

func TestBuildCodeReviewTask(t *testing.T) {
	task := BuildCodeReviewTask("standard", "git", []string{"a.go", "b.go"})
	if !containsAll(task, "git working-tree", "a.go", "b.go", "verdict") {
		t.Fatalf("task missing expected hints: %q", task)
	}
	sec := BuildCodeReviewTask("security", "all", nil)
	if !containsAll(sec, "security issues", "Discover changed files") {
		t.Fatalf("security task missing expected hints: %q", sec)
	}
}

func TestRunCodeReviewUsesRegistryTool(t *testing.T) {
	stub := &stubReviewTool{name: "review", out: "Verdict: ship as-is\n- nit: a.go:1 — ok"}
	reg := tool.NewRegistry()
	reg.Add(stub)
	ctrl := New(Options{Registry: reg})

	got := ctrl.RunCodeReview("standard", "all", []string{"a.go"})
	if got.Err != "" {
		t.Fatalf("RunCodeReview err = %q", got.Err)
	}
	if got.Text != stub.out {
		t.Fatalf("text = %q, want %q", got.Text, stub.out)
	}
	if !containsAll(stub.got, "a.go", "verdict") {
		t.Fatalf("task passed to tool = %q", stub.got)
	}
}

func TestRunCodeReviewSecurityTool(t *testing.T) {
	stub := &stubReviewTool{name: "security_review", out: "No security issues."}
	reg := tool.NewRegistry()
	reg.Add(stub)
	ctrl := New(Options{Registry: reg})

	got := ctrl.RunCodeReview("security", "session", []string{"auth.go"})
	if got.Err != "" {
		t.Fatalf("RunCodeReview err = %q", got.Err)
	}
	if stub.got == "" {
		t.Fatal("security_review tool was not invoked")
	}
}

func TestRunCodeReviewMissingTool(t *testing.T) {
	ctrl := New(Options{Registry: tool.NewRegistry()})
	got := ctrl.RunCodeReview("standard", "all", []string{"a.go"})
	if got.Err == "" {
		t.Fatal("expected error when review tool missing")
	}
}

func containsAll(s string, parts ...string) bool {
	lower := strings.ToLower(s)
	for _, p := range parts {
		if p == "" {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(p)) {
			return false
		}
	}
	return true
}
