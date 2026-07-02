package vault

// Non-markdown wiki page indexing. GitHub wikis render nine markup formats by
// file extension; this file extracts a title and a cheap plain-text body from
// the non-markdown formats using pure-Go line and regex rules, no parser. The
// markdown Parse path is untouched. Tier A formats (AsciiDoc, reStructuredText,
// Textile, Org) get a per-format title heuristic and a light body reducer that
// strips heading markers and drops comment, directive, and delimiter lines. Tier
// B formats (Creole, MediaWiki, RDoc, Pod) get a filename title and the raw
// body; SQLite FTS5 tokenization handles residual markup. Every title miss falls
// back to the filename, which is GitHub's authoritative wiki page title.

import (
	"path/filepath"
	"regexp"
	"strings"
	"unicode"
)

// markdownExts are the extensions Stardust has always treated as markdown.
var markdownExts = map[string]bool{
	".md":       true,
	".markdown": true,
}

// nonMarkdownPageExts are the github/markup non-markdown wiki formats Stardust
// indexes as title-plus-plain-text. .asc is deliberately excluded: it collides
// with ASCII-armored PGP key and signature files, which would index as garbage.
var nonMarkdownPageExts = map[string]bool{
	".adoc":      true,
	".asciidoc":  true,
	".rst":       true,
	".rest":      true,
	".textile":   true,
	".org":       true,
	".creole":    true,
	".mediawiki": true,
	".wiki":      true,
	".rdoc":      true,
	".pod":       true,
	".pod6":      true,
}

func extOf(path string) string { return strings.ToLower(filepath.Ext(path)) }

func isMarkdownExt(ext string) bool        { return markdownExts[ext] }
func isNonMarkdownPageExt(ext string) bool { return nonMarkdownPageExts[ext] }

// isSupportedPageExt reports whether ext is a markdown or a supported
// non-markdown wiki page extension. Scan and trimMarkdownExtension key off it.
func isSupportedPageExt(ext string) bool {
	return isMarkdownExt(ext) || isNonMarkdownPageExt(ext)
}

// IsMarkdownPath reports whether path has a markdown extension (.md or
// .markdown). The convention checker uses it to keep the docs-convention block
// markdown-only while the forbidden-dash rule stays global, and vault uses it to
// short-circuit edge and drift extraction for non-markdown pages.
func IsMarkdownPath(path string) bool { return isMarkdownExt(extOf(path)) }

// --- Per-format title heuristics + body reducers ---

var (
	asciidocTitleRe   = regexp.MustCompile(`^=[ \t]+(\S.*?)[ \t]*$`)
	asciidocHeadingRe = regexp.MustCompile(`^[ \t]*=+[ \t]+`)
	asciidocAttrRe    = regexp.MustCompile(`^:[^:\s]+:`)
	asciidocDelimRe   = regexp.MustCompile(`^(-{4,}|={4,}|\.{4,}|\*{4,}|\+{4,}|_{4,}|\|={3,}|--)$`)

	rstDirectiveRe = regexp.MustCompile(`^[ \t]*\.\.(\s|$)`)
	rstLinkRe      = regexp.MustCompile("`([^`<]+?)[ \\t]*<[^>]*>`_")
	rstRoleRe      = regexp.MustCompile(`:[A-Za-z][A-Za-z0-9_+.-]*:`)

	textileTitleRe = regexp.MustCompile(`^h[1-6]\.[ \t]+(\S.*?)[ \t]*$`)
	textileSigRe   = regexp.MustCompile(`^(?:h[1-6]|bq|p)\.[ \t]+`)
	textileLinkRe  = regexp.MustCompile(`"([^"]+)":[^\s]+`)

	orgTitleRe     = regexp.MustCompile(`(?i)^#\+title:[ \t]*(\S.*?)[ \t]*$`)
	orgHeadlineRe  = regexp.MustCompile(`^\*+[ \t]+(\S.*?)[ \t]*$`)
	orgKeywordRe   = regexp.MustCompile(`^[ \t]*#\+`)
	orgDrawerRe    = regexp.MustCompile(`^[ \t]*:[A-Za-z][A-Za-z0-9_-]*:`)
	orgHeadStripRe = regexp.MustCompile(`^\*+[ \t]+`)
	orgLinkDescRe  = regexp.MustCompile(`\[\[[^\]]*\]\[([^\]]*)\]\]`)
	orgLinkBareRe  = regexp.MustCompile(`\[\[([^\]]*)\]\]`)
)

// parseNonMarkdown builds a Note for a supported non-markdown wiki page. It sets
// a title (Tier A heuristic or filename fallback), a plain-text body (Tier A
// reduced or Tier B raw), the content hash, and leaves Frontmatter, Tags, and
// Links nil: these formats carry no YAML frontmatter, and edge and tag
// extraction assume markdown syntax.
func parseNonMarkdown(rel string, raw []byte) Note {
	n := Note{Path: filepath.ToSlash(rel), Hash: ContentHash(raw)}
	source := string(raw)
	switch extOf(rel) {
	case ".adoc", ".asciidoc":
		n.Title = firstAsciiDocTitle(source)
		n.Body = reduceAsciiDoc(source)
	case ".rst", ".rest":
		n.Title = firstRSTTitle(source)
		n.Body = reduceRST(source)
	case ".textile":
		n.Title = firstTextileTitle(source)
		n.Body = reduceTextile(source)
	case ".org":
		n.Title = firstOrgTitle(source)
		n.Body = reduceOrg(source)
	default: // Tier B: creole, mediawiki, wiki, rdoc, pod, pod6
		n.Body = source
	}
	if n.Title == "" {
		n.Title = filenameTitle(rel)
	}
	return n
}

