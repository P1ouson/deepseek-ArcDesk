package verification

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"arcdesk/internal/config"
	"arcdesk/internal/instruction"
)

const discoverSource = "auto-discovery"

var frontendRoots = []string{".", "desktop/frontend", "frontend", "web"}

// DiscoverOptions controls which auto-discovered tiers are emitted.
type DiscoverOptions struct {
	IncludeUnit bool
	IncludeE2E  bool
}

// DefaultDiscoverOptions returns production defaults (unit on; E2E opt-in).
func DefaultDiscoverOptions(cfg config.VerificationConfig) DiscoverOptions {
	opts := DiscoverOptions{IncludeUnit: true, IncludeE2E: false}
	if cfg.IncludeUnit != nil {
		opts.IncludeUnit = *cfg.IncludeUnit
	}
	if cfg.IncludeE2E != nil {
		opts.IncludeE2E = *cfg.IncludeE2E
	}
	return opts
}

// Discover infers build, unit, and behavioral checks from project manifests.
func Discover(root string, cfg config.VerificationConfig) []instruction.VerifyCheck {
	root = strings.TrimSpace(root)
	if root == "" {
		return nil
	}
	opts := DefaultDiscoverOptions(cfg)
	var out []instruction.VerifyCheck

	if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
		out = append(out,
			instruction.VerifyCheck{Command: "go build ./...", SourcePath: discoverSource, Category: string(CategoryBuild)},
		)
		if opts.IncludeUnit {
			out = append(out,
				instruction.VerifyCheck{Command: "go test ./...", SourcePath: discoverSource, Category: string(CategoryUnit)},
				instruction.VerifyCheck{Command: "go vet ./...", SourcePath: discoverSource, Category: string(CategoryUnit)},
			)
		}
	}

	for _, rel := range frontendRoots {
		dir := root
		if rel != "." {
			dir = filepath.Join(root, rel)
		}
		out = append(out, discoverPackageChecks(dir, rel, opts)...)
	}
	return out
}

func discoverPackageChecks(dir, rel string, opts DiscoverOptions) []instruction.VerifyCheck {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return nil
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return nil
	}
	prefix := packageManagerPrefix(dir)
	var out []instruction.VerifyCheck

	if cmd := scriptCommand(prefix, rel, "build", pkg.Scripts["build"]); cmd != "" {
		out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryBuild)})
	}
	if cmd := scriptCommand(prefix, rel, "typecheck", pkg.Scripts["typecheck"]); cmd != "" {
		out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryBuild)})
	}

	if opts.IncludeUnit {
		for _, name := range []string{"test", "test:unit"} {
			if script, ok := pkg.Scripts[name]; ok && isUnitTestScript(script) {
				if cmd := scriptCommand(prefix, rel, name, script); cmd != "" {
					out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryUnit)})
				}
			}
		}
		if len(out) == 0 || !hasCategory(out, CategoryUnit) {
			if hasVitestConfig(dir) {
				if cmd := execCommand(prefix, rel, "vitest run --passWithNoTests"); cmd != "" {
					out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryUnit)})
				}
			}
		}
	}

	if opts.IncludeE2E && playwrightReady(dir) {
		for _, name := range []string{"test:e2e", "e2e", "playwright", "test:ui"} {
			if script, ok := pkg.Scripts[name]; ok && isE2EScript(script) {
				if cmd := scriptCommand(prefix, rel, name, script); cmd != "" {
					out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryE2E)})
				}
			}
		}
		if !hasCategory(out, CategoryE2E) && hasPlaywrightConfig(dir) {
			if cmd := execCommand(prefix, rel, "playwright test --reporter=line"); cmd != "" {
				out = append(out, instruction.VerifyCheck{Command: cmd, SourcePath: discoverSource, Category: string(CategoryE2E)})
			}
		}
	}
	return out
}

type packageJSON struct {
	Scripts          map[string]string `json:"scripts"`
	DevDependencies  map[string]string `json:"devDependencies"`
	Dependencies     map[string]string `json:"dependencies"`
}

func isUnitTestScript(script string) bool {
	s := strings.ToLower(script)
	return strings.Contains(s, "vitest") || strings.Contains(s, "jest") ||
		strings.Contains(s, "mocha") || strings.Contains(s, "node --test")
}

func isE2EScript(script string) bool {
	s := strings.ToLower(script)
	return strings.Contains(s, "playwright") || strings.Contains(s, "cypress") ||
		strings.Contains(s, "puppeteer")
}

