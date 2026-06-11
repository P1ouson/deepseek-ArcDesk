package builtin

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const (
	readFileLimitSmall      = 250
	readFileLimitMedium     = 400
	readFileLimitLargeEntry = 600
	readFileLimitLargeImpl  = 280

	readFileSmartExpandMedium = 600
	readFileSmartExpandLarge  = 600

	readFileTierSmallMaxLOC  = 10_000
	readFileTierMediumMaxLOC = 80_000

	readFileWholeFileMaxLines = 300
	readFileWholeFileMaxBytes = 12 * 1024

	// readFileSmartExpandMinOffset: after two 250-line pages, bump the next chunk.
	readFileSmartExpandMinOffset = 2 * readFileLimitSmall
)

type repoTier int

const (
	repoTierSmall repoTier = iota
	repoTierMedium
	repoTierLarge
)

var (
	repoTierCache    sync.Map // workDir -> repoTier
	repoTierOverride func(workDir string) (repoTier, bool)
)

func setRepoTierOverride(fn func(string) (repoTier, bool)) {
	repoTierOverride = fn
}

func repoTierForWorkDir(workDir string) repoTier {
	if repoTierOverride != nil {
		if tier, ok := repoTierOverride(workDir); ok {
			return tier
		}
	}
	root := workDir
	if root == "" {
		var err error
		root, err = os.Getwd()
		if err != nil {
			return repoTierSmall
		}
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return repoTierSmall
	}
	if v, ok := repoTierCache.Load(root); ok {
		return v.(repoTier)
	}
	tier := scanRepoTier(root)
	repoTierCache.Store(root, tier)
	return tier
}

func scanRepoTier(root string) repoTier {
	loc := countRepoLOC(root)
	switch {
	case loc >= readFileTierMediumMaxLOC:
		return repoTierLarge
	case loc >= readFileTierSmallMaxLOC:
		return repoTierMedium
	default:
		return repoTierSmall
	}
}

func countRepoLOC(root string) int {
	var loc int
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if _, skip := readLimitSkipDir[d.Name()]; skip {
				return filepath.SkipDir
			}
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if _, ok := readLimitCodeExts[ext]; !ok {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			loc++
			if loc >= readFileTierMediumMaxLOC {
				_ = f.Close()
				return fs.SkipAll
			}
		}
		_ = f.Close()
		return nil
	})
	return loc
}

var readLimitCodeExts = map[string]struct{}{
	".go": {}, ".ts": {}, ".tsx": {}, ".js": {}, ".jsx": {},
	".py": {}, ".rs": {}, ".java": {}, ".kt": {}, ".cs": {},
	".cpp": {}, ".c": {}, ".h": {}, ".hpp": {}, ".swift": {},
	".rb": {}, ".php": {}, ".vue": {}, ".css": {}, ".scss": {},
	".html": {}, ".md": {},
}

var readLimitSkipDir = map[string]struct{}{
	"node_modules": {}, "vendor": {}, ".git": {}, "dist": {}, "build": {},
	"__pycache__": {}, ".next": {}, "target": {}, "coverage": {},
}

var readLimitEntryNames = map[string]struct{}{
	"package.json": {}, "package-lock.json": {}, "pnpm-lock.yaml": {},
	"go.mod": {}, "go.sum": {}, "cargo.toml": {}, "cargo.lock": {},
	"readme": {}, "readme.md": {}, "readme.txt": {},
	"tsconfig.json": {}, "jsconfig.json": {},
	"vite.config.ts": {}, "vite.config.js": {}, "vite.config.mjs": {},
	"webpack.config.js": {}, "rollup.config.js": {},
	"settings.json": {}, "config.toml": {}, "config.json": {},
	"pyproject.toml": {}, "setup.py": {}, "makefile": {},
	"dockerfile": {}, "docker-compose.yml": {}, "docker-compose.yaml": {},
}

// readLimitAggressiveEntryNames get the wide first-page limit on large repos.
var readLimitAggressiveEntryNames = map[string]struct{}{
	"package.json": {},
	"tsconfig.json": {}, "jsconfig.json": {},
	"config.toml": {}, "config.json": {}, "settings.json": {},
	"vite.config.ts": {}, "vite.config.js": {}, "vite.config.mjs": {},
}

var readLimitAggressiveEntryBaseNames = map[string]struct{}{
	"main.go": {}, "main.ts": {}, "main.tsx": {}, "main.py": {},
	"app.go": {}, "app.ts": {}, "app.tsx": {}, "app.jsx": {},
}

