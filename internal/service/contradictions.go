package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"github.com/alxxpersonal/stardust/internal/gitx"
	"github.com/alxxpersonal/stardust/internal/temporal"
	"github.com/alxxpersonal/stardust/internal/vault"
)

// --- Tuning constants (precision over recall at every knob) ---

const (
	// contradictionRecallLimit bounds how many same-subject B-side notes each
	// anchor recalls through hybrid retrieval.
	contradictionRecallLimit = 8
	// contradictionScoreFloor is the minimum fused retrieval score for a recalled
	// B-side note to count as demonstrably on the same subject as the anchor. RRF
	// scores are rank-based, so this floor keeps only notes recalled near the top.
	contradictionScoreFloor = 0.012
	// contradictionJaccardFloor is the minimum shared-term Jaccard over
	// non-stopword tokens. It forces a shared subject and kills topically-near but
	// different-subject pairs that embedding similarity alone would pair.
	contradictionJaccardFloor = 0.12
	// contradictionMinSharedTerms is the minimum count of shared content terms; a
	// single incidental overlap is not a shared subject.
	contradictionMinSharedTerms = 1
	// contradictionMinAnchorTokens is the minimum content tokens an A-side anchor
	// line must carry to seed a recall; too few and there is no subject to match.
	contradictionMinAnchorTokens = 2
	// contradictionPerAnchorCap bounds candidates generated per A-side anchor.
	contradictionPerAnchorCap = 3
	// contradictionDefaultLimit is the hard ceiling on total candidates. The cap is
	// a precision instrument: a run that would emit hundreds is mistuned.
	contradictionDefaultLimit = 20
)

// contradictionCursorKey is the contradiction-specific commit cursor. It is
// distinct from last_digest_sha so a digest advance never blinds the scan.
const contradictionCursorKey = "last_contradiction_sha"

// contradictionHedge is the fixed review-prompt hedge appended to every rendered
// candidate, in the drift idiom (ADR 0018): a candidate is never a verdict.
const contradictionHedge = "This is a candidate, not a verdict, and is likely benign; confirm before acting."

// --- Types ---

// Contradiction is one cross-note contradiction candidate: an assertion-bearing
// line on the recently-changed A-side and an opposing same-subject line on the
// B-side, carrying enough evidence for a human or an agent to judge without
// re-deriving the signal. It is a review prompt, never a verdict.
type Contradiction struct {
	NoteA           string   `json:"note_a"`
	LineA           string   `json:"line_a"`
	NoteB           string   `json:"note_b"`
	LineB           string   `json:"line_b"`
	Score           float64  `json:"score"`
	SharedTerms     []string `json:"shared_terms"`
	Cue             string   `json:"cue"`
	RetrievalMode   string   `json:"retrieval_mode"`
	RetrievalReason string   `json:"retrieval_reason,omitempty"`
}

// ContradictionOptions configures a contradiction scan, mirroring the digest
// command surface.
type ContradictionOptions struct {
	Since   string // git SHA to diff the A-side from; empty uses the cursor
	All     bool   // sweep every tracked note as the A-side (ignores the cursor)
	Advance bool   // advance the contradiction cursor to HEAD after the run
	Limit   int    // hard cap on candidates; zero or less uses the default
}

// ContradictionsResult is the outcome of a contradiction scan: the ranked,
// deduped, capped candidate list plus rendered markdown. RetrievalMode announces
// whether recall ran hybrid-semantic or degraded to fts-only, so the surface is
// never a silent weaker result (ADR 0016).
type ContradictionsResult struct {
	Since           string          `json:"since"`
	Head            string          `json:"head"`
	Scanned         int             `json:"scanned"`
	RetrievalMode   string          `json:"retrieval_mode"`
	RetrievalReason string          `json:"retrieval_reason,omitempty"`
	Candidates      []Contradiction `json:"candidates"`
	Markdown        string          `json:"markdown"`
}

// --- Core method ---

// scoredCandidate carries a candidate with the combined similarity-plus-overlap
// score it is ranked by.
type scoredCandidate struct {
	cand     Contradiction
	combined float64
}

