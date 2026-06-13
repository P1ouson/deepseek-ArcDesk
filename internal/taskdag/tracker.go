package taskdag

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

const (
	StatusPending = "pending"
	StatusReady   = "ready"
	StatusRunning = "running"
	StatusDone    = "done"
)

// Node is one task in the DAG.
type Node struct {
	ID     string   `json:"id"`
	Title  string   `json:"title"`
	Deps   []string `json:"deps,omitempty"`
	Status string   `json:"status"`
}

// Status is a snapshot of DAG progress.
type Status struct {
	Active       bool     `json:"active"`
	Total        int      `json:"total"`
	DoneCount    int      `json:"done_count"`
	ReadyIDs     []string `json:"ready_ids,omitempty"`
	RunningIDs   []string `json:"running_ids,omitempty"`
	BlockedCount int      `json:"blocked_count"`
	Summaries    []string `json:"summaries,omitempty"`
}

// Tracker holds live DAG state for tools.
type Tracker struct {
	mu    sync.RWMutex
	nodes map[string]*Node
	order []string
}

// NewTracker returns an empty DAG tracker.
func NewTracker() *Tracker {
	return &Tracker{nodes: make(map[string]*Node)}
}

// Load replaces the graph from JSON nodes.
func (t *Tracker) Load(nodes []Node) {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nodes = make(map[string]*Node, len(nodes))
	t.order = t.order[:0]
	used := map[string]bool{}
	for _, n := range nodes {
		id := normalizeID(n.ID)
		title := strings.TrimSpace(n.Title)
		if id == "" && title != "" {
			id = uniqueID(slugID(title), used)
		}
		if id == "" {
			continue
		}
		id = uniqueID(id, used)
		deps := append([]string(nil), n.Deps...)
		for i := range deps {
			deps[i] = normalizeID(deps[i])
		}
		t.nodes[id] = &Node{ID: id, Title: title, Deps: deps, Status: StatusPending}
		t.order = append(t.order, id)
	}
	t.refreshLocked()
}

// LoadFromPlan parses markdown/JSON plan text into a DAG.
func (t *Tracker) LoadFromPlan(plan string) {
	plan = strings.TrimSpace(plan)
	if plan == "" {
		t.Clear()
		return
	}
	if strings.HasPrefix(plan, "{") || strings.HasPrefix(plan, "[") {
		var payload struct {
			Nodes []Node `json:"nodes"`
		}
		if err := json.Unmarshal([]byte(plan), &payload); err == nil && len(payload.Nodes) > 0 {
			t.Load(payload.Nodes)
			return
		}
		var nodes []Node
		if err := json.Unmarshal([]byte(plan), &nodes); err == nil && len(nodes) > 0 {
			t.Load(nodes)
			return
		}
	}
	t.Load(ParseMarkdown(plan))
}

