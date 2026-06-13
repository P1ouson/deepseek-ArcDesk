package taskdag

import "testing"

func TestTaskDAGDeps(t *testing.T) {
	tr := NewTracker()
	tr.LoadFromPlan(`- db: Init schema
- auth: Add login (deps: db)
- ui: Wire form (deps: auth)`)
	ready := tr.Ready()
	if len(ready) != 1 || ready[0].ID != "db" {
		t.Fatalf("ready = %+v", ready)
	}
	if err := tr.Start("db"); err != nil {
		t.Fatal(err)
	}
	if _, err := tr.Complete("db", "schema ok"); err != nil {
		t.Fatal(err)
	}
	ready = tr.Ready()
	if len(ready) != 1 || ready[0].ID != "auth" {
		t.Fatalf("after db: ready = %+v", ready)
	}
}

func TestParseMarkdownDeps(t *testing.T) {
	nodes := ParseMarkdown("- a: Alpha\n- b: Beta (deps: a)")
	if len(nodes) != 2 || nodes[1].Deps[0] != "a" {
		t.Fatalf("nodes = %+v", nodes)
	}
}

func TestParseMarkdownUniqueTitles(t *testing.T) {
	nodes := ParseMarkdown("- Add login\n- Add logout")
	if len(nodes) != 2 {
		t.Fatalf("nodes = %d", len(nodes))
	}
	if nodes[0].ID == nodes[1].ID {
		t.Fatalf("duplicate ids: %+v", nodes)
	}
}

func TestValidateIssuesMissingDep(t *testing.T) {
	tr := NewTracker()
	tr.Load([]Node{{ID: "a", Title: "A", Deps: []string{"missing"}}})
	issues := tr.ValidateIssues()
	if len(issues) == 0 {
		t.Fatal("expected missing dep issue")
	}
}
