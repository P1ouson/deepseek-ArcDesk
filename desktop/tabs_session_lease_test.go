package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/config"
	"arcdesk/internal/control"
)

func TestSessionLeaseBlocksSecondTab(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	projectRoot := t.TempDir()
	shared := writeScopedSession(t, dir, "shared-project.jsonl", "project", projectRoot, "warm", time.Now())

	app := &App{
		tabs:          map[string]*WorkspaceTab{},
		sessionLeases: map[string]string{},
	}
	if !app.claimSessionPath("tab_a", shared) {
		t.Fatal("first claim should succeed")
	}

	tabB := &WorkspaceTab{ID: "tab_b", Scope: "project", WorkspaceRoot: projectRoot, TopicID: "topic_b"}
	ctrlB := control.New(control.Options{SessionDir: dir, Label: "test"})
	defer ctrlB.Close()

	got := app.resumeTabSession(ctrlB, tabB, dir)
	if got != "" {
		t.Fatalf("resume should refuse leased path, got %q", got)
	}
	if tabB.leaseNotice == "" {
		t.Fatal("expected lease notice on tab")
	}
	if ctrlB.SessionPath() != "" {
		t.Fatalf("controller should not resume leased session, path=%q", ctrlB.SessionPath())
	}

	app.releaseSessionLease("tab_a")
	if !app.claimSessionPath("tab_b", shared) {
		t.Fatal("claim should succeed after release")
	}
}

func TestClaimSessionPathNormalizesViaValidate(t *testing.T) {
	isolateDesktopUserDirs(t)
	dir := config.SessionDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "one.jsonl")
	if err := agent.NewSession("sys").Save(path); err != nil {
		t.Fatal(err)
	}
	app := &App{sessionLeases: map[string]string{}}
	if !app.claimSessionPath("tab1", filepath.Base(path)) {
		t.Fatal("claim by basename should resolve")
	}
	if owner := app.sessionLeases[path]; owner != "tab1" {
		t.Fatalf("lease owner = %q want tab1 (keys=%v)", owner, app.sessionLeases)
	}
}
