package benchagent

import (
	"encoding/json"
	"strings"
	"sync"
	"time"

	"arcdesk/internal/event"
)

type readKey struct {
	path   string
	offset int
	limit  int
}

type readFileState struct {
	path   string
	offset int
}

// Collector records real agent events into a benchmark report.
type Collector struct {
	mu sync.Mutex

	label       string
	variant     string
	projectRoot string
	prompt      string
	started     time.Time
	bootDoneMs  int64

	firstToolMs           int64
	firstReadMs           int64
	firstReasoningMs      int64
	firstAssistantTokenMs int64
	firstActionMs         int64
	completedMs           int64
	runErr              string

	totalToolCalls int
	readCalls      int
	truncatedReads int
	readLineSum    int
	readLineCount  int

	readKeys      map[readKey]int
	lastRead      readFileState
	pagingDepth   int
	maxPagingDepth int
	pagingChains  int

	stepHitRates []float64
	prefixChanges int

	fanoutSamples []int
	maxConcurrency int
	throttledRounds int

	promptTokens     int
	completionTokens int
	agentTurns       int
	estimatedCost    float64
}

// NewCollector starts a fresh benchmark session.
func NewCollector() *Collector {
	return &Collector{
		started:               time.Now(),
		readKeys:              make(map[readKey]int),
		firstToolMs:           -1,
		firstReadMs:           -1,
		firstReasoningMs:      -1,
		firstAssistantTokenMs: -1,
		firstActionMs:         -1,
		completedMs:           -1,
		bootDoneMs:            -1,
	}
}

// SetMeta attaches run metadata before the agent starts.
func (c *Collector) SetMeta(label, variant, projectRoot, prompt string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.label = label
	c.variant = variant
	c.projectRoot = projectRoot
	c.prompt = prompt
}

// MarkBootDone records project/controller boot completion.
func (c *Collector) MarkBootDone() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.bootDoneMs = time.Since(c.started).Milliseconds()
}

// RecordParallelBatch records fanout concurrency for a parallel read batch.
func RecordParallelBatch(concurrency, batchSize int) {
	c := Active()
	if c == nil || batchSize <= 1 {
		return
	}
	c.recordParallelBatch(concurrency, batchSize)
}

func (c *Collector) recordParallelBatch(concurrency, batchSize int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.fanoutSamples = append(c.fanoutSamples, concurrency)
	if concurrency > c.maxConcurrency {
		c.maxConcurrency = concurrency
	}
	// Throttled when batch wanted parallelism but concurrency was reduced below 4
	// while batch size suggests the model requested a wide fanout.
	if batchSize >= 3 && concurrency <= 2 {
		c.throttledRounds++
	} else if batchSize >= 4 && concurrency <= 3 {
		c.throttledRounds++
	}
}

// Sink wraps inner and records benchmark events.
type Sink struct {
	Inner event.Sink
	C     *Collector
}

func (s *Sink) Emit(e event.Event) {
	if s.C != nil {
		s.C.observe(e)
	}
	if s.Inner != nil {
		s.Inner.Emit(e)
	}
}

func (c *Collector) observe(e event.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.started).Milliseconds()
	mark := func(slot *int64) {
		if *slot < 0 {
			*slot = elapsed
		}
	}

	switch e.Kind {
	case event.TurnStarted:
		// reset per-turn markers are cumulative across one Run call
	case event.Reasoning:
		mark(&c.firstReasoningMs)
	case event.Text:
		mark(&c.firstAssistantTokenMs)
	case event.ToolDispatch:
		if e.Tool.Partial {
			return
		}
		c.totalToolCalls++
		mark(&c.firstToolMs)
		if e.Tool.Name == "read_file" {
			c.readCalls++
			mark(&c.firstReadMs)
			c.trackReadDispatch(e.Tool.Args)
		}
		if isActionTool(e.Tool.Name) && !e.Tool.ReadOnly {
			mark(&c.firstActionMs)
		}
	case event.ToolResult:
		if e.Tool.Name == "read_file" {
			if e.Tool.Truncated {
				c.truncatedReads++
			}
			c.trackReadResult(e.Tool.Args, e.Tool.Output)
		}
	case event.Usage:
		if e.Usage == nil {
			return
		}
		c.agentTurns++
		u := e.Usage
		c.promptTokens += u.PromptTokens
		c.completionTokens += u.CompletionTokens
		hit := u.CacheHitTokens
		miss := u.CacheMissTokens
		if hit+miss > 0 {
			c.stepHitRates = append(c.stepHitRates, float64(hit)/float64(hit+miss))
		}
		if e.CacheDiagnostics != nil && e.CacheDiagnostics.PrefixChanged {
			c.prefixChanges++
		}
		if e.Pricing != nil {
			p := e.Pricing
			c.estimatedCost += (float64(hit)*p.CacheHit +
				float64(miss)*p.Input +
				float64(u.CompletionTokens)*p.Output) / 1e6
		}
	case event.TurnDone:
		mark(&c.completedMs)
		if e.Err != nil {
			c.runErr = e.Err.Error()
		}
	}
}

