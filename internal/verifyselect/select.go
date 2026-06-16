package verifyselect

import (
	"path/filepath"
	"strings"

	"arcdesk/internal/verification"
)

// MinimumChecks returns a subset of plan checks appropriate for the changed paths.
// When no paths are given or selection would drop everything, the full plan is kept.
func MinimumChecks(plan verification.Plan, changedPaths []string) verification.Plan {
	if len(plan.Checks) == 0 || len(changedPaths) == 0 {
		return plan
	}
	profile := classifyChanges(changedPaths)
	if profile == profileUnknown {
		return plan
	}
	var out []verification.Check
	for _, c := range plan.Checks {
		if keepCheck(profile, c) {
			out = append(out, c)
		}
	}
	if len(out) == 0 {
		return plan
	}
	return verification.Plan{Checks: out, Policy: plan.Policy}
}

type changeProfile int

const (
	profileUnknown changeProfile = iota
	profileDocsOnly
	profileStyleOnly
	profileFrontend
	profileBackend
	profileDeps
)

func classifyChanges(paths []string) changeProfile {
	if len(paths) == 0 {
		return profileUnknown
	}
	hasCode := false
	allDocs := true
	allStyle := true
	hasDeps := false
	for _, p := range paths {
		p = normalize(p)
		if p == "" {
			continue
		}
		ext := strings.ToLower(filepath.Ext(p))
		base := strings.ToLower(filepath.Base(p))
		switch {
		case isDepFile(base):
			hasDeps = true
			allDocs = false
			allStyle = false
		case ext == ".md" || ext == ".txt" || base == "changelog" || strings.Contains(base, "readme"):
			// docs only candidate
		case ext == ".css" || ext == ".scss" || ext == ".less":
			allDocs = false
		default:
			allDocs = false
			allStyle = false
			hasCode = true
		}
		if ext != ".css" && ext != ".scss" && ext != ".less" {
			allStyle = false
		}
	}
	if hasDeps {
		return profileDeps
	}
	if allDocs {
		return profileDocsOnly
	}
	if allStyle && !hasCode {
		return profileStyleOnly
	}
	for _, p := range paths {
		p = normalize(p)
		if strings.HasPrefix(p, "desktop/frontend/") || isFrontendExt(p) {
			return profileFrontend
		}
	}
	if hasCode {
		return profileBackend
	}
	return profileUnknown
}

func keepCheck(profile changeProfile, c verification.Check) bool {
	switch profile {
	case profileDocsOnly:
		return c.Category == verification.CategoryCustom || c.Category == verification.CategoryBuild
	case profileStyleOnly:
		return c.Category == verification.CategoryBuild || c.Category == verification.CategoryCustom
	case profileFrontend:
		return c.Category != verification.CategoryE2E
	case profileBackend:
		return c.Category != verification.CategoryE2E
	case profileDeps:
		return true
	default:
		return true
	}
}

func isDepFile(base string) bool {
	switch base {
	case "go.mod", "go.sum", "package.json", "pnpm-lock.yaml", "package-lock.json", "yarn.lock", "bun.lock", "bun.lockb":
		return true
	}
	return false
}

func isFrontendExt(path string) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".tsx", ".jsx", ".vue", ".css", ".scss", ".html":
		return true
	}
	return false
}

func normalize(p string) string {
	return strings.TrimSpace(strings.ReplaceAll(p, "\\", "/"))
}
