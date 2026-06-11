// benchagenthook injects dev-only benchmark hooks into a checked-out agent.go
// without changing runtime behavior when BENCHMARK_AGENT is unset.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

func main() {
	agentPath := flag.String("agent", "internal/agent/agent.go", "path to agent.go")
	flag.Parse()

	data, err := os.ReadFile(*agentPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	src := string(data)
	if strings.Contains(src, "arcdesk/internal/benchagent") {
		os.Exit(0)
	}
	src = strings.Replace(src,
		`"arcdesk/internal/diff"`,
		`"arcdesk/internal/benchagent"`+"\n\t"+`"arcdesk/internal/diff"`,
		1,
	)
	src = strings.Replace(src,
		"runParallel(batch.start, batch.end, run)",
		"par := a.exploreParallelism()\n\t\t\tbenchagent.RecordParallelBatch(par, batch.end-batch.start)\n\t\t\trunParallel(batch.start, batch.end, par, run)",
		1,
	)
	src = strings.Replace(src,
		"func runParallel(start, end int, run func(int)) {",
		"func (a *Agent) exploreParallelism() int { return 8 }\n\nfunc runParallel(start, end int, concurrency int, run func(int)) {",
		1,
	)
	src = strings.Replace(src,
		"const maxParallel = 8",
		"if concurrency < 1 { concurrency = 1 }",
		1,
	)
	src = strings.Replace(src,
		"sem := make(chan struct{}, maxParallel)",
		"sem := make(chan struct{}, concurrency)",
		1,
	)
	if err := os.WriteFile(*agentPath, []byte(src), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
