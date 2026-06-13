package callgraph

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// Breakpoint is a suggested debug stop along a Wails cross-realm path.
type Breakpoint struct {
	File   string   `json:"file"`
	Line   int      `json:"line,omitempty"`
	Symbol string   `json:"symbol"`
	Kind   NodeKind `json:"kind"`
	Layer  string   `json:"layer"` // ui | bridge | go | event
	Reason string   `json:"reason"`
}

// BreakpointsFromPath extracts layered debug stops from one traced path.
func BreakpointsFromPath(p CallPath) []Breakpoint {
	if len(p.Segments) == 0 {
		return nil
	}
	var out []Breakpoint
	seenLayer := map[string]bool{}
	for _, seg := range p.Segments {
		n := seg.Node
		if n.ID == "" {
			continue
		}
		layer, reason := breakpointLayer(n, seg.Edge, p.PathKind)
		if layer == "" || seenLayer[layer] {
			continue
		}
		seenLayer[layer] = true
		out = append(out, Breakpoint{
			File:   n.File,
			Line:   n.Line,
			Symbol: n.Name,
			Kind:   n.Kind,
			Layer:  layer,
			Reason: reason,
		})
	}
	return out
}

func breakpointLayer(n NodeSnapshot, edge EdgeKind, pathKind string) (layer, reason string) {
	switch n.Kind {
	case KindUIHandler, KindUIComponent, KindHook:
		return "ui", "UI handler/component — confirm click/input reaches bridge"
	case KindBridgeCall, KindTSFunction:
		if edge == EdgeBridgeInvoke || n.Kind == KindBridgeCall || strings.Contains(n.Name, "app.") {
			return "bridge", "Wails bridge call — confirm RPC args cross the boundary"
		}
	case KindGoBind:
		return "go", "Go bind method — confirm backend logic and errors"
	case KindEventListen:
		if pathKind == "event" || edge == EdgeEventDelivers || edge == EdgeListens {
			return "event", "Frontend event listener — confirm event payload and handler"
		}
	case KindEventEmit:
		if pathKind == "event" || edge == EdgeEmits {
			return "event", "Go EventsEmit site — confirm channel name and payload"
		}
	}
	return "", ""
}

// SuggestBreakpoints returns deduped stops from the best traced paths.
func SuggestBreakpoints(paths []CallPath) []Breakpoint {
	seen := map[string]bool{}
	var out []Breakpoint
	for _, p := range paths {
		for _, bp := range BreakpointsFromPath(p) {
			key := bp.File + ":" + fmt.Sprint(bp.Line) + "#" + bp.Symbol + "@" + bp.Layer
			if seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, bp)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		order := map[string]int{"ui": 0, "bridge": 1, "go": 2, "event": 3}
		if order[out[i].Layer] != order[out[j].Layer] {
			return order[out[i].Layer] < order[out[j].Layer]
		}
		return out[i].File < out[j].File
	})
	return out
}

// AutoBreakpoints suggests debug stops for cross-realm file changes.
func (idx *Index) AutoBreakpoints(ctx context.Context, changedPaths []string, goMethod string) ([]Breakpoint, error) {
	if idx == nil {
		return nil, ErrIndexNotReady
	}
	if !crossRealmChange(idx, changedPaths) {
		return nil, nil
	}
	g, err := idx.graphForRead()
	if err != nil {
		return nil, err
	}
	var paths []CallPath
	for _, path := range changedPaths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		id, err := ResolveNodeID(g, path, "")
		if err != nil {
			for _, p := range pathsFromGoFile(g, path) {
				paths = append(paths, p)
			}
			continue
		}
		opts := DefaultTraceOptions()
		opts.MaxPaths = 2
		fwd := TraceForward(g, id, opts)
		if len(fwd) > 0 {
			paths = append(paths, fwd...)
			continue
		}
		paths = append(paths, TraceBackward(g, id, opts)...)
	}
	if goMethod != "" {
		opts := DefaultTraceOptions()
		opts.MaxPaths = 2
		if back, err := idx.TraceBackward(ctx, goMethod, opts); err == nil {
			paths = append(paths, back...)
		}
	}
	if len(paths) == 0 {
		return nil, nil
	}
	return SuggestBreakpoints(paths), nil
}

// BreakpointsForQuery resolves a frontend anchor and/or go method to debug stops.
func (idx *Index) BreakpointsForQuery(ctx context.Context, path, symbol, goMethod string) ([]Breakpoint, []CallPath, error) {
	g, err := idx.graphForRead()
	if err != nil {
		return nil, nil, err
	}
	opts := DefaultTraceOptions()
	opts.MaxPaths = 3
	var paths []CallPath
	path = strings.TrimSpace(path)
	symbol = strings.TrimSpace(symbol)
	goMethod = strings.TrimSpace(goMethod)

	if path != "" {
		id, err := ResolveNodeID(g, path, symbol)
		if err != nil {
			return nil, nil, err
		}
		paths = TraceForward(g, id, opts)
		if len(paths) == 0 {
			paths = TraceBackward(g, id, opts)
		}
	}
	if goMethod != "" {
		gobind, ok := g.MethodMap[goMethod]
		if !ok {
			gobind, ok = g.MethodMap["App."+goMethod]
		}
		if !ok {
			return nil, nil, ErrNodeNotFound
		}
		back := TraceBackward(g, gobind, opts)
		paths = append(paths, back...)
		if path != "" {
			if id, err := ResolveNodeID(g, path, symbol); err == nil {
				if bridged, err := FindBridgePath(g, id, goMethod); err == nil {
					paths = append(paths, bridged...)
				}
			}
		}
	}
	if len(paths) == 0 {
		return nil, nil, fmt.Errorf("no call paths matched")
	}
	sort.Slice(paths, func(i, j int) bool {
		return len(paths[i].Segments) < len(paths[j].Segments)
	})
	if len(paths) > opts.MaxPaths {
		paths = paths[:opts.MaxPaths]
	}
	return SuggestBreakpoints(paths), paths, nil
}
