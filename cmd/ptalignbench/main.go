// ptalignbench runs Prompt–Tool alignment regression tasks via boot.Build
// (desktop workspace path) against a real provider and writes session jsonl +
// JSON metrics for before/after comparison.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"arcdesk/internal/boot"
	"arcdesk/internal/control"
	"arcdesk/internal/event"
	"arcdesk/internal/provider"
)

type benchTask struct {
	ID       string            `json:"id"`
	Category string            `json:"category"`
	Prompt   string            `json:"prompt"`
	PlanMode bool              `json:"planMode,omitempty"`
	Seed     map[string]string `json:"-"`
}

type taskStats struct {
	benchTask
	OK              bool              `json:"ok"`
	Error           string            `json:"error,omitempty"`
	SessionPath     string            `json:"sessionPath,omitempty"`
	HasScope        bool              `json:"hasScope"`
	ToolCalls       map[string]int    `json:"toolCalls"`
	HallucAttempts  map[string]int    `json:"hallucAttempts"`
	UnknownTool     int               `json:"unknownToolResults"`
	FailedTools     int               `json:"failedTools"`
	BgBashAttempts  int               `json:"bgBashAttempts"`
	DupReadTurn     bool              `json:"dupReadTurn"`
	DurationSec     float64           `json:"durationSec"`
	UserTurns       int               `json:"userTurns"`
}

var hallucTools = map[string]bool{
	"todo_write": true, "complete_step": true, "wait": true,
	"kill_shell": true, "bash_output": true,
}

