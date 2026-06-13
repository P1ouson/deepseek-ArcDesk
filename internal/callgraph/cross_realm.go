package callgraph

import "strings"

// CrossRealmImpact returns frontend nodes affected by changes to a go bind method.
func (idx *Index) CrossRealmImpact(goMethod string) ([]NodeSnapshot, error) {
	g, err := idx.graphForRead()
	if err != nil {
		return nil, err
	}
	method := strings.TrimSpace(goMethod)
	gobind, ok := g.MethodMap[method]
	if !ok {
		gobind, ok = g.MethodMap["App."+method]
	}
	if !ok {
		return nil, ErrNodeNotFound
	}
	opts := DefaultTraceOptions()
	opts.MaxPaths = 50
	opts.IncludeEvents = true
	paths := TraceBackward(g, gobind, opts)

	seen := map[NodeID]bool{}
	var out []NodeSnapshot
	add := func(n NodeSnapshot) {
		if n.ID == "" || seen[n.ID] {
			return
		}
		switch n.Kind {
		case KindUIComponent, KindUIHandler, KindHook:
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	for _, p := range paths {
		for _, seg := range p.Segments {
			add(seg.Node)
		}
	}
	return out, nil
}
