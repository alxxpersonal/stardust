package vault

import (
	"regexp"
	"strings"
)

// --- Chunking ---

// Chunk is an indexable section of a note. Sections split on markdown headings;
// oversized sections are hard-split into overlapping windows. The parent note's
// title and tags ride along so retrieval can return the whole note (small-to-big).
type Chunk struct {
	NotePath string
	Title    string
	Tags     string // space-joined, for the FTS tags column
	Heading  string
	Ord      int
	Body     string
	TokenEst int
}

const (
	maxChunkChars = 3200 // roughly 800 tokens
	overlapChars  = 480  // roughly 15 percent overlap on oversized sections
)

var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)

// Chunks splits a note into header-aware chunks carrying parent-note metadata.
func Chunks(n Note) []Chunk {
	tags := strings.Join(n.Tags, " ")

	type section struct{ heading, body string }
	var sections []section
	heading := ""
	var b strings.Builder
	flush := func() {
		body := strings.TrimSpace(b.String())
		if body != "" {
			sections = append(sections, section{heading, body})
		}
		b.Reset()
	}
	for _, line := range strings.Split(n.Body, "\n") {
		if m := headingRe.FindStringSubmatch(line); m != nil {
			flush()
			heading = strings.TrimSpace(m[2])
			continue
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	flush()

	var chunks []Chunk
	ord := 0
	for _, s := range sections {
		for _, piece := range splitOversize(s.body) {
			if piece == "" {
				continue
			}
			chunks = append(chunks, Chunk{
				NotePath: n.Path,
				Title:    n.Title,
				Tags:     tags,
				Heading:  s.heading,
				Ord:      ord,
				Body:     piece,
				TokenEst: len(piece) / 4,
			})
			ord++
		}
	}

	// a note with only a title and no body still deserves one searchable chunk
	if len(chunks) == 0 {
		chunks = append(chunks, Chunk{
			NotePath: n.Path,
			Title:    n.Title,
			Tags:     tags,
			Body:     n.Title,
			TokenEst: len(n.Title) / 4,
		})
	}
	return chunks
}

// splitOversize breaks body into overlapping fixed-size windows when it exceeds
// the chunk budget, otherwise returns it unchanged.
func splitOversize(body string) []string {
	r := []rune(body)
	if len(r) <= maxChunkChars {
		return []string{body}
	}
	step := maxChunkChars - overlapChars
	if step <= 0 {
		step = maxChunkChars
	}
	var out []string
	for start := 0; start < len(r); start += step {
		end := start + maxChunkChars
		if end > len(r) {
			end = len(r)
		}
		out = append(out, strings.TrimSpace(string(r[start:end])))
		if end == len(r) {
			break
		}
	}
	return out
}
