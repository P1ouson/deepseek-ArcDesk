package main

import "testing"

func TestInstallLooksSuccessful(t *testing.T) {
	if !installLooksSuccessful("Successfully installed GitHub.cli", nil) {
		t.Fatal("expected winget success")
	}
	if !installLooksSuccessful("", fmtError("found an existing package already installed")) {
		t.Fatal("expected already installed")
	}
	if installLooksSuccessful("random failure", fmtError("access denied")) {
		t.Fatal("expected failure")
	}
}

func fmtError(msg string) error {
	return &testError{msg: msg}
}

type testError struct{ msg string }

func (e *testError) Error() string { return e.msg }
