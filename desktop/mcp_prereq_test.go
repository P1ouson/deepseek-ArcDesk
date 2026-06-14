package main

import "testing"

func TestCheckMCPPrerequisitesNode(t *testing.T) {
	view := NewApp().CheckMCPPrerequisites([]string{"node", "node"})
	if len(view.Items) != 1 {
		t.Fatalf("len(items) = %d, want 1 deduped", len(view.Items))
	}
	if view.Items[0].ID != "node" {
		t.Fatalf("id = %q, want node", view.Items[0].ID)
	}
}

func TestCatalogRequiresNodeInference(t *testing.T) {
	if !catalogRequiresNode(MCPCatalogEntry{Transport: "stdio", Command: "npx"}) {
		t.Fatal("expected npx stdio entry to require node")
	}
	if catalogRequiresNode(MCPCatalogEntry{Transport: "http", URL: "https://example.com"}) {
		t.Fatal("http entry should not require node by default")
	}
}

func TestInferredCatalogRequiresPrefersExplicit(t *testing.T) {
	got := inferredCatalogRequires(MCPCatalogEntry{Requires: []string{"network"}, Transport: "stdio", Command: "npx"})
	if len(got) != 1 || got[0] != "network" {
		t.Fatalf("requires = %v, want [network]", got)
	}
}