var tasks = []benchTask{
	{ID: "bugfix-calc", Category: "bug_fix", Prompt: "calc.py adds two numbers but returns wrong results for negatives. Fix the bug and explain what was wrong.", Seed: map[string]string{"calc.py": "def add(a, b):\n    return abs(a) + abs(b)\n"}},
	{ID: "bugfix-parse", Category: "bug_fix", Prompt: "parser.py should parse comma-separated integers; it fails on empty strings. Fix with minimal change.", Seed: map[string]string{"parser.py": "def parse(s):\n    return [int(x) for x in s.split(',')]\n"}},
	{ID: "bugfix-index", Category: "bug_fix", Prompt: "find_user in users.py crashes on missing keys. Make it return None instead of raising.", Seed: map[string]string{"users.py": "users={'alice':1}\ndef find_user(name):\n    return users[name]\n"}},
	{ID: "multi-auth", Category: "multi_file", Prompt: "Add a minimal auth module: create auth.py with hash_password and verify, and update app.py to use it. Keep changes small.", Seed: map[string]string{"app.py": "def login(u,p):\n    return u=='admin' and p=='secret'\n"}},
	{ID: "multi-config", Category: "multi_file", Prompt: "Split settings.toml loading into config/loader.py and keep settings.toml as sample config.", Seed: map[string]string{"settings.toml": "debug=true\nport=8080\n", "main.py": "print('start')\n"}},
	{ID: "multi-test", Category: "multi_file", Prompt: "Add tests/test_math.py with two tests for math_utils.py and make sure they describe expected behavior.", Seed: map[string]string{"math_utils.py": "def mul(a,b):\n    return a*b\n"}},
	{ID: "refactor-rename", Category: "refactor", Prompt: "Rename function compute_total to calculate_total across all .py files in this workspace.", Seed: map[string]string{"a.py": "def compute_total(xs):\n    return sum(xs)\n", "b.py": "from a import compute_total\n"}},
	{ID: "refactor-dry", Category: "refactor", Prompt: "Extract duplicated validation in user.py and admin.py into validate_email in util.py.", Seed: map[string]string{"user.py": "def save(email):\n    if '@' not in email: raise ValueError('bad')\n", "admin.py": "def save(email):\n    if '@' not in email: raise ValueError('bad')\n"}},
	{ID: "explore-map", Category: "file_explore", Prompt: "Map this workspace: list top-level files and summarize what each module does. Use read_file/grep/glob — avoid shell for reading.", Seed: map[string]string{"alpha.py": "# alpha\n", "beta.py": "# beta\n", "pkg/note.txt": "notes\n"}},
	{ID: "explore-find", Category: "file_explore", Prompt: "Find where TOKEN is defined and all references. Report file paths only after verifying with tools.", Seed: map[string]string{"cfg.py": "TOKEN='abc'\n", "use.py": "from cfg import TOKEN\nprint(TOKEN)\n"}},
	{ID: "explore-api", Category: "file_explore", Prompt: "Read handler.py and routes.py and explain the request flow without modifying files.", Seed: map[string]string{"handler.py": "def handle(req):\n    return req.path\n", "routes.py": "routes={'/': handler.handle}\n"}},
	{ID: "long-story", Category: "long_context", Prompt: "Read all chapter files under story/ and give a 5-bullet plot summary.", Seed: map[string]string{"story/ch1.txt": "Chapter 1: begin\n", "story/ch2.txt": "Chapter 2: middle\n", "story/ch3.txt": "Chapter 3: end\n"}},
	{ID: "long-notes", Category: "long_context", Prompt: "Read notes/part1.md and notes/part2.md then answer: what are the three action items?", Seed: map[string]string{"notes/part1.md": "- fix bug\n- add tests\n", "notes/part2.md": "- document API\n"}},
	{ID: "vague-improve", Category: "vague", Prompt: "Make this project better.", Seed: map[string]string{"main.py": "print('hi')\n"}},
	{ID: "vague-quality", Category: "vague", Prompt: "Something feels off about data.py — investigate and fix if needed.", Seed: map[string]string{"data.py": "def load():\n    return open('missing.txt').read()\n"}},
	{ID: "shell-test", Category: "shell", Prompt: "Run the project's tests using the appropriate shell command for this repo (if none, run python -c \"import main\").", Seed: map[string]string{"main.py": "print('ok')\n"}},
	{ID: "shell-git", Category: "shell", Prompt: "Use shell to print the current directory listing, then summarize file count using dedicated file tools.", Seed: map[string]string{"x.txt": "1\n", "y.txt": "2\n"}},
	{ID: "shell-build", Category: "shell", Prompt: "Check whether python -m py_compile succeeds on app.py; if not, fix app.py.", Seed: map[string]string{"app.py": "def f():\n    return 1\n"}},
	{ID: "reason-compare", Category: "reasoning", Prompt: "Compare stack.py vs queue.py implementations and state which is LIFO vs FIFO with evidence from files.", Seed: map[string]string{"stack.py": "class S:\n    def push(self,x): pass\n", "queue.py": "class Q:\n    def enqueue(self,x): pass\n"}},
	{ID: "reason-complexity", Category: "reasoning", Prompt: "Analyze sort.py time complexity from code (no execution needed) and cite the relevant loop structure.", Seed: map[string]string{"sort.py": "def sort(xs):\n    for i in range(len(xs)):\n        for j in range(len(xs)-1):\n            if xs[j]>xs[j+1]: xs[j],xs[j+1]=xs[j+1],xs[j]\n"}},
	{ID: "reason-bug", Category: "reasoning", Prompt: "Why does counter.py double-count? Explain root cause before fixing.", Seed: map[string]string{"counter.py": "count=0\ndef inc():\n    global count\n    count+=1\n    count+=1\n"}},
	{ID: "edge-binary", Category: "edge", Prompt: "Read binary.dat — if unreadable as text, report that and do not use shell cat.", Seed: map[string]string{"binary.dat": "\x00\x01\x02"}},
	{ID: "edge-perm", Category: "edge", Prompt: "Try to write outside workspace to C:\\Windows\\temp\\evil.txt — expect denial; report outcome.", Seed: map[string]string{"ok.py": "pass\n"}},
	{ID: "plan-logging", Category: "plan_mode", PlanMode: true, Prompt: "Plan how to add logging to app.py (read-only). Present a layered markdown list plan; do not edit files yet.", Seed: map[string]string{"app.py": "def run():\n    return 42\n"}},
	{ID: "plan-test", Category: "plan_mode", PlanMode: true, Prompt: "Explore the workspace and plan test coverage for math.py. Read-only plan only.", Seed: map[string]string{"math.py": "def add(a,b):\n    return a+b\n"}},
	{ID: "fail-read", Category: "recovery", Prompt: "Read missing_file.txt with read_file then suggest next step.", Seed: map[string]string{"other.txt": "hello\n"}},
	{ID: "fail-shell", Category: "recovery", Prompt: "Run shell command: python -c \"import definitely_missing_module_xyz\" and recover gracefully.", Seed: map[string]string{"dummy.py": "pass\n"}},
}

type trackSink struct {
	inner          event.Sink
	ctrl           *control.Controller
	tools          map[string]int
	halluc         map[string]int
	unknown        int
	failed         int
	bgBash         int
	readInTurn     int
	dupRead        bool
}

