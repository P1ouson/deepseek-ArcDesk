package main

import (
	"testing"

	"arcdesk/internal/control"
	"arcdesk/internal/runtime"
)

func TestIngestRuntimeEmptyPayload(t *testing.T) {
	app := &App{tabs: map[string]*WorkspaceTab{}}
	if err := app.IngestRuntime("missing", ""); err != nil {
		t.Fatal(err)
	}
}

func TestRuntimeHubForTab(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	ctrl := control.New(control.Options{
		RuntimeHub: hub,
	})
	app := &App{tabs: map[string]*WorkspaceTab{
		"t1": {Ctrl: ctrl},
	}}
	if got := app.runtimeHubForTab("t1"); got != hub {
		t.Fatal("expected hub")
	}
	if got := app.runtimeHubForTab("missing"); got != nil {
		t.Fatal("expected nil")
	}
}

func TestIngestRuntimeBatch(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	ctrl := control.New(control.Options{RuntimeHub: hub})
	app := &App{tabs: map[string]*WorkspaceTab{"t1": {Ctrl: ctrl}}}
	payload := `[{"kind":"console","level":"error","message":"boom"}]`
	if err := app.IngestRuntime("t1", payload); err != nil {
		t.Fatal(err)
	}
	if hub.Stats().ByKind[runtime.KindConsole] != 1 {
		t.Fatalf("stats = %+v", hub.Stats())
	}
}

func TestIngestTerminalOutput(t *testing.T) {
	hub := runtime.NewHub(runtime.DefaultLimits())
	ctrl := control.New(control.Options{RuntimeHub: hub})
	app := &App{tabs: map[string]*WorkspaceTab{"t1": {Ctrl: ctrl}}}
	app.ingestTerminalOutput("term-1", []byte("hello\n"))
	if hub.Stats().ByKind[runtime.KindGoLog] != 1 {
		t.Fatalf("stats = %+v", hub.Stats())
	}
}
