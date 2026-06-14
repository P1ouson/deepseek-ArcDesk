package control

import (
	"strings"
	"testing"

	"arcdesk/internal/event"
	"arcdesk/internal/skill"
)

// TestSlashCommandsDistinctResponses verifies management slash subcommands do not
// all collapse to the same generic listing output (the /mcp bug class).
func TestSlashCommandsDistinctResponses(t *testing.T) {
	skills := []skill.Skill{{
		Name:        "explore",
		Description: "Explore the codebase",
		Scope:       skill.ScopeProject,
		Path:        "/tmp/.arcdesk/skills/explore/SKILL.md",
	}}
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
		Skills:    skills,
		AllSkills: skills,
	})

	type caseSpec struct {
		cmd      string
		mustHave []string
	}
	cases := []caseSpec{
		{cmd: "/skills", mustHave: []string{"explore"}},
		{cmd: "/skills show explore", mustHave: []string{"skill: explore", "Explore the codebase"}},
		{cmd: "/skills paths", mustHave: []string{"skill paths:"}},
		{cmd: "/skills show missing", mustHave: []string{"unknown skill"}},
		{cmd: "/hooks trust", mustHave: []string{"trusted"}},
		{cmd: "/auto-plan", mustHave: []string{"auto-plan:"}},
		{cmd: "/auto-plan on", mustHave: []string{"auto-plan set to on"}},
		{cmd: "/language", mustHave: []string{"auto", "en", "zh"}},
		{cmd: "/language zh", mustHave: []string{"zh"}},
		{cmd: "/mcp show missing", mustHave: []string{"not found"}},
	}

	seen := map[string]string{}
	for _, tc := range cases {
		notices = nil
		if !c.managementNotice(tc.cmd) {
			t.Fatalf("managementNotice did not handle %q", tc.cmd)
		}
		text := lastNotice(notices)
		if text == "" {
			t.Fatalf("%q produced empty notice", tc.cmd)
		}
		for _, want := range tc.mustHave {
			if !strings.Contains(text, want) {
				t.Fatalf("%q notice missing %q:\n%s", tc.cmd, want, text)
			}
		}
		if prev, ok := seen[text]; ok && prev != tc.cmd {
			t.Fatalf("%q and %q produced identical notice:\n%s", prev, tc.cmd, text)
		}
		seen[text] = tc.cmd
	}
}

func TestHooksTrustDistinctFromList(t *testing.T) {
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.managementNotice("/hooks")
	list := lastNotice(notices)
	notices = nil
	c.managementNotice("/hooks trust")
	trust := lastNotice(notices)
	if list == trust {
		t.Fatalf("/hooks and /hooks trust should differ:\nlist=%q\ntrust=%q", list, trust)
	}
	if !strings.Contains(trust, "trusted") {
		t.Fatalf("/hooks trust = %q", trust)
	}
}

func TestAutoPlanDistinctFromBare(t *testing.T) {
	_ = isolateControlHome(t)
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.managementNotice("/auto-plan")
	bare := lastNotice(notices)
	notices = nil
	c.managementNotice("/auto-plan on")
	on := lastNotice(notices)
	if bare == on {
		t.Fatalf("/auto-plan and /auto-plan on should differ:\nbare=%q\non=%q", bare, on)
	}
}

func TestLanguageDistinctFromSet(t *testing.T) {
	_ = isolateControlHome(t)
	var notices []string
	c := New(Options{
		Sink: event.FuncSink(func(e event.Event) {
			if e.Text != "" {
				notices = append(notices, e.Text)
			}
		}),
	})
	c.managementNotice("/language")
	bare := lastNotice(notices)
	notices = nil
	c.managementNotice("/language zh")
	zh := lastNotice(notices)
	if bare == zh {
		t.Fatalf("/language and /language zh should differ:\nbare=%q\nzh=%q", bare, zh)
	}
}
