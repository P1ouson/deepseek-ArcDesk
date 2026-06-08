package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureBundledSkillsInstallsCopywriting(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	if err := ensureBundledSkills(); err != nil {
		t.Fatal(err)
	}
	skillPath := filepath.Join(home, ".arcdesk", "skills", "copywriting", "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("SKILL.md missing: %v", err)
	}
	refPath := filepath.Join(home, ".arcdesk", "skills", "copywriting", "references", "copy-frameworks.md")
	if _, err := os.Stat(refPath); err != nil {
		t.Fatalf("reference missing: %v", err)
	}

	// Second run must not overwrite.
	if err := os.WriteFile(skillPath, []byte("custom"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := ensureBundledSkills(); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "custom" {
		t.Fatalf("ensureBundledSkills overwrote existing skill")
	}
}