var readLimitStructureBaseNames = map[string]struct{}{
	"main.rs": {},
	"index.ts": {}, "index.tsx": {}, "index.js": {},
	"router.go": {}, "router.ts": {}, "router.tsx": {},
	"routes.go": {}, "routes.ts": {}, "routes.tsx": {},
	"bootstrap.go": {}, "bootstrap.ts": {},
}

func isAggressiveEntryFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if _, ok := readLimitAggressiveEntryNames[base]; ok {
		return true
	}
	if _, ok := readLimitAggressiveEntryBaseNames[base]; ok {
		return true
	}
	return false
}

func isStructureFile(path string) bool {
	base := strings.ToLower(filepath.Base(path))
	if _, ok := readLimitEntryNames[base]; ok {
		return true
	}
	if _, ok := readLimitStructureBaseNames[base]; ok {
		return true
	}
	lower := strings.ToLower(filepath.ToSlash(path))
	for segment := range readLimitEntryPathSegments {
		if strings.Contains(lower, segment) {
			return true
		}
	}
	return false
}

func isEntryFile(path string) bool {
	return isAggressiveEntryFile(path) || isStructureFile(path)
}

var readLimitEntryPathSegments = map[string]struct{}{
	"/router/": {}, "/routes/": {}, "/cmd/": {},
}

func isLargeImplementationFile(path string, info os.FileInfo) bool {
	if isEntryFile(path) {
		return false
	}
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".cs", ".cpp", ".c":
	default:
		return false
	}
	if info != nil && info.Size() > 24*1024 {
		return true
	}
	return false
}

func effectiveReadLimit(workDir, path string, offset int, explicitLimit int) int {
	if explicitLimit > 0 {
		return explicitLimit
	}
	if offset == 0 {
		if limit, ok := wholeFileReadLimit(path); ok {
			return limit
		}
	}
	tier := repoTierForWorkDir(workDir)
	limit := adaptiveReadLimit(path, offset, tier)
	if offset >= readFileSmartExpandMinOffset {
		limit = maxInt(limit, smartExpandLimit(tier))
	}
	return limit
}

func wholeFileReadLimit(path string) (int, bool) {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() || info.Size() > readFileWholeFileMaxBytes {
		return 0, false
	}
	lines, err := countFileLines(path)
	if err != nil || lines <= 0 || lines > readFileWholeFileMaxLines {
		return 0, false
	}
	return lines, true
}

func countFileLines(path string) (int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	n := 0
	for sc.Scan() {
		n++
	}
	return n, sc.Err()
}

func adaptiveReadLimit(path string, offset int, tier repoTier) int {
	switch tier {
	case repoTierSmall:
		return readFileLimitSmall
	case repoTierMedium:
		if offset == 0 && isEntryFile(path) {
			return readFileLimitMedium
		}
		return readFileLimitSmall
	default: // large
		if offset == 0 && isAggressiveEntryFile(path) {
			return readFileLimitLargeEntry
		}
		info, _ := os.Stat(path)
		if isLargeImplementationFile(path, info) {
			return readFileLimitLargeImpl
		}
		if offset == 0 && isStructureFile(path) {
			return readFileLimitMedium
		}
		if offset == 0 {
			return readFileLimitMedium
		}
		return readFileLimitSmall
	}
}

func smartExpandLimit(tier repoTier) int {
	switch tier {
	case repoTierLarge:
		return readFileSmartExpandLarge
	case repoTierMedium:
		return readFileSmartExpandMedium
	default:
		return readFileLimitMedium
	}
}

func defaultLimitHint(workDir string) int {
	switch repoTierForWorkDir(workDir) {
	case repoTierLarge:
		return readFileLimitLargeEntry
	case repoTierMedium:
		return readFileLimitMedium
	default:
		return readFileLimitSmall
	}
}

func readFileDescription(workDir string) string {
	const base = "Read a text file with optional line offset/limit. Output prefixes each line with its 1-based number (e.g. `   42→...`) so subsequent edit_file calls can target exact lines. "
	switch repoTierForWorkDir(workDir) {
	case repoTierSmall:
		return base + "Files under 300 lines are returned whole. Page larger files with offset/limit."
	case repoTierMedium:
		return base + "When exploring, read structure files (README, package.json, go.mod, main/app entrypoints) before large implementation files. " +
			"Files under 300 lines are returned whole. Page larger files with offset/limit (first page ~400 lines on entry/config files)."
	default:
		hint := defaultLimitHint(workDir)
		return base + "When exploring a new repo, read structure files first (README, package.json, go.mod, main/app entrypoints, config) before large implementation files. " +
			fmt.Sprintf("Page files larger than ~%d lines with offset/limit; entry/config files may start wider. ", hint) +
			"After two 250-line pages of the same file, the next page widens automatically."
	}
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