func hasVitestConfig(dir string) bool {
	for _, name := range []string{"vitest.config.ts", "vitest.config.js", "vitest.config.mts", "vitest.config.mjs"} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func hasPlaywrightConfig(dir string) bool {
	for _, name := range []string{"playwright.config.ts", "playwright.config.js", "playwright.config.mts"} {
		if fileExists(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

// playwrightReady reports whether dir can run Playwright: @playwright/test is
// present and browser binaries exist (ms-playwright cache or PLAYWRIGHT_BROWSERS_PATH).
// Scripts that mention playwright are skipped when install has not been run.
func playwrightReady(dir string) bool {
	if !hasPlaywrightPackage(dir) {
		return false
	}
	return hasPlaywrightBrowsers()
}

func hasPlaywrightPackage(dir string) bool {
	if fileExists(filepath.Join(dir, "node_modules", "@playwright", "test")) {
		return true
	}
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return false
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return false
	}
	for _, m := range []map[string]string{pkg.DevDependencies, pkg.Dependencies} {
		for name := range m {
			if name == "@playwright/test" {
				return true
			}
		}
	}
	return false
}

func hasPlaywrightBrowsers() bool {
	cache := playwrightBrowsersDir()
	if cache == "" {
		return false
	}
	ents, err := os.ReadDir(cache)
	if err != nil || len(ents) == 0 {
		return false
	}
	for _, e := range ents {
		name := e.Name()
		if strings.HasPrefix(name, "chromium") || strings.HasPrefix(name, "firefox") || strings.HasPrefix(name, "webkit") {
			return true
		}
	}
	return false
}

func playwrightBrowsersDir() string {
	if p := os.Getenv("PLAYWRIGHT_BROWSERS_PATH"); p != "" {
		return p
	}
	if runtime.GOOS == "windows" {
		if p := os.Getenv("LOCALAPPDATA"); p != "" {
			return filepath.Join(p, "ms-playwright")
		}
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library", "Caches", "ms-playwright")
	}
	return filepath.Join(home, ".cache", "ms-playwright")
}

func hasCategory(checks []instruction.VerifyCheck, cat Category) bool {
	for _, c := range checks {
		if categoryOf(c.Category) == cat {
			return true
		}
	}
	return false
}

func scriptCommand(prefix, rel, scriptName, scriptBody string) string {
	if strings.TrimSpace(scriptBody) == "" {
		return ""
	}
	rel = filepath.ToSlash(rel)
	switch {
	case strings.HasPrefix(prefix, "pnpm"):
		if rel == "." || rel == "" {
			return "pnpm " + scriptName
		}
		return "pnpm -C " + rel + " " + scriptName
	case strings.HasPrefix(prefix, "npm run"):
		if rel == "." || rel == "" {
			return "npm run " + scriptName
		}
		return "npm run " + scriptName + " --prefix " + rel
	case strings.HasPrefix(prefix, "yarn"):
		if rel == "." || rel == "" {
			return "yarn " + scriptName
		}
		return "yarn --cwd " + rel + " " + scriptName
	case strings.HasPrefix(prefix, "bun"):
		if rel == "." || rel == "" {
			return "bun run " + scriptName
		}
		return "bun --cwd " + rel + " run " + scriptName
	default:
		return prefix + " " + scriptName
	}
}

func execCommand(prefix, rel, args string) string {
	rel = filepath.ToSlash(rel)
	switch {
	case strings.HasPrefix(prefix, "pnpm"):
		if rel == "." || rel == "" {
			return "pnpm exec " + args
		}
		return "pnpm -C " + rel + " exec " + args
	case strings.HasPrefix(prefix, "npm run"):
		if rel == "." || rel == "" {
			return "npx " + args
		}
		return "npm exec --prefix " + rel + " " + args
	case strings.HasPrefix(prefix, "yarn"):
		if rel == "." || rel == "" {
			return "yarn exec " + args
		}
		return "yarn --cwd " + rel + " exec " + args
	case strings.HasPrefix(prefix, "bun"):
		if rel == "." || rel == "" {
			return "bunx " + args
		}
		return "bun --cwd " + rel + " x " + args
	default:
		return "npx " + args
	}
}

func packageBuildCommand(dir, rel string) string {
	pkgPath := filepath.Join(dir, "package.json")
	data, err := os.ReadFile(pkgPath)
	if err != nil {
		return ""
	}
	var pkg packageJSON
	if err := json.Unmarshal(data, &pkg); err != nil {
		return ""
	}
	return scriptCommand(packageManagerPrefix(dir), rel, "build", pkg.Scripts["build"])
}

func packageManagerPrefix(dir string) string {
	if fileExists(filepath.Join(dir, "pnpm-lock.yaml")) || fileExists(filepath.Join(dir, "..", "pnpm-lock.yaml")) {
		return "pnpm"
	}
	if fileExists(filepath.Join(dir, "yarn.lock")) {
		return "yarn"
	}
	if fileExists(filepath.Join(dir, "bun.lockb")) || fileExists(filepath.Join(dir, "bun.lock")) {
		return "bun"
	}
	return "npm run"
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
