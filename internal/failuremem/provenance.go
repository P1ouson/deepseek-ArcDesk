package failuremem

import (
	"strings"
	"time"

	"arcdesk/internal/repomap"
)

// SearchContext supplies workspace revision and retention policy for retrieval.
type SearchContext struct {
	RepoHead             string
	WorkspaceFingerprint string
	Now                  time.Time
	TTLDays              int
	RequireMatchingHead  bool
}

// ProvenanceStatus describes whether an entry may be auto-injected.
type ProvenanceStatus struct {
	AutoInjectable bool
	Reason         string // commit_mismatch | expired | stale_confidence
	HintHead       string
}

// NewSearchContext resolves current workspace revision for retrieval filters.
func NewSearchContext(workspaceRoot string, ttlDays int, requireMatchingHead bool) SearchContext {
	head, fp := WorkspaceProvenance(workspaceRoot)
	ctx := SearchContext{
		RepoHead:             head,
		WorkspaceFingerprint: fp,
		Now:                  time.Now().UTC(),
		TTLDays:              ttlDays,
		RequireMatchingHead:  requireMatchingHead,
	}
	return ctx.withDefaults()
}

func (c SearchContext) withDefaults() SearchContext {
	if c.Now.IsZero() {
		c.Now = time.Now().UTC()
	}
	if c.TTLDays <= 0 {
		c.TTLDays = 90
	}
	return c
}

// WorkspaceProvenance reads git HEAD or a non-git fingerprint for workspaceRoot.
func WorkspaceProvenance(workspaceRoot string) (gitHead, fingerprint string) {
	return repomap.WorkspaceRevision(workspaceRoot)
}

// StampProvenance fills missing provenance fields from the workspace root.
func (s *Store) StampProvenance(e *Entry) {
	if s == nil || e == nil {
		return
	}
	head, fp := WorkspaceProvenance(s.root)
	if e.RepoHead == "" {
		e.RepoHead = head
	}
	if e.WorkspaceFingerprint == "" {
		e.WorkspaceFingerprint = fp
	}
}

// ProvenanceStatus evaluates auto-injection eligibility for e under ctx.
func (e Entry) ProvenanceStatus(ctx SearchContext) ProvenanceStatus {
	ctx = ctx.withDefaults()
	if !e.IsInjectable() {
		return ProvenanceStatus{Reason: "stale_confidence"}
	}
	if ctx.TTLDays > 0 && !e.TS.IsZero() {
		age := ctx.Now.Sub(e.TS.UTC())
		if age > time.Duration(ctx.TTLDays)*24*time.Hour {
			return ProvenanceStatus{Reason: "expired", HintHead: shortHead(e.RepoHead)}
		}
	}
	if ctx.RequireMatchingHead && e.RepoHead != "" && ctx.RepoHead != "" && e.RepoHead != ctx.RepoHead {
		return ProvenanceStatus{Reason: "commit_mismatch", HintHead: shortHead(e.RepoHead)}
	}
	head := shortHead(e.RepoHead)
	if head == "" {
		head = shortHead(ctx.RepoHead)
	}
	return ProvenanceStatus{AutoInjectable: true, HintHead: head}
}

func shortHead(full string) string {
	full = strings.TrimSpace(full)
	if len(full) <= 7 {
		return full
	}
	return full[:7]
}