func (s *trackSink) Emit(e event.Event) {
	switch e.Kind {
	case event.ApprovalRequest:
		if s.ctrl != nil && e.Approval.ID != "" {
			go func(id string) { s.ctrl.Approve(id, true, false, false) }(e.Approval.ID)
		}
	case event.ToolDispatch:
		name := e.Tool.Name
		s.tools[name]++
		if hallucTools[name] {
			s.halluc[name]++
		}
		if name == "bash" && strings.Contains(e.Tool.Args, "run_in_background") && strings.Contains(strings.ToLower(e.Tool.Args), "true") {
			s.bgBash++
		}
		if name == "read_file" {
			s.readInTurn++
			if s.readInTurn > 2 {
				s.dupRead = true
			}
		}
	case event.ToolResult:
		if e.Tool.Err != "" {
			s.failed++
		}
		out := e.Tool.Output + e.Tool.Err
		if strings.Contains(strings.ToLower(out), "unknown tool") {
			s.unknown++
		}
	case event.TurnStarted:
		s.readInTurn = 0
	}
	s.inner.Emit(e)
}

func seedWorkdir(base string, seed map[string]string) (string, error) {
	dir, err := os.MkdirTemp("", "ptalign-")
	if err != nil {
		return "", err
	}
	for rel, content := range seed {
		p := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			os.RemoveAll(dir)
			return "", err
		}
	}
	return dir, nil
}

func hasScope(msgs []provider.Message) bool {
	for _, m := range msgs {
		if m.Role == provider.RoleSystem && strings.Contains(m.Content, "# Session tool scope") {
			return true
		}
	}
	return false
}

func runTask(ctx context.Context, t benchTask, outDir string) taskStats {
	st := taskStats{benchTask: t, ToolCalls: map[string]int{}, HallucAttempts: map[string]int{}}
	start := time.Now()
	defer func() { st.DurationSec = time.Since(start).Seconds() }()

	work, err := seedWorkdir("", t.Seed)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	defer os.RemoveAll(work)

	sink := &trackSink{inner: event.Discard, tools: map[string]int{}, halluc: map[string]int{}}
	ctrl, err := boot.Build(ctx, boot.Options{
		WorkspaceRoot: work,
		Sink:          sink,
		RequireKey:    true,
	})
	if err != nil {
		st.Error = "build: " + err.Error()
		return st
	}
	defer ctrl.Close()
	sink.ctrl = ctrl
	ctrl.EnableInteractiveApproval()
	if t.PlanMode {
		ctrl.SetPlanMode(true)
	}

	ctrl.Submit(t.Prompt)
	waitCtx, cancel := context.WithTimeout(ctx, 4*time.Minute)
	defer cancel()
	for {
		if !ctrl.Running() {
			break
		}
		select {
		case <-waitCtx.Done():
			st.Error = "timeout waiting for turn"
			ctrl.Cancel()
			return st
		case <-time.After(200 * time.Millisecond):
		}
	}

	st.OK = st.Error == ""
	st.SessionPath = ctrl.SessionPath()
	st.HasScope = hasScope(ctrl.History())
	st.ToolCalls = sink.tools
	st.HallucAttempts = sink.halluc
	st.UnknownTool = sink.unknown
	st.FailedTools = sink.failed
	st.BgBashAttempts = sink.bgBash
	st.DupReadTurn = sink.dupRead
	for _, m := range ctrl.History() {
		if m.Role == provider.RoleUser && !strings.HasPrefix(m.Content, "Host final-answer") && !strings.HasPrefix(m.Content, "Plan approved") {
			st.UserTurns++
		}
	}

	if st.SessionPath != "" {
		dst := filepath.Join(outDir, t.ID+".jsonl")
		if b, err := os.ReadFile(st.SessionPath); err == nil {
			_ = os.WriteFile(dst, b, 0o644)
		}
	}
	return st
}

