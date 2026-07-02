// Package temporal treats the git history as the event stream: what changed
// since a cursor, plus the commitments ("I'll do X", TODO) embedded in those
// changes. Git IS the change-feed, so there is no Kafka or CDC; a digest is just
// a diff since the last processed commit, grouped and summarized.
package temporal

import (
	"regexp"
	"strings"
)

// Commitment is an action-item line found in a note.
type Commitment struct {
	Path string `json:"path"`
	Line string `json:"line"`
}

var commitmentRe = regexp.MustCompile(`(?i)(\btodo\b|\bfixme\b|\bi'll\b|\bi will\b|\bneed to\b|\[ \])`)

// Commitments returns the action-item lines in body: TODO/FIXME, "I'll"/"I will",
// "need to", or an unchecked task box. Heading lines are skipped.
func Commitments(path, body string) []Commitment {
	var out []Commitment
	for _, line := range strings.Split(body, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if commitmentRe.MatchString(t) {
			out = append(out, Commitment{Path: path, Line: t})
		}
	}
	return out
}

// TopArea returns the first path segment of a slash path (its project or area),
// or "(root)" when there is none.
func TopArea(rel string) string {
	if i := strings.Index(rel, "/"); i >= 0 {
		return rel[:i]
	}
	return "(root)"
}

// --- Contradiction polarity lexicon ---

// Anchor is a line classified by the polarity lexicon as assertion-bearing or
// reversal-bearing. It is the deterministic signal that seeds a cross-note
// contradiction candidate: a decision on one side and its reversal on the other.
type Anchor struct {
	Line     string `json:"line"`     // the trimmed source line
	Cue      string `json:"cue"`      // the lexicon phrase that fired
	Reversal bool   `json:"reversal"` // true for a reversal cue, false for a plain assertion
}

// reversalCues mark a negation, deprecation, or reversal of a prior decision.
// Multi-word phrases lead so a phrase wins over a single-word substring when
// both match (for example "decided against" over the assertion "decided").
var reversalCues = []string{
	"no longer", "rolled back", "replaced by", "instead of",
	"decided against", "changed our mind",
	"not", "never", "isn't", "aren't", "won't", "can't", "don't",
	"deprecated", "reverted", "cancelled", "abandoned", "dropped",
	"removed", "obsolete", "superseded",
}

// assertionMarkers mark a decision or a firm claim without a reversal.
var assertionMarkers = []string{
	"we will", "decided", "chose", "locked", "canonical", "must", "always", "default",
}

// cueMatcher pairs a lexicon phrase with its word-boundary matcher.
type cueMatcher struct {
	phrase string
	re     *regexp.Regexp
}

var (
	reversalMatchers  = compileCues(reversalCues)
	assertionMatchers = compileCues(assertionMarkers)
)

// compileCues builds a word-boundary matcher for each phrase, preserving order.
func compileCues(cues []string) []cueMatcher {
	out := make([]cueMatcher, len(cues))
	for i, c := range cues {
		out[i] = cueMatcher{phrase: c, re: regexp.MustCompile(`(?i)\b` + regexp.QuoteMeta(c) + `\b`)}
	}
	return out
}

// firstCue returns the first phrase whose matcher fires in s, in list order.
func firstCue(matchers []cueMatcher, s string) (string, bool) {
	for _, m := range matchers {
		if m.re.MatchString(s) {
			return m.phrase, true
		}
	}
	return "", false
}

// AnchorOf classifies line against the polarity lexicon and reports whether it
// carries a decision or a reversal. ok is false when the line matches neither
// set. A reversal cue takes precedence over an assertion marker, because the
// reversal of a decision is the discriminating signal for a contradiction pair.
func AnchorOf(line string) (Anchor, bool) {
	t := strings.TrimSpace(line)
	if t == "" {
		return Anchor{}, false
	}
	if cue, ok := firstCue(reversalMatchers, t); ok {
		return Anchor{Line: t, Cue: cue, Reversal: true}, true
	}
	if cue, ok := firstCue(assertionMatchers, t); ok {
		return Anchor{Line: t, Cue: cue, Reversal: false}, true
	}
	return Anchor{}, false
}
