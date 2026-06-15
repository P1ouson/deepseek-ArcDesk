package main

import "testing"

func TestIsDetachedGhShellCommand(t *testing.T) {
	cases := []struct {
		cmd  string
		want bool
	}{
		{"gh --version", true},
		{"gh auth status", true},
		{"gh pr view --json number", true},
		{`"C:\Program Files\GitHub CLI\gh.exe" --version`, true},
		{"git status", false},
		{"winget install gh", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := isDetachedGhShellCommand(tc.cmd); got != tc.want {
			t.Fatalf("isDetachedGhShellCommand(%q) = %v, want %v", tc.cmd, got, tc.want)
		}
	}
}

func TestRunShellQuietDetachedGhWithoutSession(t *testing.T) {
	isolateDesktopUserDirs(t)
	app := NewApp()
	if app.activeCtrl() != nil {
		t.Fatal("expected no active controller for isolated app")
	}
	got := app.RunShellQuiet("gh --version")
	if got.Err == "no active session" {
		t.Fatalf("RunShellQuiet gh without session should use detached shell, got %q", got.Err)
	}
	// gh may or may not be installed on the CI machine; either outcome is fine.
	if got.Err != "" && got.Output == "" && got.Err != "cancelled" {
		t.Logf("gh probe without install: err=%q output=%q", got.Err, got.Output)
	}
}
