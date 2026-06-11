package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

func TestMessageDialogConfirmedWindowsQuestion(t *testing.T) {
	dt := runtime.QuestionDialog
	if !messageDialogConfirmed("Yes", "Allow LAN access", dt, true) {
		t.Fatal("Yes should confirm on Windows question dialog")
	}
	if messageDialogConfirmed("No", "Allow LAN access", dt, true) {
		t.Fatal("No should cancel on Windows question dialog")
	}
	if messageDialogConfirmed("Allow LAN access", "Allow LAN access", dt, true) {
		t.Fatal("custom label must not match on Windows question dialog")
	}
}

func TestMessageDialogConfirmedWindowsWarning(t *testing.T) {
	dt := runtime.WarningDialog
	if !messageDialogConfirmed("Ok", "Delete", dt, true) {
		t.Fatal("Ok should confirm on Windows warning dialog")
	}
}

func TestMessageDialogConfirmedCustomLabelsNonWindows(t *testing.T) {
	dt := runtime.QuestionDialog
	if !messageDialogConfirmed("Allow LAN access", "Allow LAN access", dt, false) {
		t.Fatal("custom confirm label should match on non-Windows")
	}
	if messageDialogConfirmed("Yes", "Allow LAN access", dt, false) {
		t.Fatal("Yes should not match custom confirm label on non-Windows")
	}
}