// filenameTitle returns the de-extensioned base name, GitHub's authoritative
// wiki page title and the universal title fallback.
func filenameTitle(rel string) string {
	base := filepath.Base(filepath.ToSlash(rel))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// --- AsciiDoc ---

func firstAsciiDocTitle(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if m := asciidocTitleRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func reduceAsciiDoc(source string) string {
	var b strings.Builder
	for _, line := range strings.Split(source, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") || asciidocAttrRe.MatchString(trimmed) || asciidocDelimRe.MatchString(trimmed) {
			continue
		}
		line = asciidocHeadingRe.ReplaceAllString(line, "")
		line = stripChars(line, "*`")
		line = underscoreWrap.collapse(line)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// --- reStructuredText ---

func firstRSTTitle(source string) string {
	lines := strings.Split(source, "\n")
	for i := 0; i+1 < len(lines); i++ {
		text := strings.TrimSpace(lines[i])
		if text == "" || rstAdornmentLen(lines[i]) > 0 {
			continue
		}
		if rstAdornmentLen(lines[i+1]) >= len([]rune(text)) {
			return text
		}
	}
	return ""
}

func reduceRST(source string) string {
	var b strings.Builder
	for _, line := range strings.Split(source, "\n") {
		if rstDirectiveRe.MatchString(line) || rstAdornmentLen(line) > 0 {
			continue
		}
		line = rstLinkRe.ReplaceAllString(line, "$1")
		line = rstRoleRe.ReplaceAllString(line, "")
		line = stripChars(line, "*`")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// rstAdornmentLen returns the rune length of an RST adornment line (a run of one
// repeated punctuation character), or 0 when line is not an adornment.
func rstAdornmentLen(line string) int {
	t := strings.TrimRight(line, " \t")
	if len(t) < 2 {
		return 0
	}
	first := rune(t[0])
	if !isRSTAdornmentChar(first) {
		return 0
	}
	n := 0
	for _, r := range t {
		if r != first {
			return 0
		}
		n++
	}
	return n
}

// isRSTAdornmentChar reports whether r is an ASCII punctuation character usable
// as an RST title adornment (any printable ASCII that is not a letter or digit).
func isRSTAdornmentChar(r rune) bool {
	return r > 0x20 && r < 0x7f && !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

// --- Textile ---

func firstTextileTitle(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if m := textileTitleRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func reduceTextile(source string) string {
	var b strings.Builder
	for _, line := range strings.Split(source, "\n") {
		line = textileSigRe.ReplaceAllString(line, "")
		line = textileLinkRe.ReplaceAllString(line, "$1")
		line = stripChars(line, "*@")
		line = underscoreWrap.collapse(line)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// --- Org ---

func firstOrgTitle(source string) string {
	for _, line := range strings.Split(source, "\n") {
		if m := orgTitleRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	for _, line := range strings.Split(source, "\n") {
		if m := orgHeadlineRe.FindStringSubmatch(line); m != nil {
			return strings.TrimSpace(m[1])
		}
	}
	return ""
}

func reduceOrg(source string) string {
	var b strings.Builder
	for _, line := range strings.Split(source, "\n") {
		if orgKeywordRe.MatchString(line) || orgDrawerRe.MatchString(line) {
			continue
		}
		line = orgHeadStripRe.ReplaceAllString(line, "")
		line = orgLinkDescRe.ReplaceAllString(line, "$1")
		line = orgLinkBareRe.ReplaceAllString(line, "$1")
		line = stripChars(line, "*~")
		line = orgWrap.collapse(line)
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// --- Inline emphasis reduction ---

// stripChars removes every rune in chars from s. Used for emphasis markers that
// never occur inside prose words (`*`, backtick, `@`, `~`), so plain removal is
// safe and cheap.
func stripChars(s, chars string) string {
	return strings.Map(func(r rune) rune {
		if strings.ContainsRune(chars, r) {
			return -1
		}
		return r
	}, s)
}

// wrapCollapser removes emphasis marker characters that sit at a word boundary,
// so `/italic/` loses its slashes but `src/main` and `a=b` keep their markers.
// It handles the word-risky emphasis chars (`_`, `/`, `=`) that plain removal
// would corrupt inside identifiers and paths.
type wrapCollapser struct {
	open  *regexp.Regexp
	close *regexp.Regexp
}

func newWrapCollapser(markers string) wrapCollapser {
	q := regexp.QuoteMeta(markers)
	return wrapCollapser{
		open:  regexp.MustCompile(`(^|[\s(\[])[` + q + `]+(\S)`),
		close: regexp.MustCompile(`(\S)[` + q + `]+([\s).,;:!?\]]|$)`),
	}
}

func (w wrapCollapser) collapse(s string) string {
	s = w.open.ReplaceAllString(s, "$1$2")
	s = w.close.ReplaceAllString(s, "$1$2")
	return s
}

var (
	underscoreWrap = newWrapCollapser("_")
	orgWrap        = newWrapCollapser("/=_")
)
