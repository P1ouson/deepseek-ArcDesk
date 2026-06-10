package main

import (
	"testing"

	"arcdesk/internal/control"
	"arcdesk/internal/event"
)

func TestClawDecisionSinkBroadcastsApproval(t *testing.T) {
	app := &App{mobileDecision: newMobileDecisionStore()}
	sink := &clawDecisionSink{app: app}
	sink.Emit(event.Event{
		Kind: event.ApprovalRequest,
		Approval: event.Approval{
			ID: "42", Tool: "bash", Subject: "rm -rf /tmp/x",
		},
	})

	pending := app.GetMobilePendingDecision()
	if pending == nil {
		t.Fatal("want pending mobile decision")
	}
	if pending.TabID != clawDecisionTabID {
		t.Fatalf("tabId = %q, want %q", pending.TabID, clawDecisionTabID)
	}
	if pending.ID != "42" || pending.Kind != "approval" || pending.Tool != "bash" {
		t.Fatalf("pending = %+v, want claw approval", pending)
	}
}

func TestClawDecisionSinkClearsOnTurnDone(t *testing.T) {
	app := &App{mobileDecision: newMobileDecisionStore()}
	sink := &clawDecisionSink{app: app}
	sink.Emit(event.Event{
		Kind: event.ApprovalRequest,
		Approval: event.Approval{ID: "1", Tool: "bash"},
	})
	sink.Emit(event.Event{Kind: event.TurnDone})
	if app.GetMobilePendingDecision() != nil {
		t.Fatal("TurnDone should clear pending decision")
	}
}

func TestRespondMobileDecisionClawTabDoesNotRequireWorkspaceTab(t *testing.T) {
	app := &App{mobileDecision: newMobileDecisionStore()}
	ctrl := control.New(control.Options{Sink: event.Discard})
	app.setClawRunCtrl(ctrl)

	app.mobileDecision.set(&MobilePendingDecision{
		Kind: "approval", ID: "7", TabID: clawDecisionTabID, Tool: "bash",
	})
	if err := app.RespondMobileDecision("7", false, nil); err != nil {
		t.Fatalf("RespondMobileDecision: %v", err)
	}
	if app.GetMobilePendingDecision() != nil {
		t.Fatal("pending should be cleared")
	}
}

func TestEnableDesktopInteractiveNilSafe(t *testing.T) {
	enableDesktopInteractive(nil)
}