// Contradictions prepares a short, high-precision list of cross-note
// contradiction candidate pairs deterministically, with no LLM: it scopes the
// A-side to notes changed since the contradiction cursor (or --since / --all),
// filters those to assertion-bearing anchor lines, recalls same-subject B-side
// chunks through hybrid retrieval, and keeps only pairs that pass every
// conservative gate. The binary never judges whether a pair truly conflicts;
// each candidate is a review prompt an agent judges downstream (ADR 0043).
func (s *Service) Contradictions(ctx context.Context, opts ContradictionOptions) (ContradictionsResult, error) {
	if !gitx.IsRepo(ctx, s.Layout.Root) {
		return ContradictionsResult{}, fmt.Errorf("contradictions: %s is not a git repository", s.Layout.Root)
	}
	head, _ := gitx.HeadSHA(ctx, s.Layout.Root)

	since := opts.Since
	if opts.All {
		since = ""
	} else if since == "" {
		since, _ = s.store.GetMeta(ctx, contradictionCursorKey)
	}
	changed, err := gitx.DiffNames(ctx, s.Layout.Root, since)
	if err != nil {
		return ContradictionsResult{}, err
	}
	changed = filterIgnored(changed, s.Config.Ignore)

	limit := opts.Limit
	if limit <= 0 {
		limit = contradictionDefaultLimit
	}

	// probe the embedder once; the recall mode is uniform across the run and
	// announced on the result so a degraded scan is never silent (ADR 0016).
	available := s.embed.Available(ctx)
	mode := RetrievalFTSOnly
	reason := ftsOnlyReason
	if available {
		mode = RetrievalHybridSemantic
		reason = ""
	}

	directoryIndexPaths, err := s.directoryIndexPathSet()
	if err != nil {
		return ContradictionsResult{}, err
	}

	scanned := 0
	var scored []scoredCandidate
	for _, relA := range changed {
		relA = filepath.ToSlash(relA)
		if s.contradictionExcluded(relA, directoryIndexPaths) {
			continue
		}
		if _, statErr := os.Stat(filepath.Join(s.Layout.Root, relA)); statErr != nil {
			continue // deleted on the A-side, nothing to assert
		}
		noteA, parseErr := vault.Parse(s.Layout.Root, relA)
		if parseErr != nil {
			continue
		}
		if hasSupersedeFrontmatter(noteA) {
			continue // A is a sanctioned superseded record, not a contradiction source
		}
		scanned++
		pairs, err := s.contradictionsForNote(ctx, relA, noteA, available, mode, reason, directoryIndexPaths)
		if err != nil {
			return ContradictionsResult{}, err
		}
		scored = append(scored, pairs...)
	}

	candidates := rankDedupeCap(scored, limit)
	if opts.Advance && head != "" {
		_ = s.store.SetMeta(ctx, contradictionCursorKey, head)
	}

	res := ContradictionsResult{
		Since:           since,
		Head:            head,
		Scanned:         scanned,
		RetrievalMode:   mode,
		RetrievalReason: reason,
		Candidates:      candidates,
	}
	res.Markdown = renderContradictions(res)
	return res, nil
}

// contradictionsForNote generates the scored candidate pairs anchored in noteA:
// for each assertion-bearing anchor line it recalls same-subject B-side notes
// through hybrid retrieval and keeps only the pairs that pass every gate.
func (s *Service) contradictionsForNote(ctx context.Context, relA string, noteA vault.Note, available bool, mode, reason string, directoryIndexPaths map[string]bool) ([]scoredCandidate, error) {
	var out []scoredCandidate
	for _, anchor := range anchorLines(noteA.Body) {
		aTokens := contentTokens(anchor.Line)
		if len(aTokens) < contradictionMinAnchorTokens {
			continue
		}
		var queryVec []float32
		if available {
			queryVec = s.embedOne(ctx, anchor.Line)
		}
		hits, err := s.store.Hybrid(ctx, anchor.Line, queryVec, contradictionRecallLimit)
		if err != nil {
			return nil, err
		}
		perAnchor := 0
		for _, h := range hits {
			if perAnchor >= contradictionPerAnchorCap {
				break
			}
			relB := filepath.ToSlash(h.Path)
			if relB == relA {
				continue // different notes only
			}
			if h.Score < contradictionScoreFloor {
				continue // not demonstrably on the same subject
			}
			if s.contradictionExcluded(relB, directoryIndexPaths) {
				continue
			}
			if vault.CollectionKey(relA) == vault.CollectionKey(relB) {
				continue // same collection record versioned over time
			}
			noteB, parseErr := vault.Parse(s.Layout.Root, relB)
			if parseErr != nil {
				continue
			}
			if hasSupersedeFrontmatter(noteB) {
				continue // sanctioned change-your-mind path (SPEC 12.3), not a contradiction
			}
			opposing, shared, jac, ok := bestOpposingLine(noteB.Body, anchor, aTokens)
			if !ok {
				continue
			}
			cue := anchor.Cue
			if !anchor.Reversal {
				cue = opposing.Cue // the reversal side carries the discriminating cue
			}
			cand := Contradiction{
				NoteA:           relA,
				LineA:           anchor.Line,
				NoteB:           relB,
				LineB:           opposing.Line,
				Score:           h.Score,
				SharedTerms:     shared,
				Cue:             cue,
				RetrievalMode:   mode,
				RetrievalReason: reason,
			}
			out = append(out, scoredCandidate{cand: cand, combined: jac + h.Score})
			perAnchor++
		}
	}
	return out, nil
}

