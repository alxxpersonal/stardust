package agentsync

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Sentinel markers that delimit stardust's managed rules block inside a shared
// markdown memory file (CLAUDE.md, AGENTS.md, GEMINI.md). They mirror the hook
// block contract of ADR 0008 (compose, never clobber) with HTML-comment markers
// so they stay invisible when the markdown is rendered. inject replaces what is
// between them, strip removes from rulesBlockStart through rulesBlockEnd, and
// lines outside them are never touched.
const (
	rulesBlockStart = "<!-- >>> stardust rules >>> (managed block, do not edit) -->"
	rulesBlockEnd   = "<!-- <<< stardust rules <<< -->"
)

// rulesAdapter renders the canonical rules body into the block one tool expects.
// fileName is that tool's conventional memory-file name (the seam a later
// per-tool wrapper keys off). render strips the source frontmatter and applies
// any tool-specific shaping; the three converge today, but each owns its own
// render so a future divergence lands for one tool without touching the others.
type rulesAdapter struct {
	fileName string
	render   func(body string) string
}

// rulesAdapters maps every supported tool to its rules adapter.
var rulesAdapters = map[Tool]rulesAdapter{
	ToolClaude: {fileName: "CLAUDE.md", render: renderRulesBody},
	ToolCodex:  {fileName: "AGENTS.md", render: renderRulesBody},
	ToolGemini: {fileName: "GEMINI.md", render: renderRulesBody},
}

// renderRulesBody strips the source YAML frontmatter and trims surrounding
// whitespace, leaving the markdown body that belongs inside the managed block.
func renderRulesBody(body string) string {
	return strings.TrimSpace(stripFrontmatter(body))
}

// renderRules renders the canonical body for one tool via its adapter.
func renderRules(tool Tool, body string) (string, error) {
	adapter, ok := rulesAdapters[tool]
	if !ok {
		return "", fmt.Errorf("no rules adapter for tool %q", tool)
	}
	return adapter.render(body), nil
}

// stripFrontmatter returns text with a leading YAML frontmatter block removed.
// It mirrors the marker handling of parseFrontmatter: text without frontmatter,
// or with an unterminated fence, is returned unchanged.
func stripFrontmatter(text string) string {
	if !strings.HasPrefix(text, "---\n") && !strings.HasPrefix(text, "---\r\n") {
		return text
	}
	rest := text[4:]
	if strings.HasPrefix(text, "---\r\n") {
		rest = text[5:]
	}
	for _, marker := range []string{"\n---\n", "\n---\r\n", "\r\n---\r\n", "\r\n---\n"} {
		if idx := strings.Index(rest, marker); idx >= 0 {
			return rest[idx+len(marker):]
		}
	}
	return text
}

// injectRulesBlock adds stardust's managed rules block (body, wrapped in the
// sentinel markers) to the markdown file at path with a single read-modify-write.
// A missing file is created (with its parent). An existing block is replaced in
// place, so repeated calls are idempotent and never duplicate the block. Every
// line outside the markers is preserved untouched, and the block is appended
// after any user content.
func injectRulesBlock(path, body string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("read rules %s: %w", path, err)
		}
		existing = nil
	}

	base := stripExistingRulesBlock(string(existing))
	block := rulesBlockStart + "\n" + ensureRulesTrailingNewline(body) + rulesBlockEnd + "\n"

	var next string
	if strings.TrimSpace(base) == "" {
		next = block
	} else {
		next = ensureRulesTrailingNewline(base) + "\n" + block
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create rules parent %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(next), 0o644); err != nil {
		return fmt.Errorf("write rules %s: %w", path, err)
	}
	return nil
}

// stripRulesBlock removes stardust's managed rules block from the markdown file
// at path, collapsing the blank lines left where the block was. A file without
// the block and a missing file are both left untouched and reported as success.
func stripRulesBlock(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read rules %s: %w", path, err)
	}

	body := string(existing)
	stripped := stripExistingRulesBlock(body)
	if stripped == body {
		return nil
	}
	if err := os.WriteFile(path, []byte(stripped), 0o644); err != nil {
		return fmt.Errorf("write rules %s: %w", path, err)
	}
	return nil
}

// extractRulesBlock returns the body between the sentinel markers and whether a
// block was found. Marker matching trims each line, so trailing carriage returns
// on CRLF files do not defeat it.
func extractRulesBlock(body string) (string, bool) {
	lines := strings.Split(body, "\n")
	inner := make([]string, 0, len(lines))
	inBlock := false
	found := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == rulesBlockStart:
			inBlock = true
			found = true
		case inBlock && trimmed == rulesBlockEnd:
			inBlock = false
		case inBlock:
			inner = append(inner, line)
		}
	}
	return strings.Join(inner, "\n"), found
}

// stripExistingRulesBlock returns body with stardust's managed rules block
// removed. It drops every line from rulesBlockStart through rulesBlockEnd
// (inclusive) and collapses the resulting run of blank lines into a single one,
// so the file does not accumulate gaps across sync cycles. Body with no block is
// returned unchanged.
func stripExistingRulesBlock(body string) string {
	lines := strings.Split(body, "\n")
	out := make([]string, 0, len(lines))
	inBlock := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == rulesBlockStart:
			inBlock = true
		case inBlock && trimmed == rulesBlockEnd:
			inBlock = false
		case inBlock:
			// drop lines inside the block
		default:
			out = append(out, line)
		}
	}
	return collapseRulesBlankRuns(strings.Join(out, "\n"))
}

// collapseRulesBlankRuns squeezes any run of two or more blank lines into a
// single blank line, leaving the rest of the text intact.
func collapseRulesBlankRuns(s string) string {
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// ensureRulesTrailingNewline returns s with exactly one trailing newline, so
// blocks append on their own line. An empty string stays empty.
func ensureRulesTrailingNewline(s string) string {
	if s == "" {
		return ""
	}
	return strings.TrimRight(s, "\n") + "\n"
}
