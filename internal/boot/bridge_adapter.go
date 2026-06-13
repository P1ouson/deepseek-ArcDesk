package boot

import (
	"arcdesk/internal/callgraph"
	"arcdesk/internal/dependency"
)

type bridgeImpactAdapter struct {
	cg *callgraph.Index
}

func newBridgeImpactAdapter(cg *callgraph.Index) dependency.BridgeImpactAnalyzer {
	if cg == nil {
		return nil
	}
	return bridgeImpactAdapter{cg: cg}
}

func (a bridgeImpactAdapter) Available() bool {
	if a.cg == nil {
		return false
	}
	return a.cg.BridgeImpactAnalyzer().Available()
}

func (a bridgeImpactAdapter) AffectedUI(goMethod string) ([]dependency.CrossRealmImpactEntry, error) {
	nodes, err := a.cg.BridgeImpactAnalyzer().AffectedUI(goMethod)
	if err != nil {
		return nil, err
	}
	out := make([]dependency.CrossRealmImpactEntry, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, dependency.CrossRealmImpactEntry{
			ID:   string(n.ID),
			Name: n.Name,
			Kind: string(n.Kind),
		})
	}
	return out, nil
}