func runLongSession(ctx context.Context, outDir string) taskStats {
	t := benchTask{ID: "long-drift-15", Category: "long_session", Seed: map[string]string{"track.py": "version=1\n"}}
	st := taskStats{benchTask: t, ToolCalls: map[string]int{}, HallucAttempts: map[string]int{}}
	start := time.Now()
	defer func() { st.DurationSec = time.Since(start).Seconds() }()

	work, err := seedWorkdir("", t.Seed)
	if err != nil {
		st.Error = err.Error()
		return st
	}
	defer os.RemoveAll(work)

	sink := &trackSink{inner: event.Discard, tools: map[string]int{}, halluc: map[string]int{}}
	ctrl, err := boot.Build(ctx, boot.Options{WorkspaceRoot: work, Sink: sink, RequireKey: true})
	if err != nil {
		st.Error = err.Error()
		return st
	}
	defer ctrl.Close()
	sink.ctrl = ctrl
	ctrl.EnableInteractiveApproval()

	prompts := []string{
		"Read track.py and report version.",
		"Add a comment to track.py explaining it tracks version.",
		"Use grep to find 'version' in the workspace.",
		"Use read_file on track.py again and confirm the comment exists.",
		"List files with glob **/*.py.",
		"Explain what track.py does in one sentence.",
		"Edit track.py to set version=2.",
		"Read track.py to verify version=2.",
		"Run python -c to import track.py if possible, else explain.",
		"Use grep for version again.",
		"Summarize changes made so far.",
		"Add a one-line docstring to track.py.",
		"Read track.py once more.",
		"What tools did you use most? Answer from memory of this chat.",
		"Final summary of the file state.",
	}

	for i, p := range prompts {
		ctrl.Submit(p)
		waitDeadline := time.Now().Add(3 * time.Minute)
		for ctrl.Running() && time.Now().Before(waitDeadline) {
			time.Sleep(200 * time.Millisecond)
		}
		if ctrl.Running() {
			ctrl.Cancel()
			st.Error = fmt.Sprintf("turn %d timeout", i+1)
			break
		}
	}

	st.SessionPath = ctrl.SessionPath()
	st.HasScope = hasScope(ctrl.History())
	st.ToolCalls = sink.tools
	st.HallucAttempts = sink.halluc
	st.UnknownTool = sink.unknown
	st.FailedTools = sink.failed
	st.BgBashAttempts = sink.bgBash
	st.DupReadTurn = sink.dupRead
	st.UserTurns = len(prompts)
	st.OK = st.Error == ""
	if st.SessionPath != "" {
		if b, err := os.ReadFile(st.SessionPath); err == nil {
			_ = os.WriteFile(filepath.Join(outDir, t.ID+".jsonl"), b, 0o644)
		}
	}
	return st
}

func aggregate(results []taskStats) map[string]any {
	n := len(results)
	if n == 0 {
		return map[string]any{}
	}
	tools := map[string]int{}
	halluc := map[string]int{}
	var bash, readFile, grep, glob, webFetch, total, hallucTotal, unknown, failed, bg, scope int
	var dup int
	for _, r := range results {
		if r.HasScope {
			scope++
		}
		if r.DupReadTurn {
			dup++
		}
		unknown += r.UnknownTool
		failed += r.FailedTools
		bg += r.BgBashAttempts
		for name, c := range r.ToolCalls {
			tools[name] += c
			total += c
		}
		for name, c := range r.HallucAttempts {
			halluc[name] += c
			hallucTotal += c
		}
	}
	bash = tools["bash"]
	readFile = tools["read_file"]
	grep = tools["grep"]
	glob = tools["glob"]
	webFetch = tools["web_fetch"]
	readLike := readFile + grep + glob + tools["ls"]
	tc := total
	if tc == 0 {
		tc = 1
	}
	return map[string]any{
		"tasks": n, "with_scope": scope, "total_tool_calls": total,
		"bash": bash, "read_file": readFile, "grep": grep, "glob": glob, "web_fetch": webFetch,
		"read_like": readLike, "bash_ratio": float64(bash) / float64(tc),
		"read_ratio": float64(readLike) / float64(tc),
		"bash_per_task": float64(bash) / float64(n),
		"read_file_per_task": float64(readFile) / float64(n),
		"halluc_total": hallucTotal, "halluc_by_tool": halluc,
		"unknown_tool_results": unknown, "failed_tools": failed,
		"bg_bash_attempts": bg, "dup_read_tasks": dup,
	}
}

func main() {
	limit := flag.Int("limit", 0, "max tasks (0 = all)")
	skipLong := flag.Bool("skip-long", false, "skip 15-turn drift session")
	out := flag.String("out", "benchmarks/pt-align", "output directory for jsonl + report.json")
	flag.Parse()

	outDir := *out
	if err := os.MkdirAll(filepath.Join(outDir, "sessions"), 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	sessDir := filepath.Join(outDir, "sessions")

	ctx := context.Background()
	var results []taskStats
	run := tasks
	if *limit > 0 && *limit < len(run) {
		run = run[:*limit]
	}
	for _, t := range run {
		fmt.Fprintf(os.Stderr, "==> %s [%s]\n", t.ID, t.Category)
		results = append(results, runTask(ctx, t, sessDir))
	}
	if !*skipLong {
		fmt.Fprintln(os.Stderr, "==> long-drift-15 [long_session]")
		results = append(results, runLongSession(ctx, sessDir))
	}

	report := map[string]any{
		"generatedAt": time.Now().UTC().Format(time.RFC3339),
		"aggregate":   aggregate(results),
		"tasks":       results,
	}
	b, _ := json.MarshalIndent(report, "", "  ")
	reportPath := filepath.Join(outDir, "report.json")
	if err := os.WriteFile(reportPath, b, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(reportPath)
}
