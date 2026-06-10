package main

import "testing"

func TestConfirmRunShellBlocksWithoutApproval(t *testing.T) {
	a := &App{}
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) { return false, nil }
	defer func() { nativeConfirmHook = nil }()

	if a.confirmRunShell("echo hi") {
		t.Fatal("expected run shell blocked")
	}
}

func TestConfirmRunShellQuietAllowsBenign(t *testing.T) {
	a := &App{}
	if !a.confirmRunShellQuiet("git status") {
		t.Fatal("benign quiet shell should pass without dialog")
	}
}

func TestConfirmRunShellQuietBlocksRisky(t *testing.T) {
	a := &App{}
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) { return false, nil }
	defer func() { nativeConfirmHook = nil }()

	if a.confirmRunShellQuiet("rm -rf /tmp/x") {
		t.Fatal("risky quiet shell should require confirm")
	}
}

func TestSetBypassRequiresConfirm(t *testing.T) {
	a := &App{tabs: map[string]*WorkspaceTab{}}
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) { return false, nil }
	defer func() { nativeConfirmHook = nil }()

	a.SetBypass(true)
	// SetModeForTab early-returns on cancelled confirm — no panic expected.
}

func TestConfirmYOLOOffSkipsDialog(t *testing.T) {
	a := &App{}
	called := false
	nativeConfirmHook = func(_ NativeConfirmRequest) (bool, error) {
		called = true
		return true, nil
	}
	defer func() { nativeConfirmHook = nil }()

	if !a.confirmYOLO(false) {
		t.Fatal("disabling YOLO should not prompt")
	}
	if called {
		t.Fatal("dialog should not run when disabling YOLO")
	}
}