// Clear drops DAG state.
func (t *Tracker) Clear() {
	if t == nil {
		return
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.nodes = make(map[string]*Node)
	t.order = nil
}

// Status returns a progress snapshot.
func (t *Tracker) Status() Status {
	if t == nil {
		return Status{}
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	st := Status{Total: len(t.order), Active: len(t.order) > 0}
	for _, id := range t.order {
		n := t.nodes[id]
		if n == nil {
			continue
		}
		switch n.Status {
		case StatusDone:
			st.DoneCount++
		case StatusReady:
			st.ReadyIDs = append(st.ReadyIDs, id)
		case StatusRunning:
			st.RunningIDs = append(st.RunningIDs, id)
		case StatusPending:
			st.BlockedCount++
		}
	}
	sort.Strings(st.ReadyIDs)
	sort.Strings(st.RunningIDs)
	return st
}

// Ready returns nodes whose dependencies are done.
func (t *Tracker) Ready() []Node {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	var out []Node
	for _, id := range t.order {
		n := t.nodes[id]
		if n != nil && n.Status == StatusReady {
			out = append(out, *n)
		}
	}
	return out
}

// Start marks a ready node running.
func (t *Tracker) Start(id string) error {
	if t == nil {
		return fmt.Errorf("taskdag not configured")
	}
	id = normalizeID(id)
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.nodes[id]
	if n == nil {
		return fmt.Errorf("unknown task %q", id)
	}
	if n.Status != StatusReady {
		return fmt.Errorf("task %q is %s, not ready", id, n.Status)
	}
	n.Status = StatusRunning
	return nil
}

// Complete marks a running/ready node done and refreshes dependents.
func (t *Tracker) Complete(id, summary string) (string, error) {
	if t == nil {
		return "", fmt.Errorf("taskdag not configured")
	}
	id = normalizeID(id)
	t.mu.Lock()
	defer t.mu.Unlock()
	n := t.nodes[id]
	if n == nil {
		return "", fmt.Errorf("unknown task %q", id)
	}
	if n.Status != StatusReady && n.Status != StatusRunning {
		return "", fmt.Errorf("task %q is %s, cannot complete", id, n.Status)
	}
	n.Status = StatusDone
	t.refreshLocked()
	ready := make([]string, 0)
	for _, nid := range t.order {
		if nn := t.nodes[nid]; nn != nil && nn.Status == StatusReady {
			ready = append(ready, nid)
		}
	}
	msg := fmt.Sprintf("Completed %q (%s).", id, n.Title)
	if summary = strings.TrimSpace(summary); summary != "" {
		msg += " " + summary
	}
	if len(ready) > 0 {
		msg += fmt.Sprintf(" Ready now: %s.", strings.Join(ready, ", "))
	} else if t.allDoneLocked() {
		msg += " All tasks complete."
	}
	return msg, nil
}

func (t *Tracker) allDoneLocked() bool {
	for _, id := range t.order {
		if n := t.nodes[id]; n == nil || n.Status != StatusDone {
			return false
		}
	}
	return len(t.order) > 0
}

// ValidateIssues reports missing dependencies and cycles after Load.
func (t *Tracker) ValidateIssues() []string {
	if t == nil {
		return nil
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	var issues []string
	for _, id := range t.order {
		n := t.nodes[id]
		if n == nil {
			continue
		}
		for _, dep := range n.Deps {
			if t.nodes[dep] == nil {
				issues = append(issues, fmt.Sprintf("task %q depends on unknown %q", id, dep))
			}
		}
	}
	for _, cyc := range detectCycles(t.nodes, t.order) {
		issues = append(issues, "cycle: "+strings.Join(cyc, " -> "))
	}
	return issues
}

func detectCycles(nodes map[string]*Node, order []string) [][]string {
	state := map[string]int{}
	var cycles [][]string
	var stack []string
	var walk func(id string)
	walk = func(id string) {
		switch state[id] {
		case 1:
			for i, s := range stack {
				if s == id {
					cycles = append(cycles, append(append([]string(nil), stack[i:]...), id))
					break
				}
			}
			return
		case 2:
			return
		}
		state[id] = 1
		stack = append(stack, id)
		if n := nodes[id]; n != nil {
			for _, dep := range n.Deps {
				walk(dep)
			}
		}
		stack = stack[:len(stack)-1]
		state[id] = 2
	}
	for _, id := range order {
		if state[id] == 0 {
			walk(id)
		}
	}
	return cycles
}

func (t *Tracker) refreshLocked() {
	for _, id := range t.order {
		n := t.nodes[id]
		if n == nil || n.Status == StatusDone || n.Status == StatusRunning {
			continue
		}
		if depsDone(t.nodes, n.Deps) {
			n.Status = StatusReady
		} else {
			n.Status = StatusPending
		}
	}
}

func depsDone(nodes map[string]*Node, deps []string) bool {
	for _, d := range deps {
		n := nodes[d]
		if n == nil || n.Status != StatusDone {
			return false
		}
	}
	return true
}

var depSuffix = regexp.MustCompile(`(?i)\(deps?:\s*([^)]+)\)`)

func normalizeID(id string) string {
	id = strings.ToLower(strings.TrimSpace(id))
	id = strings.ReplaceAll(id, " ", "-")
	return id
}

// ParseMarkdown turns list items into nodes. Format:
// - task-id: Title (deps: a, b)
func ParseMarkdown(plan string) []Node {
	var nodes []Node
	used := map[string]bool{}
	for _, raw := range strings.Split(plan, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		for strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			line = strings.TrimSpace(line[2:])
		}
		if i := strings.IndexByte(line, '.'); i > 0 && i < 4 && line[i+1] == ' ' {
			line = strings.TrimSpace(line[i+2:])
		}
		if line == "" {
			continue
		}
		id, title, deps := splitLine(line)
		if title == "" {
			continue
		}
		if id == "" {
			id = uniqueID(slugID(title), used)
		} else {
			id = uniqueID(normalizeID(id), used)
		}
		nodes = append(nodes, Node{ID: id, Title: title, Deps: deps})
	}
	return nodes
}

func splitLine(line string) (id, title string, deps []string) {
	if m := depSuffix.FindStringSubmatch(line); len(m) == 2 {
		for _, d := range strings.Split(m[1], ",") {
			if d = normalizeID(d); d != "" {
				deps = append(deps, d)
			}
		}
		line = depSuffix.ReplaceAllString(line, "")
	}
	line = strings.TrimSpace(line)
	if i := strings.Index(line, ":"); i > 0 {
		id = normalizeID(line[:i])
		title = strings.TrimSpace(line[i+1:])
		return id, title, deps
	}
	return "", strings.TrimSpace(line), deps
}

func slugID(title string) string {
	title = strings.ToLower(strings.TrimSpace(title))
	if title == "" {
		return "task"
	}
	var b strings.Builder
	prevDash := false
	for _, r := range title {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			prevDash = false
			continue
		}
		if !prevDash {
			b.WriteByte('-')
			prevDash = true
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "task"
	}
	return out
}

func uniqueID(base string, used map[string]bool) string {
	base = normalizeID(base)
	if base == "" {
		base = "task"
	}
	id := base
	for n := 2; used[id]; n++ {
		id = fmt.Sprintf("%s-%d", base, n)
	}
	used[id] = true
	return id
}
