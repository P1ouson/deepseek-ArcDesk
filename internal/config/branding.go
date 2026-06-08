package config

import (
	"os"
	"path/filepath"
)

const (
	AppName     = "arcdesk"
	DisplayName = "ArcDesk"

	ConfigDirName       = "arcdesk"
	ProjectConfigFile   = "arcdesk.toml"
	ProjectMetaDir      = ".arcdesk"
	UserMemoryFile      = "ARCDESK.md"
	ProjectMemorySubdir = "memory"

	LegacyConfigDirName     = "reasonix"
	LegacyProjectConfigFile = "reasonix.toml"
	LegacyProjectMetaDir    = ".reasonix"
	LegacyUserMemoryFile    = "REASONIX.md"
	LegacyHomeDir           = ".reasonix"
)

func userConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, ConfigDirName)
}

func legacyUserConfigDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, LegacyConfigDirName)
}

func projectConfigPathForRoot(root string) string {
	root = resolveRoot(root)
	if root == "." {
		return ProjectConfigFile
	}
	return filepath.Join(root, ProjectConfigFile)
}

// ProjectConfigPathForRoot returns the project-level config file path for root.
func ProjectConfigPathForRoot(root string) string {
	return projectConfigPathForRoot(root)
}

func legacyProjectConfigPathForRoot(root string) string {
	root = resolveRoot(root)
	if root == "." {
		return LegacyProjectConfigFile
	}
	return filepath.Join(root, LegacyProjectConfigFile)
}

func projectMetaPath(root, leaf string) string {
	root = resolveRoot(root)
	if root == "." {
		return filepath.Join(ProjectMetaDir, leaf)
	}
	return filepath.Join(root, ProjectMetaDir, leaf)
}

func legacyProjectMetaPath(root, leaf string) string {
	root = resolveRoot(root)
	if root == "." {
		return filepath.Join(LegacyProjectMetaDir, leaf)
	}
	return filepath.Join(root, LegacyProjectMetaDir, leaf)
}

func firstExistingPath(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}