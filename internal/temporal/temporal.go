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
