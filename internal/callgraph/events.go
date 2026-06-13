package callgraph

// AttachEventEmits adds event_emit nodes and emits edges from source anchors.
func AttachEventEmits(g *CallGraph, emits []EventEmitSite) []ParseWarning {
	if g == nil {
		return nil
	}
	var warnings []ParseWarning
	for _, e := range emits {
		if e.File == "" {
			continue
		}
		if e.VariableChannel {
			warnings = append(warnings, ParseWarning{
				File:    e.File,
				Line:    e.Line,
				Message: "event_emit_variable_channel",
			})
			continue
		}
		if e.Channel == "" {
			continue
		}
		id := NewEventEmitID(e.File, e.Line, e.Channel)
		g.AddNode(&Node{
			ID:   id,
			Kind: KindEventEmit,
			Name: e.Channel,
			File: e.File,
			Line: e.Line,
		})
		from := e.From
		if from == "" {
			from = NewGoInternalID(e.File, "package")
			g.AddNode(&Node{ID: from, Kind: KindGoInternal, Name: "package", File: e.File})
		}
		g.AddEdge(from, id, EdgeEmits)
	}
	return warnings
}

// LinkEventDelivers connects event_emit nodes to event_listen by channel name.
func LinkEventDelivers(g *CallGraph) int {
	if g == nil {
		return 0
	}
	byChannel := map[string][]NodeID{}
	for id, n := range g.Nodes {
		if n == nil {
			continue
		}
		switch n.Kind {
		case KindEventEmit:
			byChannel[n.Name] = append(byChannel[n.Name], id)
		}
	}
	count := 0
	for id, n := range g.Nodes {
		if n == nil || n.Kind != KindEventListen {
			continue
		}
		for _, emitID := range byChannel[n.Name] {
			g.AddEdge(emitID, id, EdgeEventDelivers)
			count++
		}
	}
	return count
}

// LinkListenEdges connects event_listen nodes to UI handlers/components.
func LinkListenEdges(g *CallGraph, listens []TSListen) {
	if g == nil {
		return
	}
	for _, l := range listens {
		to := l.Scope
		if to == "" {
			continue
		}
		g.AddEdge(l.ID, to, EdgeListens)
	}
}
