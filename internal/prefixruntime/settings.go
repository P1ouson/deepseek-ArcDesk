package prefixruntime

// Settings controls Phase-7 prefix stability and cache-health behavior.
type Settings struct {
	Enabled          bool
	MinHitRate       float64 // session hit/(hit+miss) below this triggers heal mode
	MinStepsForAlert int     // minimum API steps before low-hit alerts
	PrewarmOnOpen    bool
}

func (s Settings) withDefaults() Settings {
	if s.MinHitRate <= 0 {
		s.MinHitRate = 0.4
	}
	if s.MinStepsForAlert <= 0 {
		s.MinStepsForAlert = 2
	}
	return s
}

func (s Settings) Active() bool { return s.Enabled }
