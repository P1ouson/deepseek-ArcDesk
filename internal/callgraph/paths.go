package callgraph

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
)

// TraceForward traces from a node ID forward toward Go bind methods.
func TraceForward(g *CallGraph, from NodeID, opts TraceOptions) []CallPath {
	if g == nil || from == "" {
		return nil
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = DefaultTraceOptions().MaxDepth
	}
	if opts.MaxPaths <= 0 {
		opts.MaxPaths = DefaultTraceOptions().MaxPaths
	}
	paths := bfsPaths(g, from, true, opts)
	if opts.IncludeGoInternal && opts.SymbolQuery != nil && opts.SymbolQuery.Available() {
		paths = extendForwardWithSymbols(g, paths, opts)
	}
	return paths
}

// TraceBackward traces from a go_bind node backward toward UI nodes.
func TraceBackward(g *CallGraph, to NodeID, opts TraceOptions) []CallPath {
	if g == nil || to == "" {
		return nil
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = DefaultTraceOptions().MaxDepth
	}
	if opts.MaxPaths <= 0 {
		opts.MaxPaths = DefaultTraceOptions().MaxPaths
	}
	return bfsPaths(g, to, false, opts)
}

// FindBridgePath finds shortest paths between a bridge/go bind and UI using
// bidirectional search when direct forward paths miss the go bind target.
func FindBridgePath(g *CallGraph, frontend NodeID, goMethod string) ([]CallPath, error) {
	if g == nil {
		return nil, ErrIndexNotReady
	}
	gobind, ok := g.MethodMap[goMethod]
	if !ok {
		gobind, ok = g.MethodMap["App."+goMethod]
	}
	if !ok {
		return nil, fmt.Errorf("go bind method %q not found", goMethod)
	}
	opts := DefaultTraceOptions()
	opts.MaxPaths = 1
	forward := TraceForward(g, frontend, opts)
	var out []CallPath
	for _, p := range forward {
		if len(p.Segments) == 0 {
			continue
		}
		last := p.Segments[len(p.Segments)-1].Node.ID
		if last == gobind {
			out = append(out, p)
		}
	}
	if len(out) > 0 {
		return out, nil
	}
	if bridged := bidirectionalBridge(g, frontend, gobind, opts); len(bridged) > 0 {
		return bridged, nil
	}
	backward := TraceBackward(g, gobind, opts)
	for _, p := range backward {
		if len(p.Segments) == 0 {
			continue
		}
		first := p.Segments[0].Node.ID
		if first == frontend {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no path between %s and %s", frontend, gobind)
	}
	return out, nil
}

func bidirectionalBridge(g *CallGraph, frontend, gobind NodeID, opts TraceOptions) []CallPath {
	if g == nil || frontend == "" || gobind == "" {
		return nil
	}
	fwdDepth := map[NodeID][]pathStep{frontend: {{node: frontend}}}
	fwdQueue := []NodeID{frontend}
	bwdDepth := map[NodeID][]pathStep{gobind: {{node: gobind}}}
	bwdQueue := []NodeID{gobind}
	maxDepth := opts.MaxDepth
	if maxDepth <= 0 {
		maxDepth = DefaultTraceOptions().MaxDepth
	}

	for depth := 0; depth < maxDepth && (len(fwdQueue) > 0 || len(bwdQueue) > 0); depth++ {
		var nextFwd []NodeID
		for _, id := range fwdQueue {
			for _, step := range traceNeighbors(g, id, true, opts) {
				if _, seen := fwdDepth[step.node]; seen {
					continue
				}
				path := append(slicesCloneSteps(fwdDepth[id]), step)
				fwdDepth[step.node] = path
				if bwdPath, ok := bwdDepth[step.node]; ok {
					return []CallPath{mergeBridgePaths(g, path, bwdPath)}
				}
				nextFwd = append(nextFwd, step.node)
			}
		}
		fwdQueue = nextFwd

		var nextBwd []NodeID
		for _, id := range bwdQueue {
			for _, step := range traceNeighbors(g, id, false, opts) {
				if _, seen := bwdDepth[step.node]; seen {
					continue
				}
				path := append(slicesCloneSteps(bwdDepth[id]), step)
				bwdDepth[step.node] = path
				if fwdPath, ok := fwdDepth[step.node]; ok {
					return []CallPath{mergeBridgePaths(g, fwdPath, path)}
				}
				nextBwd = append(nextBwd, step.node)
			}
		}
		bwdQueue = nextBwd
	}
	return nil
}

func mergeBridgePaths(g *CallGraph, forward, backward []pathStep) CallPath {
	steps := append([]pathStep{}, forward...)
	rev := append([]pathStep{}, backward...)
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	if len(rev) > 1 {
		steps = append(steps, rev[1:]...)
	}
	return buildCallPath(g, steps, true, false)
}

type queueItem struct {
	id    NodeID
	depth int
	path  []pathStep
}

type pathStep struct {
	node NodeID
	edge EdgeKind
}

func bfsPaths(g *CallGraph, start NodeID, forward bool, opts TraceOptions) []CallPath {
	if _, ok := g.Node(start); !ok {
		return nil
	}
	var results []CallPath
	seenPath := map[string]bool{}

	queue := []queueItem{{id: start, depth: 0, path: []pathStep{{node: start}}}}
	for len(queue) > 0 && len(results) < opts.MaxPaths {
		cur := queue[0]
		queue = queue[1:]

		n, ok := g.Node(cur.id)
		if !ok {
			continue
		}
		if isTraceTerminal(n.Kind, forward, opts) && cur.depth > 0 {
			p := buildCallPath(g, cur.path, forward, false)
			if !seenPath[p.ID] {
				seenPath[p.ID] = true
				results = append(results, p)
			}
			if forward && n.Kind == KindGoBind && opts.StopAtGoBindForward {
				continue
			}
		}

		if cur.depth >= opts.MaxDepth {
			p := buildCallPath(g, cur.path, forward, true)
			if !seenPath[p.ID] {
				seenPath[p.ID] = true
				results = append(results, p)
			}
			continue
		}

		for _, step := range traceNeighbors(g, cur.id, forward, opts) {
			next := step.node
			if next == cur.id {
				continue
			}
			if pathContainsNode(cur.path, next) {
				continue
			}
			newPath := append(slicesCloneSteps(cur.path), step)
			queue = append(queue, queueItem{id: next, depth: cur.depth + 1, path: newPath})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return len(results[i].Segments) < len(results[j].Segments)
	})
	if len(results) > opts.MaxPaths {
		results = results[:opts.MaxPaths]
	}
	return results
}

func traceNeighbors(g *CallGraph, id NodeID, forward bool, opts TraceOptions) []pathStep {
	var out []pathStep
	if forward {
		edgeMap := outgoingEdges(g, id)
		for _, next := range g.Out[id] {
			edge := edgeMap[next]
			if edge == "" {
				edge = EdgeCalls
			}
			out = append(out, pathStep{node: next, edge: edge})
		}
		return out
	}

	inMap := incomingEdges(g, id)
	for _, prev := range g.In[id] {
		edge := inMap[prev]
		if edge == "" {
			edge = EdgeCalls
		}
		if !opts.IncludeGoInternal {
			if nn, ok := g.Node(prev); ok && nn.Kind == KindGoInternal {
				continue
			}
		}
		out = append(out, pathStep{node: prev, edge: edge})
	}

	if opts.IncludeEvents {
		outMap := outgoingEdges(g, id)
		for _, next := range g.Out[id] {
			edge := outMap[next]
			switch edge {
			case EdgeEmits, EdgeEventDelivers, EdgeListens:
				out = append(out, pathStep{node: next, edge: edge})
			}
		}
	}
	return out
}

func isTraceTerminal(kind NodeKind, forward bool, opts TraceOptions) bool {
	if forward {
		return kind == KindGoBind || (opts.IncludeGoInternal && kind == KindGoInternal)
	}
	return kind == KindUIComponent || kind == KindUIHandler || kind == KindHook || kind == KindBridgeCall
}

func outgoingEdges(g *CallGraph, from NodeID) map[NodeID]EdgeKind {
	out := map[NodeID]EdgeKind{}
	for _, e := range g.edges {
		if e.From == from {
			out[e.To] = e.Kind
		}
	}
	return out
}

func incomingEdges(g *CallGraph, to NodeID) map[NodeID]EdgeKind {
	out := map[NodeID]EdgeKind{}
	for _, e := range g.edges {
		if e.To == to {
			out[e.From] = e.Kind
		}
	}
	return out
}

func buildCallPath(g *CallGraph, steps []pathStep, forward bool, truncated bool) CallPath {
	dir := "forward"
	if !forward {
		dir = "backward"
	}
	segments := make([]PathSegment, 0, len(steps))
	pathKind := "rpc"
	eventChannel := ""
	for i, step := range steps {
		n, _ := g.Node(step.node)
		seg := PathSegment{Node: nodeSnapshot(n)}
		if i+1 < len(steps) {
			seg.Edge = steps[i+1].edge
			nextEdge := steps[i+1].edge
			seg.RealmCross = nextEdge == EdgeBridgeInvoke || nextEdge == EdgeEventDelivers
			if nextEdge == EdgeEventDelivers && n != nil {
				eventChannel = n.Name
				pathKind = "event"
			}
			if nextEdge == EdgeEmits && n != nil && n.Kind == KindEventEmit {
				eventChannel = n.Name
			}
		}
		segments = append(segments, seg)
	}
	id := pathHash(steps, dir)
	hint := ""
	if truncated {
		hint = fmt.Sprintf("truncated at depth %d", len(steps)-1)
	} else if eventChannel != "" {
		hint = fmt.Sprintf("via event %q", eventChannel)
	}
	return CallPath{
		ID:           id,
		Direction:    dir,
		Segments:     segments,
		Truncated:    truncated,
		Hint:         hint,
		EventChannel: eventChannel,
		PathKind:     pathKind,
	}
}

func extendForwardWithSymbols(g *CallGraph, paths []CallPath, opts TraceOptions) []CallPath {
	if opts.SymbolQuery == nil || !opts.SymbolQuery.Available() {
		return paths
	}
	ctx := opts.SymbolContext
	if ctx == nil {
		ctx = context.Background()
	}
	var extended []CallPath
	for _, p := range paths {
		if len(p.Segments) == 0 {
			extended = append(extended, p)
			continue
		}
		last := p.Segments[len(p.Segments)-1].Node
		if last.Kind != KindGoBind {
			extended = append(extended, p)
			continue
		}
		callees, err := opts.SymbolQuery.Callees(ctx, last.Name, 3)
		if err != nil || len(callees) == 0 {
			extended = append(extended, p)
			continue
		}
		for _, c := range callees {
			file := normalizeSlash(c.File)
			seg := PathSegment{
				Node: NodeSnapshot{
					ID:   NewGoInternalID(file, c.Name),
					Kind: KindGoInternal,
					Name: c.Name,
					File: file,
					Line: c.Line,
				},
				Edge: EdgeGoCalls,
			}
			ext := p
			ext.Segments = append(append([]PathSegment{}, p.Segments...), seg)
			extended = append(extended, ext)
		}
	}
	if len(extended) == 0 {
		return paths
	}
	sort.Slice(extended, func(i, j int) bool {
		return len(extended[i].Segments) < len(extended[j].Segments)
	})
	if len(extended) > opts.MaxPaths {
		extended = extended[:opts.MaxPaths]
	}
	return extended
}

func buildCallPathWithInternals(g *CallGraph, steps []pathStep, ref SymbolRef) CallPath {
	p := buildCallPath(g, steps, true, false)
	p.Segments = append(p.Segments, PathSegment{
		Node: NodeSnapshot{
			ID:   NewGoInternalID(ref.File, ref.Name),
			Kind: KindGoInternal,
			Name: ref.Name,
			File: ref.File,
			Line: ref.Line,
		},
	})
	return p
}

func pathContainsNode(steps []pathStep, id NodeID) bool {
	for _, s := range steps {
		if s.node == id {
			return true
		}
	}
	return false
}

func pathHash(steps []pathStep, dir string) string {
	h := sha256.New()
	h.Write([]byte(dir))
	for _, s := range steps {
		h.Write([]byte(string(s.node)))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)[:8])
}

func slicesCloneSteps(in []pathStep) []pathStep {
	out := make([]pathStep, len(in))
	copy(out, in)
	return out
}

// ResolveNodeID resolves shorthand to a NodeID.
func ResolveNodeID(g *CallGraph, path, symbol string) (NodeID, error) {
	if g == nil {
		return "", ErrIndexNotReady
	}
	path = normalizeSlash(path)
	symbol = strings.TrimSpace(symbol)
	if path == "" {
		return "", fmt.Errorf("path required")
	}
	var matches []NodeID
	for id, n := range g.Nodes {
		if n == nil {
			continue
		}
		if n.File != path {
			continue
		}
		if symbol == "" || n.Name == symbol || string(id) == symbol {
			matches = append(matches, id)
		}
	}
	if len(matches) == 1 {
		return matches[0], nil
	}
	if symbol != "" {
		candidate := NewUIID(path, symbol)
		if _, ok := g.Node(candidate); ok {
			return candidate, nil
		}
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("ambiguous path %s", path)
	}
	return "", ErrNodeNotFound
}
