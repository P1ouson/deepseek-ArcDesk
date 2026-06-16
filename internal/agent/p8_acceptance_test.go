package agent

import (
	"testing"

	"arcdesk/internal/config"
	"arcdesk/internal/evidence"
	"arcdesk/internal/instruction"
	"arcdesk/internal/tool"
	"arcdesk/internal/toolcache"
)

func TestEffectiveProjectChecksStyleOnly(t *testing.T) {
	a := &Agent{
		verifySelectEnabled: true,
		evidence:          evidence.NewLedger(),
		projectChecks: []instruction.VerifyCheck{
			{Command: "npm run build", Category: "build"},
			{Command: "npm run test", Category: "unit"},
			{Command: "npm run e2e", Category: "e2e"},
		},
	}
	a.evidence.Record(evidence.Receipt{Write: true, Paths: []string{"desktop/frontend/src/App.css"}, Success: true})

	checks := a.effectiveProjectChecks()
	for _, c := range checks {
		if c.Category == "unit" || c.Category == "e2e" {
			t.Fatalf("unexpected check for css-only write: %#v", checks)
		}
	}
	if len(checks) == 0 {
		t.Fatal("expected at least build check")
	}
}

func TestToolCacheSelectiveInvalidationViaAgent(t *testing.T) {
	cache := toolcache.New()
	a := &Agent{
		toolCacheEnabled: true,
		toolCache:        cache,
	}
	cache.Put("read_file", `{"path":"a.go"}`, toolcache.Entry{Output: "a"})
	cache.Put("read_file", `{"path":"b.go"}`, toolcache.Entry{Output: "b"})

	a.invalidateToolCacheForPaths([]string{"a.go"})
	if _, ok := cache.Get("read_file", `{"path":"a.go"}`); ok {
		t.Fatal("expected a.go invalidated")
	}
	if _, ok := cache.Get("read_file", `{"path":"b.go"}`); !ok {
		t.Fatal("expected b.go still cached")
	}
}

func TestToolSchemasCached(t *testing.T) {
	reg := tool.NewRegistry()
	reg.Add(fakeTool{name: "read_file", readOnly: true})
	a := New(nil, reg, NewSession(""), Options{}, nil)
	s1 := a.toolSchemas()
	s2 := a.toolSchemas()
	if len(s1) == 0 || len(s1) != len(s2) {
		t.Fatalf("schemas len = %d / %d", len(s1), len(s2))
	}
}

func TestVerifySelectDecoupledFromPrefixRuntime(t *testing.T) {
	pr := config.PrefixRuntimeConfig{}
	a := New(nil, tool.NewRegistry(), NewSession(""), Options{PrefixRuntime: pr}, nil)
	if !a.verifySelectEnabled {
		t.Fatal("verify select should default on without prefix runtime")
	}
}