func (c *Collector) trackReadDispatch(argsJSON string) {
	path, offset, limit := parseReadArgs(argsJSON)
	key := readKey{path: path, offset: offset, limit: limit}
	c.readKeys[key]++
	if c.readKeys[key] > 1 {
		// counted later in report as duplicateReads
	}
	if path != "" && c.lastRead.path == path && offset > c.lastRead.offset {
		c.pagingDepth++
		if c.pagingDepth > c.maxPagingDepth {
			c.maxPagingDepth = c.pagingDepth
		}
	} else if path != "" && c.lastRead.path == path && offset == 0 && c.lastRead.offset > 0 {
		c.pagingChains++
		c.pagingDepth = 1
	} else {
		if c.pagingDepth > 1 {
			c.pagingChains++
		}
		c.pagingDepth = 1
	}
	c.lastRead = readFileState{path: path, offset: offset}
	if limit > 0 {
		c.readLineSum += limit
		c.readLineCount++
	}
}

func (c *Collector) trackReadResult(argsJSON, output string) {
	_, _, limit := parseReadArgs(argsJSON)
	if limit <= 0 {
		lines := strings.Count(output, "\n")
		if lines > 0 && !strings.HasSuffix(output, "\n") {
			lines++
		}
		if lines > 0 {
			c.readLineSum += lines
			c.readLineCount++
		}
	}
}

func parseReadArgs(argsJSON string) (path string, offset, limit int) {
	var args struct {
		Path   string `json:"path"`
		Offset *int   `json:"offset"`
		Limit  *int   `json:"limit"`
	}
	if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
		return "", 0, 0
	}
	path = args.Path
	if args.Offset != nil {
		offset = *args.Offset
	}
	if args.Limit != nil {
		limit = *args.Limit
	}
	return path, offset, limit
}

func isActionTool(name string) bool {
	switch name {
	case "edit_file", "write_file", "multi_edit", "notebook_edit",
		"delete_symbol", "delete_range", "bash":
		return true
	default:
		return false
	}
}

// BuildReport materializes the JSON report from collected events.
func (c *Collector) BuildReport(projectSize ProjectSize) Report {
	c.mu.Lock()
	defer c.mu.Unlock()

	repeated := 0
	dup := 0
	for _, n := range c.readKeys {
		if n > 1 {
			dup += n - 1
		}
		if n > 1 {
			repeated++
		}
	}

	avgLines := 0.0
	if c.readLineCount > 0 {
		avgLines = float64(c.readLineSum) / float64(c.readLineCount)
	}

	avgHit, lowHit, highHit := 0.0, 0.0, 0.0
	if len(c.stepHitRates) > 0 {
		sum := 0.0
		lowHit = c.stepHitRates[0]
		highHit = c.stepHitRates[0]
		for _, r := range c.stepHitRates {
			sum += r
			if r < lowHit {
				lowHit = r
			}
			if r > highHit {
				highHit = r
			}
		}
		avgHit = sum / float64(len(c.stepHitRates))
	}

	avgConc := 0.0
	if len(c.fanoutSamples) > 0 {
		sum := 0
		for _, v := range c.fanoutSamples {
			sum += v
		}
		avgConc = float64(sum) / float64(len(c.fanoutSamples))
	}

	completed := c.completedMs
	if completed < 0 {
		completed = time.Since(c.started).Milliseconds()
	}

	toMs := func(v int64) int64 {
		if v < 0 {
			return 0
		}
		return v
	}

	return Report{
		Label:       c.label,
		Variant:     c.variant,
		ProjectRoot: c.projectRoot,
		Prompt:      c.prompt,
		StartedAt:   c.started,
		FinishedAt:  c.started.Add(time.Duration(completed) * time.Millisecond),
		ProjectSize: projectSize,
		Timings: Timings{
			ProjectOpenMs:         toMs(c.bootDoneMs),
			FirstToolMs:           toMs(c.firstToolMs),
			FirstReadMs:           toMs(c.firstReadMs),
			FirstReasoningMs:      toMs(c.firstReasoningMs),
			FirstAssistantTokenMs: toMs(c.firstAssistantTokenMs),
			FirstActionMs:         toMs(c.firstActionMs),
			TaskCompletedMs:       toMs(completed),
		},
		ToolUsage: ToolUsage{
			TotalToolCalls:        c.totalToolCalls,
			ReadFileCalls:         c.readCalls,
			RepeatedReadFileCalls: repeated,
			AvgReadFileLines:      avgLines,
			TruncatedReads:        c.truncatedReads,
		},
		ReadPatterns: ReadPatterns{
			OffsetPagingChains: c.pagingChains,
			MaxPagingDepth:     c.maxPagingDepth,
			DuplicateReads:     dup,
		},
		Cache: CacheStats{
			AvgHitRate:         avgHit,
			LowestStepHitRate:  lowHit,
			HighestStepHitRate: highHit,
			PrefixChangedCount: c.prefixChanges,
		},
		Fanout: FanoutStats{
			AvgConcurrency:  avgConc,
			MaxConcurrency:  c.maxConcurrency,
			ThrottledRounds: c.throttledRounds,
		},
		API: APIStats{
			TotalAgentTurns:       c.agentTurns,
			TotalPromptTokens:     c.promptTokens,
			TotalCompletionTokens: c.completionTokens,
			EstimatedCost:         c.estimatedCost,
		},
		Error: c.runErr,
	}
}
