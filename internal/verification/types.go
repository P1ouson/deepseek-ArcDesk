package verification

// Category classifies verification checks for discovery and retry hints.
type Category string

const (
	CategoryBuild  Category = "build"
	CategoryUnit   Category = "unit"
	CategoryE2E    Category = "e2e"
	CategoryCustom Category = "custom"
)

// Plan is the resolved verification set for one workspace session.
type Plan struct {
	Checks []Check `json:"checks"`
	Policy Policy  `json:"policy"`
}

// Check is one host-observable command the agent must run after writes.
type Check struct {
	Command    string   `json:"command"`
	Category   Category `json:"category"`
	SourcePath string   `json:"sourcePath,omitempty"`
	Line       int      `json:"line,omitempty"`
}

func categoryOf(c string) Category {
	switch Category(c) {
	case CategoryBuild, CategoryUnit, CategoryE2E:
		return Category(c)
	default:
		return CategoryCustom
	}
}
