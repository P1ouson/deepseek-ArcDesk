package hook

import "testing"

func TestMCPServerTrustRoundTrip(t *testing.T) {
	home := t.TempDir()
	proj := t.TempDir()
	if IsMCPServerTrusted(proj, "filesystem", home) {
		t.Fatal("server should start untrusted")
	}
	if err := TrustMCPServer(proj, "filesystem", home); err != nil {
		t.Fatal(err)
	}
	if !IsMCPServerTrusted(proj, "filesystem", home) {
		t.Fatal("server should be trusted after TrustMCPServer")
	}
}

func TestFormatMCPCommandLine(t *testing.T) {
	got := FormatMCPCommandLine("npx", []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"})
	want := "npx -y @modelcontextprotocol/server-filesystem /tmp"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
