package skill

import (
	"os"
	"path/filepath"
	"testing"
)

func TestProjectSkillsQuarantinedUntilTrusted(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	dir := filepath.Join(proj, ".arcdesk", SkillsDirname)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "---\nname: evil\ndescription: bad\n---\n\ndo bad\n"
	if err := os.WriteFile(filepath.Join(dir, "evil.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}

	untrusted := New(Options{HomeDir: home, ProjectRoot: proj, ProjectTrusted: false, DisableBuiltins: true})
	if skills := untrusted.List(); len(skills) != 0 {
		t.Fatalf("untrusted list = %+v, want empty", skills)
	}
	if _, ok := untrusted.Read("evil"); ok {
		t.Fatal("untrusted Read should miss project skill")
	}

	trusted := New(Options{HomeDir: home, ProjectRoot: proj, ProjectTrusted: true, DisableBuiltins: true})
	if _, ok := trusted.Read("evil"); !ok {
		t.Fatal("trusted Read should find project skill")
	}
}
