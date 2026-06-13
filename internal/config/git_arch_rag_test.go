package config

import "testing"

func TestGitRagShouldEnable(t *testing.T) {
	on := true
	off := false
	cfg := GitRagConfig{}
	if !cfg.ShouldEnable(true) {
		t.Fatal("default should enable in git repo")
	}
	if (GitRagConfig{Enabled: &off}).ShouldEnable(true) {
		t.Fatal("explicit off")
	}
	if !(GitRagConfig{Enabled: &on}).ShouldEnable(false) {
		t.Fatal("explicit on")
	}
	if (GitRagConfig{AutoDiscover: &off}).ShouldEnable(true) {
		t.Fatal("auto discover off")
	}
}

func TestArchRagShouldEnable(t *testing.T) {
	off := false
	cfg := ArchRagConfig{}
	if !cfg.ShouldEnable() {
		t.Fatal("default on")
	}
	if (ArchRagConfig{Enabled: &off}).ShouldEnable() {
		t.Fatal("explicit off")
	}
}
