package callgraph

// BridgeImpactAnalyzer is the callgraph export consumed by dependency for CrossRealm backfill.
type BridgeImpactAnalyzer interface {
	AffectedUI(goMethod string) ([]NodeSnapshot, error)
	Available() bool
}

type bridgeImpactAdapter struct {
	idx *Index
}

// BridgeImpactAnalyzer returns an analyzer for dependency wiring.
func (idx *Index) BridgeImpactAnalyzer() BridgeImpactAnalyzer {
	if idx == nil {
		return bridgeImpactAdapter{}
	}
	return bridgeImpactAdapter{idx: idx}
}

func (a bridgeImpactAdapter) Available() bool {
	if a.idx == nil {
		return false
	}
	g, err := a.idx.graphForRead()
	return err == nil && g != nil && len(g.Nodes) > 0
}

func (a bridgeImpactAdapter) AffectedUI(goMethod string) ([]NodeSnapshot, error) {
	if a.idx == nil {
		return nil, ErrIndexNotReady
	}
	return a.idx.CrossRealmImpact(goMethod)
}