// embedOne embeds a single text for recall, returning nil when the embed fails.
// Callers gate it behind a single availability probe so the run does not re-probe
// per anchor.
func (s *Service) embedOne(ctx context.Context, text string) []float32 {
	vecs, err := s.embed.Embed(ctx, []string{text})
	if err != nil || len(vecs) != 1 {
		return nil
	}
	return vecs[0]
}

// --- Pair gates and helpers (pure) ---

// anchorLines returns the assertion-bearing or reversal-bearing lines in body,
// skipping headings and blank lines, in document order.
func anchorLines(body string) []temporal.Anchor {
	var out []temporal.Anchor
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if a, ok := temporal.AnchorOf(line); ok {
			out = append(out, a)
		}
	}
	return out
}

// bestOpposingLine finds the line in body that most shares the anchor's subject
// while carrying the opposite polarity (the polarity XOR gate). It returns the
// opposing anchor, the sorted shared terms, and the Jaccard, or ok=false when no
// line clears the shared-term floor.
func bestOpposingLine(body string, anchor temporal.Anchor, aTokens map[string]bool) (temporal.Anchor, []string, float64, bool) {
	best := -1.0
	var bestAnchor temporal.Anchor
	var bestShared []string
	ok := false
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		b, isAnchor := temporal.AnchorOf(line)
		if !isAnchor || b.Reversal == anchor.Reversal {
			continue // exactly one side may carry a reversal cue
		}
		jac, shared := jaccard(aTokens, contentTokens(line))
		if jac < contradictionJaccardFloor || len(shared) < contradictionMinSharedTerms {
			continue
		}
		if jac > best {
			best, bestAnchor, bestShared, ok = jac, b, shared, true
		}
	}
	return bestAnchor, bestShared, best, ok
}

// rankDedupeCap ranks candidates by combined score descending, collapses
// symmetric duplicates so an unordered note pair counts once, and hard-caps to
// limit. Ties break deterministically by note paths then lines.
func rankDedupeCap(in []scoredCandidate, limit int) []Contradiction {
	sort.SliceStable(in, func(i, j int) bool {
		if in[i].combined != in[j].combined {
			return in[i].combined > in[j].combined
		}
		if in[i].cand.NoteA != in[j].cand.NoteA {
			return in[i].cand.NoteA < in[j].cand.NoteA
		}
		if in[i].cand.NoteB != in[j].cand.NoteB {
			return in[i].cand.NoteB < in[j].cand.NoteB
		}
		return in[i].cand.LineA < in[j].cand.LineA
	})
	seen := map[string]bool{}
	out := make([]Contradiction, 0, limit)
	for _, sc := range in {
		key := unorderedPairKey(sc.cand.NoteA, sc.cand.NoteB)
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, sc.cand)
		if len(out) >= limit {
			break
		}
	}
	return out
}

// unorderedPairKey returns a symmetric key so (a, b) and (b, a) collapse.
func unorderedPairKey(a, b string) string {
	if a <= b {
		return a + "\x00" + b
	}
	return b + "\x00" + a
}

// contradictionExcluded reports whether rel is a benign non-source: a directory
// index or a template. These carry structural or boilerplate text and must never
// anchor or answer a contradiction.
func (s *Service) contradictionExcluded(rel string, directoryIndexPaths map[string]bool) bool {
	return directoryIndexPaths[rel] || isTemplateNote(rel)
}

