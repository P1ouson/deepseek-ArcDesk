package control

import (
	"context"
	"os"
	"strings"
	"testing"

	"arcdesk/internal/agent"
	"arcdesk/internal/event"
	"arcdesk/internal/permission"
	"arcdesk/internal/skill"
	"arcdesk/internal/tool"
)

func TestAttachmentHelpersCoverage(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("src.png", mustBase64(t, tinyPNG), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveImageFile("src.png"); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile("doc.txt", []byte("hello doc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := SaveAttachmentFile("doc.txt"); err != nil {
		t.Fatal(err)
	}
	path, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	url, err := ImageDataURL(path)
	if err != nil || !strings.HasPrefix(url, "data:") {
		t.Fatalf("url=%q err=%v", url, err)
	}
	if _, err := ImageDataURL("../outside.png"); err == nil {
		t.Fatal("outside path should fail")
	}
	if err := ensureAttachmentRoot(); err != nil {
		t.Fatal(err)
	}
	got, err := SaveImageDataURL("data:image/png;base64," + tinyPNG)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cleanAttachmentPath(got); err != nil {
		t.Fatal(err)
	}
}

func TestSlashManagementSkillsDisable(t *testing.T) {
	var notices []string
	c := New(Options{
		Skills: []skill.Skill{{Name: "explore", Scope: skill.ScopeBuiltin}},
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.managementNotice("/skills enable missing-skill")
	if !strings.Contains(strings.Join(notices, "\n"), "skill enable") {
		t.Fatalf("notices=%v", notices)
	}
}

func TestSubmitWhileBusyEmitsNotice(t *testing.T) {
	var notices []string
	c := New(Options{Sink: event.FuncSink(func(e event.Event) {
		if e.Text != "" {
			notices = append(notices, e.Text)
		}
	})})
	c.mu.Lock()
	c.running = true
	c.mu.Unlock()
	c.Submit("hello while busy")
	if len(notices) == 0 {
		t.Fatal("expected busy notice")
	}
}

func TestGateOnRememberViaExecutor(t *testing.T) {
	var remembered string
	ids := make(chan string, 1)
	reg := tool.NewRegistry()
	reg.Add(fakeControlTool{name: "bash"})
	exec := agent.New(nil, reg, agent.NewSession("sys"), agent.Options{}, event.Discard)
	c := New(Options{
		Executor: exec,
		Sink: event.FuncSink(func(e event.Event) {
			if e.Kind == event.ApprovalRequest {
				ids <- e.Approval.ID
			}
		}),
		Policy: permission.New("default", nil, []string{"bash(*)"}, nil),
		OnRemember: func(rule string) {
			remembered = rule
		},
	})
	c.EnableInteractiveApproval()
	go func() {
		id := <-ids
		c.Approve(id, true, false, true)
	}()
	gate := permission.NewGate(c.policy, gateApprover{c})
	gate.OnRemember = c.onRemember
	allow, _, err := gate.Check(context.Background(), "bash", []byte(`{"command":"go test"}`), false)
	if err != nil || !allow {
		t.Fatalf("check = %v err=%v", allow, err)
	}
	if remembered == "" {
		t.Fatalf("remembered = %q", remembered)
	}
}
