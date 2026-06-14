package main

import (
	"strings"
	"testing"
)

func TestSkillRepoPathCandidates(t *testing.T) {
	got := skillRepoPathCandidates("vercel-labs/agent-skills", "vercel-react-best-practices", "vercel-labs/agent-skills/vercel-react-best-practices")
	if len(got) == 0 {
		t.Fatal("expected candidates")
	}
	if got[0] != "skills/vercel-react-best-practices" {
		t.Fatalf("first candidate = %q", got[0])
	}
}

func TestScoreSkillDirMatch(t *testing.T) {
	want := skillMatchKeys("react:components")
	if scoreSkillDirMatch("react-components", want) < 80 {
		t.Fatal("expected react-components to match react:components")
	}
	if scoreSkillDirMatch("unrelated", want) >= 0 {
		t.Fatal("expected unrelated to not match")
	}
}

func TestParseGitHubSource(t *testing.T) {
	owner, repo, err := parseGitHubSource("anthropics/skills")
	if err != nil || owner != "anthropics" || repo != "skills" {
		t.Fatalf("parseGitHubSource = %q/%q err=%v", owner, repo, err)
	}
	if _, _, err := parseGitHubSource("bad"); err == nil {
		t.Fatal("expected error for bad source")
	}
}

func TestSkillsMarketQueryForPage(t *testing.T) {
	if got := skillsMarketQueryForPage("skill", 0); got != "skill" {
		t.Fatalf("page 0 = %q", got)
	}
	if got := skillsMarketQueryForPage("skill", 1); got != "agent" {
		t.Fatalf("page 1 = %q", got)
	}
	if got := skillsMarketQueryForPage("react", 0); got != "react" {
		t.Fatalf("custom page 0 = %q", got)
	}
}

func TestSkillsMarketDestRoot(t *testing.T) {
	globalRoot, err := skillsMarketDestRoot("global", "")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(globalRoot, ".arcdesk") || !strings.Contains(globalRoot, "skills") {
		t.Fatalf("global root = %q", globalRoot)
	}
	project := t.TempDir()
	projectRoot, err := skillsMarketDestRoot("project", project)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(projectRoot, project) {
		t.Fatalf("project root = %q want under %q", projectRoot, project)
	}
}