// isTemplateNote reports whether rel lives under a templates directory.
func isTemplateNote(rel string) bool {
	for _, seg := range strings.Split(strings.ToLower(rel), "/") {
		if seg == "templates" || seg == "_templates" || seg == "template" {
			return true
		}
	}
	return false
}

// hasSupersedeFrontmatter reports whether a note participates in the sanctioned
// invalidate-not-delete mechanism (SPEC 12.3): a non-empty superseded_by or
// valid_to frontmatter field. Such a note is changing its mind on purpose, not
// contradicting another.
func hasSupersedeFrontmatter(n vault.Note) bool {
	for _, key := range []string{"superseded_by", "valid_to"} {
		v, ok := n.Frontmatter[key]
		if !ok || v == nil {
			continue
		}
		if str, isStr := v.(string); isStr && strings.TrimSpace(str) == "" {
			continue
		}
		return true
	}
	return false
}

// contradictionStopwords are function words plus the polarity lexicon's own cue
// words, excluded from shared-subject tokens so a shared "not" or "will" never
// counts as a shared subject.
var contradictionStopwords = map[string]bool{
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"to": true, "of": true, "in": true, "on": true, "for": true, "with": true,
	"as": true, "at": true, "by": true, "from": true, "into": true, "than": true,
	"then": true, "so": true, "up": true, "out": true, "off": true, "its": true,
	"it": true, "this": true, "that": true, "these": true, "those": true,
	"we": true, "our": true, "us": true, "you": true, "your": true, "i": true,
	"do": true, "does": true, "did": true, "has": true, "have": true, "had": true,
	"use": true, "used": true, "using": true, "will": true, "no": true, "not": true,
	"never": true, "longer": true, "must": true, "always": true, "default": true,
	"chose": true, "decided": true, "locked": true, "canonical": true,
	"isn": true, "aren": true, "won": true, "can": true, "don": true,
	"deprecated": true, "reverted": true, "cancelled": true, "abandoned": true,
	"dropped": true, "removed": true, "obsolete": true, "superseded": true,
	"replaced": true, "instead": true, "against": true, "changed": true, "mind": true,
}

// contentTokens returns the lowercased, de-stopworded content tokens of s, the
// subject terms a Jaccard overlap is measured over.
func contentTokens(s string) map[string]bool {
	out := map[string]bool{}
	for _, f := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	}) {
		if len(f) < 2 || contradictionStopwords[f] {
			continue
		}
		out[f] = true
	}
	return out
}

// jaccard returns the Jaccard similarity of two token sets and their sorted
// intersection, or zero and nil when either set is empty.
func jaccard(a, b map[string]bool) (float64, []string) {
	if len(a) == 0 || len(b) == 0 {
		return 0, nil
	}
	var inter []string
	for t := range a {
		if b[t] {
			inter = append(inter, t)
		}
	}
	if len(inter) == 0 {
		return 0, nil
	}
	union := len(a) + len(b) - len(inter)
	sort.Strings(inter)
	return float64(len(inter)) / float64(union), inter
}

// --- Rendering ---

// renderContradictions renders the result as markdown: a loud retrieval-mode
// line, then one review prompt per candidate carrying its evidence and the fixed
// candidate-not-verdict hedge.
func renderContradictions(res ContradictionsResult) string {
	var b strings.Builder
	b.WriteString("# Contradiction candidates\n\n")

	mode := res.RetrievalMode
	if res.RetrievalReason != "" {
		mode = fmt.Sprintf("%s - %s", res.RetrievalMode, res.RetrievalReason)
	}
	if len(res.Candidates) == 0 {
		fmt.Fprintf(&b, "No contradiction candidates (retrieval: %s).\n", mode)
		return b.String()
	}
	fmt.Fprintf(&b, "%s (retrieval: %s).\n\n", pluralCandidates(len(res.Candidates)), mode)
	for _, c := range res.Candidates {
		fmt.Fprintf(&b, "- Possible contradiction (review): `%s` says \"%s\" and `%s` says \"%s\". Shared terms: %s. Cue: %s. %s\n",
			c.NoteA, oneLineN(c.LineA, 160), c.NoteB, oneLineN(c.LineB, 160),
			strings.Join(c.SharedTerms, ", "), c.Cue, contradictionHedge)
	}
	return b.String()
}

// pluralCandidates renders the candidate count with correct pluralization.
func pluralCandidates(n int) string {
	if n == 1 {
		return "1 candidate"
	}
	return fmt.Sprintf("%d candidates", n)
}
