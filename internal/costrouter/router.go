package costrouter

import (
	"strings"
)

// Tier is a coarse task class for model routing.
type Tier string

const (
	TierClassify Tier = "classify"
	TierExecute  Tier = "execute"
	TierCompact  Tier = "compact"
	TierExplore  Tier = "explore"
)

// Config holds per-tier model references (provider names from arcdesk.toml).
type Config struct {
	Enabled       bool
	DefaultModel  string
	ClassifyModel string
	ExecuteModel  string
	CompactModel  string
	ExploreModel  string
}

// Router resolves tiers to configured model names.
type Router struct {
	cfg Config
}

// New returns a router. Disabled routers still classify but return DefaultModel.
func New(cfg Config) *Router {
	return &Router{cfg: cfg}
}

// Enabled reports whether tier overrides are active.
func (r *Router) Enabled() bool {
	return r != nil && r.cfg.Enabled
}

// Classify picks a tier from a user/sub-agent prompt using lightweight heuristics.
func (r *Router) Classify(prompt string) Tier {
	p := strings.ToLower(strings.TrimSpace(prompt))
	if p == "" {
		return TierExecute
	}
	switch {
	case strings.Contains(p, "compact") || strings.Contains(p, "summarize") || strings.Contains(p, "summary"):
		return TierCompact
	case strings.Contains(p, "explore the") || strings.Contains(p, "explore this") ||
		strings.Contains(p, "search codebase") || strings.Contains(p, "find where") ||
		strings.HasPrefix(p, "look for "):
		return TierExplore
	case len(p) < 120 && (strings.HasSuffix(p, "?") || strings.HasPrefix(p, "what ") || strings.HasPrefix(p, "how ")):
		return TierClassify
	default:
		return TierExecute
	}
}

// ModelForTier returns the configured provider name for tier (may be empty).
func (r *Router) ModelForTier(t Tier) string {
	if r == nil || !r.cfg.Enabled {
		return ""
	}
	switch t {
	case TierClassify:
		return strings.TrimSpace(r.cfg.ClassifyModel)
	case TierExecute:
		return strings.TrimSpace(r.cfg.ExecuteModel)
	case TierCompact:
		return strings.TrimSpace(r.cfg.CompactModel)
	case TierExplore:
		return strings.TrimSpace(r.cfg.ExploreModel)
	default:
		return ""
	}
}

// ResolveModel picks tier then model, falling back to defaultModel when unset.
func (r *Router) ResolveModel(prompt, defaultModel string) (Tier, string) {
	if r == nil {
		return TierExecute, defaultModel
	}
	tier := r.Classify(prompt)
	if m := r.ModelForTier(tier); m != "" {
		return tier, m
	}
	if m := strings.TrimSpace(r.cfg.DefaultModel); m != "" {
		return tier, m
	}
	return tier, defaultModel
}

// Snapshot is exposed through cost_router_status.
type Snapshot struct {
	Enabled      bool              `json:"enabled"`
	DefaultModel string            `json:"default_model,omitempty"`
	Tiers        map[Tier]string   `json:"tiers,omitempty"`
}

// Snapshot returns current routing table.
func (r *Router) Snapshot() Snapshot {
	if r == nil {
		return Snapshot{}
	}
	s := Snapshot{
		Enabled:      r.cfg.Enabled,
		DefaultModel: r.cfg.DefaultModel,
		Tiers: map[Tier]string{
			TierClassify: r.cfg.ClassifyModel,
			TierExecute:  r.cfg.ExecuteModel,
			TierCompact:  r.cfg.CompactModel,
			TierExplore:  r.cfg.ExploreModel,
		},
	}
	return s
}
