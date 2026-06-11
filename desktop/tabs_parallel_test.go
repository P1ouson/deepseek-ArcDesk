package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"arcdesk/internal/agent"
	"arcdesk/internal/control"
	"arcdesk/internal/event"
)

type tabBlockingRunner struct {
	once    sync.Once
	started chan struct{}
	release chan struct{}
}

func (r tabBlockingRunner) Run(ctx context.Context, _ string) error {
	r.once.Do(func() { close(r.started) })
	select {
	case <-r.release:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestParallelTabSubmitWhileOtherRunning(t *testing.T) {
	app := NewApp()

	makeReadyTab := func(id string, runner tabBlockingRunner) *WorkspaceTab {
		sess := agent.NewSession("sys")
		exec := agent.New(nil, nil, sess, agent.Options{}, event.Discard)
		ctrl := control.New(control.Options{
			Runner:   runner,
			Executor: exec,
			Sink:     event.Discard,
		})
		tab := &WorkspaceTab{
			ID:    id,
			Scope: "global",
			Ctrl:  ctrl,
			Ready: true,
		}
		app.mu.Lock()
		app.tabs[id] = tab
		app.tabOrder = append(app.tabOrder, id)
		app.mu.Unlock()
		return tab
	}

	runnerA := tabBlockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	runnerB := tabBlockingRunner{started: make(chan struct{}), release: make(chan struct{})}
	tabA := makeReadyTab("tab_a", runnerA)
	tabB := makeReadyTab("tab_b", runnerB)
	defer func() {
		if tabA.Ctrl != nil {
			tabA.Ctrl.Close()
		}
		if tabB.Ctrl != nil {
			tabB.Ctrl.Close()
		}
	}()

	if err := app.SubmitToTab("tab_a", "first"); err != nil {
		t.Fatalf("submit tab_a: %v", err)
	}
	select {
	case <-runnerA.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for tab_a to start")
	}

	if err := app.SubmitToTab("tab_b", "second"); err != nil {
		t.Fatalf("submit tab_b while tab_a running: %v", err)
	}
	select {
	case <-runnerB.started:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for tab_b to start")
	}

	if !tabA.Ctrl.Running() || !tabB.Ctrl.Running() {
		t.Fatalf("both tabs should be running: a=%v b=%v", tabA.Ctrl.Running(), tabB.Ctrl.Running())
	}

	close(runnerA.release)
	waitNotRunning(t, tabA.Ctrl)
	if !tabB.Ctrl.Running() {
		t.Fatal("tab_b should still be running after tab_a finishes")
	}

	close(runnerB.release)
	waitNotRunning(t, tabB.Ctrl)
}
