// Package intent classifies user task turns into stable canonical buckets (Phase 2).
// v1 is rule-based with zero extra model calls; consumers include auto-plan routing
// and future planning-cache keys.
package intent

import "strings"

// Class names used across routing and metrics.
const (
	ClassGeneral       = "general"
	ClassExplore       = "explore"
	ClassVerifyFailure = "verify_failure"
	ClassRefactor      = "refactor"
	ClassDependency    = "dependency"
	ClassWrite         = "write"
	ClassQA            = "qa"
)

// Result is a stable task-intent snapshot for one user turn.
type Result struct {
	Class      string
	Canonical  string
	Confidence float64
}

// Classify maps free-form user input to a canonical intent bucket.
func Classify(input string) Result {
	text := strings.TrimSpace(input)
	if text == "" {
		return Result{Class: ClassGeneral, Canonical: ClassGeneral, Confidence: 0.5}
	}
	lower := strings.ToLower(text)

	switch {
	case isWriteTurn(lower):
		return Result{Class: ClassWrite, Canonical: ClassWrite, Confidence: 0.95}
	case matchesAny(lower, verifyFailureTerms):
		return Result{Class: ClassVerifyFailure, Canonical: ClassVerifyFailure, Confidence: 0.9}
	case matchesAny(lower, refactorTerms):
		return Result{Class: ClassRefactor, Canonical: ClassRefactor, Confidence: 0.85}
	case matchesAny(lower, dependencyTerms):
		return Result{Class: ClassDependency, Canonical: ClassDependency, Confidence: 0.85}
	case matchesAny(lower, exploreTerms):
		return Result{Class: ClassExplore, Canonical: ClassExplore, Confidence: 0.85}
	case isQATurn(lower):
		return Result{Class: ClassQA, Canonical: ClassQA, Confidence: 0.8}
	default:
		return Result{Class: ClassGeneral, Canonical: ClassGeneral, Confidence: 0.6}
	}
}

func isWriteTurn(lower string) bool {
	return strings.HasPrefix(lower, "/write") ||
		strings.HasPrefix(lower, "/copywriting") ||
		strings.Contains(lower, "写作模式")
}

func isQATurn(lower string) bool {
	prefixes := []string{
		"解释", "说明", "怎么看", "查一下", "what ", "why ", "how ", "show ",
		"explain ", "describe ",
	}
	for _, p := range prefixes {
		if strings.HasPrefix(lower, p) {
			return !matchesAny(lower, refactorTerms) && !matchesAny(lower, verifyFailureTerms)
		}
	}
	return false
}

func matchesAny(s string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(s, term) {
			return true
		}
	}
	return false
}

var verifyFailureTerms = []string{
	"test fail", "tests fail", "test failed", "ci red", "ci failed", "verify fail",
	"verification fail", "lint error", "build fail", "编译失败", "测试失败", "ci 红",
	"verify 卡", "校验失败", "构建失败",
}

var refactorTerms = []string{
	"refactor", "redesign", "migrate", "restructure", "cleanup", "rename across",
	"重构", "迁移", "改造", "拆分", "合并模块",
}

var dependencyTerms = []string{
	"dependency", "import graph", "impact", "who calls", "callers of", "cycles",
	"依赖", "影响面", "调用链", "谁在用", "循环依赖",
}

var exploreTerms = []string{
	"explore", "overview", "architecture", "first open", "onboarding", "layout",
	"structure of", "快速探索", "探索", "架构", "鸟瞰", "首次打开", "项目结构",
}
