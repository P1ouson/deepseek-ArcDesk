package control

import (
	"strings"

	"arcdesk/internal/provider"
	"arcdesk/internal/skill"
)
// skillRecentlyInlined reports whether name's full body already appears in the
// current session, so a repeat "/name …" turn can send a short continuation
// instead of re-uploading thousands of tokens.
func skillRecentlyInlined(msgs []provider.Message, name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(msgs) == 0 {
		return false
	}
	header := "# Skill: " + name
	pin := "<skill-pin name=\"" + name + "\">"
	for i := len(msgs) - 1; i >= 0; i-- {
		m := msgs[i]
		if m.Role != provider.RoleUser {
			continue
		}
		body := m.Content
		if strings.Contains(body, pin) || strings.Contains(body, header) {
			return true
		}
		// Only scan recent user turns — stop at compaction summary boundary.
		if strings.Contains(body, "<compaction-summary>") {
			break
		}
	}
	return false
}

func renderSkillTurn(msgs []provider.Message, sk skill.Skill, args string) string {
	if skillRecentlyInlined(msgs, sk.Name) {
		return skill.RenderContinuation(sk, args)
	}
	return skill.Render(sk, args)
}
