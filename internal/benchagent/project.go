package benchagent

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var codeExts = map[string]struct{}{
	".go": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {},
	".py": {}, ".rs": {}, ".java": {}, ".kt": {}, ".cs": {},
	".cpp": {}, ".c": {}, ".h": {}, ".hpp": {}, ".swift": {},
	".rb": {}, ".php": {}, ".vue": {}, ".css": {}, ".scss": {},
	".html": {}, ".md": {},
}

var skipDir = map[string]struct{}{
	"node_modules": {}, "vendor": {}, ".git": {}, "dist": {}, "build": {},
	"__pycache__": {}, ".next": {}, "target": {}, "coverage": {},
}

// ScanProjectSize counts code files and lines under root (best-effort).
func ScanProjectSize(root string) ProjectSize {
	var ps ProjectSize
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := skipDir[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := codeExts[ext]; !ok {
			return nil
		}
		ps.Files++
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			ps.LOC++
		}
		_ = f.Close()
		return nil
	})
	return ps
}
