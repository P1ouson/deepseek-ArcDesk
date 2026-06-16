package repomap

import (
	"testing"
)

func TestWorkspaceRevisionCachedUntilInvalidate(t *testing.T) {
	root := t.TempDir()
	head1, _ := WorkspaceRevision(root)
	head2, _ := WorkspaceRevision(root)
	if head1 != head2 {
		t.Fatalf("revision1=%q revision2=%q", head1, head2)
	}
	InvalidateWorkspaceRevision(root)
	head3, _ := WorkspaceRevision(root)
	if head1 != head3 {
		// non-git fingerprint should be stable for same dir
		t.Fatalf("after invalidate head1=%q head3=%q", head1, head3)
	}
}
