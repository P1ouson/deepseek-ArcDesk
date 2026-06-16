package control

import (
	"strings"
	"testing"

	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
)

func TestSkillRecentlyInlined(t *testing.T) {
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "<skill-pin name=\"copywriting\">\n# Skill: copywriting\nbody\n</skill-pin>"},
	}
	if !skillRecentlyInlined(msgs, "copywriting") {
		t.Fatal("expected copywriting to be detected as recently inlined")
	}
	if skillRecentlyInlined(msgs, "explore") {
		t.Fatal("explore should not match copywriting pin")
	}
}

func TestRenderSkillTurnContinuation(t *testing.T) {
	sk := skill.Skill{Name: "copywriting", Body: strings.Repeat("x", 5000)}
	msgs := []provider.Message{
		{Role: provider.RoleUser, Content: "# Skill: copywriting\n" + sk.Body},
	}
	full := renderSkillTurn(nil, sk, "task")
	if len(full) < 1000 {
		t.Fatalf("first turn should inline full body, len=%d", len(full))
	}
	cont := renderSkillTurn(msgs, sk, "follow up")
	if strings.Contains(cont, sk.Body) {
		t.Fatalf("continuation should not repeat body: len=%d", len(cont))
	}
	if !strings.Contains(cont, "already loaded") {
		t.Fatalf("continuation hint missing: %q", cont)
	}
}
