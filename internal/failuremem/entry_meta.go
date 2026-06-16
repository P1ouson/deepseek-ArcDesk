package failuremem

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

const (
	KindFix       = "fix"
	KindPlaybook  = "playbook"
	KindConvention = "convention"

	ConfidenceDraft         = "draft"
	ConfidenceVerified      = "verified"
	ConfidenceUserConfirmed = "user_confirmed"
	ConfidenceStale         = "stale"
)

// NormalizeEntry fills defaults and trims fields on e.
func NormalizeEntry(e *Entry) {
	if e == nil {
		return
	}
	e.Signature = strings.TrimSpace(e.Signature)
	e.Error = strings.TrimSpace(e.Error)
	e.Fix = strings.TrimSpace(e.Fix)
	if e.Kind == "" {
		e.Kind = KindFix
	}
	if e.Confidence == "" {
		if e.Hits > 0 {
			e.Confidence = ConfidenceVerified
		} else {
			e.Confidence = ConfidenceDraft
		}
	}
	if fp := Fingerprint(*e); fp != "" {
		e.ID = fp
	} else if e.ID == "" {
		e.ID = slugFromSignature(e.Signature)
	}
	if e.TS.IsZero() {
		e.TS = time.Now().UTC()
	}
}

// Fingerprint returns a stable merge key for similar experiences.
func Fingerprint(e Entry) string {
	sig := strings.ToLower(strings.TrimSpace(e.Signature))
	path := ""
	if len(e.Paths) > 0 {
		path = strings.ToLower(strings.TrimSpace(e.Paths[0]))
	}
	errLine := firstLine(e.Error)
	h := sha256.Sum256([]byte(sig + "\x00" + path + "\x00" + errLine))
	return hex.EncodeToString(h[:8])
}

func slugFromSignature(sig string) string {
	s := strings.ToLower(strings.TrimSpace(sig))
	if s == "" {
		return "entry"
	}
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		default:
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "entry"
	}
	if len(out) > 48 {
		return out[:48]
	}
	return out
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return s
}

// SummaryLine is a one-line hint for injection.
func (e Entry) SummaryLine(max int) string {
	if max <= 0 {
		max = 200
	}
	fix := strings.TrimSpace(e.Fix)
	if fix == "" {
		fix = strings.TrimSpace(e.Signature)
	}
	line := fix
	if sig := strings.TrimSpace(e.Signature); sig != "" && !strings.Contains(strings.ToLower(fix), strings.ToLower(sig)) {
		line = sig + ": " + fix
	}
	if len(line) > max {
		line = line[:max-1] + "…"
	}
	return line
}

// IsInjectable reports whether the entry may appear in retry hints.
func (e Entry) IsInjectable() bool {
	switch strings.ToLower(strings.TrimSpace(e.Confidence)) {
	case ConfidenceStale:
		return false
	default:
		return strings.TrimSpace(e.Fix) != "" || strings.TrimSpace(e.Signature) != ""
	}
}

var genericFixPhrases = []string{
	"verify passed after edits in session",
	"test passed",
	"fixed",
	"done",
}

// QualifiesForRecord rejects empty or generic lessons before they enter the store.
func QualifiesForRecord(e Entry) bool {
	NormalizeEntry(&e)
	sig := strings.TrimSpace(e.Signature)
	fix := strings.TrimSpace(e.Fix)
	if len(sig) < 4 || len(fix) < 12 {
		return false
	}
	lowFix := strings.ToLower(fix)
	for _, phrase := range genericFixPhrases {
		if lowFix == phrase || lowFix == phrase+"." {
			return false
		}
	}
	return true
}

// MergeInto updates dst with fresher data from src and increments hits.
func MergeInto(dst *Entry, src Entry) {
	if dst == nil {
		return
	}
	dst.Hits++
	if src.Fix != "" && (dst.Fix == "" || len(src.Fix) > len(dst.Fix)) {
		dst.Fix = src.Fix
	}
	if src.Error != "" {
		dst.Error = src.Error
	}
	if len(src.Paths) > 0 {
		dst.Paths = append([]string(nil), src.Paths...)
	}
	if src.RepoHead != "" {
		dst.RepoHead = src.RepoHead
	}
	if src.WorkspaceFingerprint != "" {
		dst.WorkspaceFingerprint = src.WorkspaceFingerprint
	}
	if len(src.Tags) > 0 {
		dst.Tags = append([]string(nil), src.Tags...)
	}
	dst.TS = time.Now().UTC()
	dst.LastUsedAt = dst.TS
	if dst.Confidence == ConfidenceDraft && dst.Hits >= 2 {
		dst.Confidence = ConfidenceVerified
	}
}
