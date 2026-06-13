package dependency

// CrossRealmImpactEntry is one UI node affected across the Wails bridge.
type CrossRealmImpactEntry struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"` // ui_component, ui_handler, hook
}

// BridgeImpactAnalyzer is implemented by callgraph and wired at boot.
type BridgeImpactAnalyzer interface {
	AffectedUI(goMethod string) ([]CrossRealmImpactEntry, error)
	Available() bool
}
