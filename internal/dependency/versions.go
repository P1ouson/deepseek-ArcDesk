package dependency

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// detectVersionConflicts reports duplicate require versions across go.mod files.
// Phase 1: require duplicates only; replace directives are not resolved.
func detectVersionConflicts(root string, g *Graph) []VersionConflict {
	_ = g
	modFiles, err := findGoModFiles(root)
	if err != nil || len(modFiles) == 0 {
		return nil
	}

	type versionEntry struct {
		version string
		file    string
	}
	byModule := map[string][]versionEntry{}

	for _, modFile := range modFiles {
		info, err := parseGoMod(modFile)
		if err != nil {
			continue
		}
		rel, _ := filepath.Rel(root, modFile)
		rel = normalizeSlash(rel)
		for _, req := range info.Requires {
			byModule[req.Path] = append(byModule[req.Path], versionEntry{
				version: req.Version,
				file:    rel,
			})
		}
	}

	var conflicts []VersionConflict
	for module, entries := range byModule {
		versions := map[string][]string{}
		for _, e := range entries {
			versions[e.version] = append(versions[e.version], e.file)
		}
		if len(versions) <= 1 {
			continue
		}
		var vers []string
		var paths []string
		for v, files := range versions {
			vers = append(vers, v)
			paths = append(paths, files...)
		}
		sort.Strings(vers)
		sort.Strings(paths)
		conflicts = append(conflicts, VersionConflict{
			Module:   module,
			Versions: vers,
			Paths:    paths,
			Message:  fmt.Sprintf("module %q required as %s", module, strings.Join(vers, " vs ")),
		})
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Module < conflicts[j].Module })
	return conflicts
}

func findGoModFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if goWalkSkipDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if d.Name() == "go.mod" {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}
