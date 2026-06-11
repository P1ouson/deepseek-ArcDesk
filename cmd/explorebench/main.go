// explorebench runs first-project exploration benchmarks with dev-only
// instrumentation (BENCHMARK_AGENT=1). It writes JSON reports under
// desktop/benchmarks/.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"arcdesk/internal/benchagent"
	"arcdesk/internal/boot"
	"arcdesk/internal/event"
)

type scenario struct {
	Label  string
	Dir    string
	Prompt string
	Steps  int
}

func main() {
	variant := flag.String("variant", "after", "label for this build (before|after)")
	outDir := flag.String("out", "", "output directory (default desktop/benchmarks)")
	model := flag.String("model", "", "provider/model (default config)")
	only := flag.String("only", "", "run one scenario label: small|medium|large|coding")
	flag.Parse()

	os.Setenv("BENCHMARK_AGENT", "1")
	benchagent.ResetGlobal()

	scenarios := []scenario{
		{
			Label:  "small",
			Dir:    filepath.Join("internal", "agent"),
			Prompt: "快速探索此项目：找出主入口、核心模块和测试布局。先读最小必要文件，不要一次读整文件。",
			Steps:  24,
		},
		{
			Label:  "medium",
			Dir:    filepath.Join("desktop", "frontend", "src"),
			Prompt: "探索这个前端项目：说明路由/页面结构、状态管理和与 desktop 后端的集成点。优先 grep/glob，再按需 read_file。",
			Steps:  30,
		},
		{
			Label:  "large",
			Dir:    ".",
			Prompt: "首次打开此 monorepo：给出架构鸟瞰（desktop/agent/provider/tools），指出首次探索应优先读的 5 个文件。不要试图读完所有代码。",
			Steps:  36,
		},
		{
			Label:  "coding",
			Dir:    filepath.Join("benchmarks", "e2e", "tasks", "fix-add-bug", "workdir"),
			Prompt: "The file calc.py has a bug: add(a, b) returns the wrong result. Fix add so it returns the sum of a and b. Keep the other functions unchanged.",
			Steps:  12,
		},
	}

	repoRoot, err := os.Getwd()
	if err != nil {
		fatal(err)
	}
	absOut := *outDir
	if absOut == "" {
		absOut = filepath.Join(repoRoot, "desktop", "benchmarks")
	} else if !filepath.IsAbs(absOut) {
		absOut = filepath.Join(repoRoot, absOut)
	}
	if err := os.MkdirAll(absOut, 0o755); err != nil {
		fatal(err)
	}

	var paths []string
	for _, sc := range scenarios {
		if *only != "" && sc.Label != *only {
			continue
		}
		absDir := sc.Dir
		if !filepath.IsAbs(absDir) {
			absDir = filepath.Join(repoRoot, absDir)
		}
		if st, err := os.Stat(absDir); err != nil || !st.IsDir() {
			fatal(fmt.Errorf("scenario %s dir missing: %s", sc.Label, absDir))
		}
		path, err := runScenario(sc, absDir, repoRoot, *variant, *model, absOut)
		if err != nil {
			fmt.Fprintf(os.Stderr, "scenario %s failed: %v\n", sc.Label, err)
			continue
		}
		paths = append(paths, path)
		fmt.Printf("wrote %s\n", path)
	}
	if len(paths) == 0 {
		os.Exit(1)
	}
}

func runScenario(sc scenario, absDir, repoRoot, variant, model, outDir string) (string, error) {
	collector := benchagent.Active()
	if collector == nil {
		return "", fmt.Errorf("benchagent not enabled")
	}
	collector.SetMeta(sc.Label, variant, absDir, sc.Prompt)

	projectSize := benchagent.ScanProjectSize(absDir)
	if sc.Label == "large" {
		projectSize = benchagent.ScanProjectSize(repoRoot)
	}

	if err := os.Chdir(absDir); err != nil {
		return "", err
	}
	defer func() {
		_ = os.Chdir(repoRoot)
	}()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	sink := &benchagent.Sink{Inner: event.Discard, C: collector}
	opts := boot.Options{
		Model:         model,
		MaxSteps:      sc.Steps,
		RequireKey:    true,
		Sink:          sink,
		WorkspaceRoot: absDir,
		DeferEagerMCP: true,
		Stderr:        os.Stderr,
	}
	ctrl, err := boot.Build(ctx, opts)
	if err != nil {
		return "", err
	}
	collector.MarkBootDone()
	defer ctrl.Close()

	if err := ctrl.Run(ctx, sc.Prompt); err != nil {
		report := collector.BuildReport(projectSize)
		report.Error = strings.TrimSpace(report.Error + "; " + err.Error())
		return benchagent.WriteReport(report, outDir)
	}
	report := collector.BuildReport(projectSize)
	return benchagent.WriteReport(report, outDir)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

// Ensure benchmark timestamps are monotonic in tests.
var _ = time.Now
