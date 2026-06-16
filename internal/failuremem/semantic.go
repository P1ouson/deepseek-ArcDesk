package failuremem

import (
	"strings"
	"unicode"
)

// SemanticSettings controls Phase-6 fallback when exact ranked scores are weak.
type SemanticSettings struct {
	Enabled          bool
	MinExactScore    float64 // top exact score below this triggers semantic fallback
	MinSemanticScore float64 // minimum token similarity to return a semantic hit
}

func (s SemanticSettings) withDefaults() SemanticSettings {
	if s.MinExactScore <= 0 {
		s.MinExactScore = 0.5
	}
	if s.MinSemanticScore <= 0 {
		s.MinSemanticScore = 0.35
	}
	return s
}

// MatchKind describes how a ranked row was retrieved.
type MatchKind string

const (
	MatchExact    MatchKind = "exact"
	MatchSemantic MatchKind = "semantic"
)

// RankedMatch is one scored retrieval row.
type RankedMatch struct {
	Entry     Entry
	Score     float64
	TextScore float64
	Kind      MatchKind
}

// RankedSearchSmart runs exact scoring first; on miss or weak top score it falls
// back to lightweight token similarity (no embedding API).
func (s *Store) RankedSearchSmart(ctx SearchContext, query string, paths []string, limit int, sem SemanticSettings) ([]RankedMatch, error) {
	if s == nil {
		return nil, errStoreNil
	}
	if limit <= 0 {
		limit = 3
	}
	sem = sem.withDefaults()
	ctx = ctx.withDefaults()

	entries, err := s.List(s.maxEntries)
	if err != nil {
		return nil, err
	}

	exact := rankExactEntries(entries, ctx, query, paths, limit)
	if !sem.Enabled || len(exact) == 0 && len(entries) == 0 {
		return exact, nil
	}
	if !sem.Enabled {
		return exact, nil
	}
	if len(exact) > 0 && exact[0].TextScore >= sem.MinExactScore {
		return exact, nil
	}

	semantic := rankSemanticEntries(entries, ctx, query, paths, limit, sem.MinSemanticScore)
	if len(semantic) == 0 {
		return exact, nil
	}
	if len(exact) == 0 {
		return semantic, nil
	}
	// Weak exact text: prefer semantic playbook over path-only exact rows.
	return mergeRanked(semantic, exact, limit), nil
}

func rankExactEntries(entries []Entry, ctx SearchContext, query string, paths []string, limit int) []RankedMatch {
	q := strings.ToLower(strings.TrimSpace(query))
	var ranked []scoredEntry
	for _, e := range entries {
		NormalizeEntry(&e)
		if !e.IsInjectable() {
			continue
		}
		if st := e.ProvenanceStatus(ctx); !st.AutoInjectable {
			continue
		}
		sc := scoreEntry(e, q, paths)
		if sc <= 0 {
			continue
		}
		ranked = append(ranked, scoredEntry{e: e, score: sc, textScore: scoreEntryText(e, q)})
	}
	sortEntriesByScore(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]RankedMatch, len(ranked))
	for i, r := range ranked {
		out[i] = RankedMatch{Entry: r.e, Score: r.score, TextScore: r.textScore, Kind: MatchExact}
	}
	return out
}

func rankSemanticEntries(entries []Entry, ctx SearchContext, query string, paths []string, limit int, minScore float64) []RankedMatch {
	corpus := strings.TrimSpace(query + " " + strings.Join(paths, " "))
	if corpus == "" {
		return nil
	}
	qTokens := tokenSet(corpus)
	if len(qTokens) == 0 {
		return nil
	}
	var ranked []scoredEntry
	for _, e := range entries {
		NormalizeEntry(&e)
		if !e.IsInjectable() {
			continue
		}
		if st := e.ProvenanceStatus(ctx); !st.AutoInjectable {
			continue
		}
		doc := entryDocument(e)
		sc := tokenSimilarity(qTokens, tokenSet(doc))
		if sc < minScore {
			continue
		}
		for _, p := range paths {
			p = strings.ToLower(strings.TrimSpace(p))
			if p == "" {
				continue
			}
			for _, ep := range e.Paths {
				if strings.Contains(strings.ToLower(ep), p) || strings.Contains(p, strings.ToLower(ep)) {
					sc += 0.1
					break
				}
			}
		}
		if sc > 1 {
			sc = 1
		}
		ranked = append(ranked, scoredEntry{e: e, score: sc})
	}
	sortEntriesByScore(ranked)
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]RankedMatch, len(ranked))
	for i, r := range ranked {
		out[i] = RankedMatch{Entry: r.e, Score: r.score, Kind: MatchSemantic}
	}
	return out
}

func mergeRanked(primary, secondary []RankedMatch, limit int) []RankedMatch {
	seen := map[string]bool{}
	var out []RankedMatch
	add := func(rows []RankedMatch) {
		for _, r := range rows {
			fp := Fingerprint(r.Entry)
			if fp != "" && seen[fp] {
				continue
			}
			if fp != "" {
				seen[fp] = true
			}
			out = append(out, r)
			if len(out) >= limit {
				return
			}
		}
	}
	add(primary)
	if len(out) < limit {
		add(secondary)
	}
	return out
}

func entryDocument(e Entry) string {
	var parts []string
	parts = append(parts, e.Signature, e.Error, e.Fix)
	parts = append(parts, e.Paths...)
	return strings.Join(parts, " ")
}

func tokenSet(text string) map[string]bool {
	text = strings.ToLower(text)
	var b strings.Builder
	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	set := make(map[string]bool)
	for _, tok := range strings.Fields(b.String()) {
		if len(tok) < 2 {
			continue
		}
		set[tok] = true
	}
	return set
}

func tokenSimilarity(query, doc map[string]bool) float64 {
	if len(query) == 0 || len(doc) == 0 {
		return 0
	}
	matched := 0
	for qt := range query {
		for dt := range doc {
			if qt == dt || strings.Contains(qt, dt) || strings.Contains(dt, qt) {
				matched++
				break
			}
		}
	}
	if matched == 0 {
		return 0
	}
	recall := float64(matched) / float64(len(query))
	union := len(query) + len(doc) - matched
	if union <= 0 {
		return recall
	}
	jaccard := float64(matched) / float64(union)
	if recall > jaccard {
		return recall
	}
	return jaccard
}

// FixSkeleton returns a step-oriented playbook excerpt when fix text looks structured.
func FixSkeleton(fix string) string {
	fix = strings.TrimSpace(fix)
	if fix == "" {
		return ""
	}
	lines := strings.Split(fix, "\n")
	var steps []string
	for _, ln := range lines {
		ln = strings.TrimSpace(ln)
		if ln == "" {
			continue
		}
		trim := strings.TrimSpace(ln)
		if strings.HasPrefix(trim, "- ") || strings.HasPrefix(trim, "* ") {
			steps = append(steps, trim)
			continue
		}
		if i := strings.Index(trim, ". "); i > 0 && i <= 3 {
			allDigit := true
			for _, r := range trim[:i] {
				if r < '0' || r > '9' {
					allDigit = false
					break
				}
			}
			if allDigit {
				steps = append(steps, trim)
			}
		}
	}
	if len(steps) >= 2 {
		return strings.Join(steps, "; ")
	}
	if i := strings.IndexByte(fix, '\n'); i > 0 {
		return strings.TrimSpace(fix[:i])
	}
	return fix
}

var errStoreNil = errorString("failure memory not configured")

type errorString string

func (e errorString) Error() string { return string(e) }
