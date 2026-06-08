package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

//go:embed bundled-skills/copywriting/SKILL.md
//go:embed bundled-skills/copywriting/SOURCE.md
//go:embed bundled-skills/copywriting/references/copy-frameworks.md
//go:embed bundled-skills/copywriting/references/natural-transitions.md
var bundledSkills embed.FS

const bundledCopywritingSkill = "copywriting"

// ensureBundledSkills installs shipped skills into the user's global skills directory.
// Skips when SKILL.md already exists so manual edits are not overwritten.
func ensureBundledSkills() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dest := filepath.Join(home, ".arcdesk", "skills", bundledCopywritingSkill)
	if _, err := os.Stat(filepath.Join(dest, "SKILL.md")); err == nil {
		return nil
	}
	return copyEmbeddedSkillTree(bundledSkills, "bundled-skills/"+bundledCopywritingSkill, dest)
}

func copyEmbeddedSkillTree(root fs.FS, srcRoot, destRoot string) error {
	return fs.WalkDir(root, srcRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcRoot, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		target := filepath.Join(destRoot, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(root, path)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return err
		}
		slog.Info("installed bundled skill", "path", target)
		return nil
	})
}
