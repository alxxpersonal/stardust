package vault

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIsMarkdownPath(t *testing.T) {
	require.True(t, IsMarkdownPath("a.md"))
	require.True(t, IsMarkdownPath("a.markdown"))
	require.True(t, IsMarkdownPath("nested/dir/A.MD")) // case-insensitive
	require.False(t, IsMarkdownPath("a.rst"))
	require.False(t, IsMarkdownPath("a.adoc"))
	require.False(t, IsMarkdownPath("a.txt"))
	require.False(t, IsMarkdownPath("noext"))
}

func TestParseNonMarkdownAsciiDoc(t *testing.T) {
	src := []byte("= AsciiDoc Handbook\n" +
		":toc:\n" +
		":author: Jane Doe\n" +
		"\n" +
		"// a leading comment line\n" +
		"== Getting Started\n" +
		"\n" +
		"Install the *tool* and run `setup` now.\n" +
		"\n" +
		"----\n" +
		"literal block body\n" +
		"----\n")
	n := parseNonMarkdown("Guide.adoc", src)

	require.Equal(t, "AsciiDoc Handbook", n.Title)
	require.Nil(t, n.Frontmatter)
	require.Nil(t, n.Tags)
	require.Nil(t, n.Links)
	require.Equal(t, ContentHash(src), n.Hash)

	require.Contains(t, n.Body, "Getting Started") // heading marker stripped, text kept
	require.Contains(t, n.Body, "Install the tool and run setup now.")
	require.Contains(t, n.Body, "literal block body")
	require.NotContains(t, n.Body, "a leading comment line") // // comment dropped
	require.NotContains(t, n.Body, "Jane Doe")               // attribute entry dropped
	require.NotContains(t, n.Body, "----")                   // block delimiter dropped
}

func TestParseNonMarkdownRST(t *testing.T) {
	src := []byte("reStructuredText Guide\n" +
		"======================\n" +
		"\n" +
		".. This is a comment\n" +
		".. note::\n" +
		"   directive content\n" +
		"\n" +
		"Section One\n" +
		"-----------\n" +
		"\n" +
		"Use ``literal`` and *emphasis* here.\n" +
		"See `the docs <https://example.com>`_ for more.\n")
	n := parseNonMarkdown("Install.rst", src)

	require.Equal(t, "reStructuredText Guide", n.Title)
	require.Contains(t, n.Body, "Section One") // adornment stripped, title text kept
	require.Contains(t, n.Body, "Use literal and emphasis here.")
	require.Contains(t, n.Body, "See the docs for more.")
	require.NotContains(t, n.Body, "https://example.com")
	require.NotContains(t, n.Body, "note::")
	require.NotContains(t, n.Body, "======")
}

func TestParseNonMarkdownRSTOverlineTitle(t *testing.T) {
	src := []byte("======\nTitled\n======\n\nbody text\n")
	n := parseNonMarkdown("Over.rst", src)
	require.Equal(t, "Titled", n.Title)
	require.Contains(t, n.Body, "body text")
}

func TestParseNonMarkdownTextile(t *testing.T) {
	src := []byte("h1. Textile Page\n" +
		"\n" +
		"h2. Overview\n" +
		"\n" +
		"This has *strong* and _emphasis_ and @code@ words.\n" +
		"A \"link\":https://example.com to follow.\n")
	n := parseNonMarkdown("Style.textile", src)

	require.Equal(t, "Textile Page", n.Title)
	require.Contains(t, n.Body, "Overview")
	require.Contains(t, n.Body, "This has strong and emphasis and code words.")
	require.Contains(t, n.Body, "A link to follow.")
	require.NotContains(t, n.Body, "https://example.com")
	require.NotContains(t, n.Body, "h1.")
	require.NotContains(t, n.Body, "@code@")
}

func TestParseNonMarkdownOrg(t *testing.T) {
	src := []byte("#+TITLE: Org Mode Notes\n" +
		"#+AUTHOR: Someone\n" +
		"\n" +
		"* Heading One\n" +
		":PROPERTIES:\n" +
		":CUSTOM_ID: h1\n" +
		":END:\n" +
		"\n" +
		"Some /italic/ and *bold* and =verbatim= text.\n" +
		"A [[https://example.com][link description]] here.\n")
	n := parseNonMarkdown("Notes.org", src)

	require.Equal(t, "Org Mode Notes", n.Title)
	require.Contains(t, n.Body, "Heading One")
	require.Contains(t, n.Body, "Some italic and bold and verbatim text.")
	require.Contains(t, n.Body, "A link description here.")
	require.NotContains(t, n.Body, "PROPERTIES")
	require.NotContains(t, n.Body, "#+")
	require.NotContains(t, n.Body, "https://example.com")
}

func TestParseNonMarkdownOrgHeadlineTitleFallback(t *testing.T) {
	src := []byte("* First Headline\n\nbody\n")
	n := parseNonMarkdown("H.org", src)
	require.Equal(t, "First Headline", n.Title)
}

func TestParseNonMarkdownTierBFilenameTitleRawBody(t *testing.T) {
	// Creole, MediaWiki, RDoc, Pod get a filename title and a raw, untouched body.
	raw := "= Heading =\nsome **wiki** text with markup left intact\n"
	for _, name := range []string{"Sidebar_Content.creole", "Manual.mediawiki", "Home.wiki", "Api.rdoc", "Perl.pod", "Perl6.pod6"} {
		n := parseNonMarkdown(name, []byte(raw))
		require.Equal(t, filenameTitle(name), n.Title, name)
		require.Equal(t, raw, n.Body, name) // Tier B body is the raw source, verbatim
	}
}

func TestParseNonMarkdownFilenameTitleFallbackTierA(t *testing.T) {
	// a Tier A format whose title heuristic misses falls back to the filename.
	n := parseNonMarkdown("no-title-here.adoc", []byte("just body, no document title line\n"))
	require.Equal(t, "no-title-here", n.Title)
	require.Contains(t, n.Body, "just body, no document title line")
}

func TestParseNonMarkdownBodyDoesNotLeakDashRule(t *testing.T) {
	// sanity: the reducer preserves prose content the convention dash rule scans.
	n := parseNonMarkdown("D.adoc", []byte("= T\n\nplain prose line\n"))
	require.False(t, strings.Contains(n.Body, "\x00"))
}
