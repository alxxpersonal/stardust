package render

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// ansiSeq matches CSI color/style escapes and OSC hyperlink escapes so tests can
// assert on the plain text content glamour styles word-by-word.
var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]|\x1b\\][^\x07]*\x07")

func stripANSI(s string) string { return ansiSeq.ReplaceAllString(s, "") }

// TestGlamourRenderThemesAllElements proves the cosmic markdown style renders the
// full set of block elements (heading, fenced code, inline code, list, link,
// blockquote, table) with ANSI styling, mirroring the exo-jobs style's coverage.
func TestGlamourRenderThemesAllElements(t *testing.T) {
	md := strings.Join([]string{
		"# Heading One",
		"## Heading Two",
		"Body **strong** and *emph* with `inline code` and a [link](https://x.dev).",
		"",
		"- first item",
		"- second item",
		"",
		"> a quoted line",
		"",
		"```go",
		"func main() { println(\"hi\") }",
		"```",
		"",
		"| Col A | Col B |",
		"|-------|-------|",
		"| a1    | b1    |",
	}, "\n")

	out := GlamourRender(md, 80)

	// the style actually emits ANSI escapes (it is themed, not raw passthrough).
	require.Contains(t, out, "\x1b[", "themed markdown must carry ANSI escapes")

	// content survives rendering (compared on the ANSI-stripped text since
	// glamour styles word-by-word).
	plain := stripANSI(out)
	for _, want := range []string{"Heading One", "Heading Two", "inline code", "link", "first item", "quoted line", "func main", "Col A", "a1"} {
		require.Contains(t, plain, want, "rendered markdown should contain %q", want)
	}
	// the cosmic list bullet and blockquote rail are themed in.
	require.Contains(t, out, "✧", "list items use the cosmic bullet")
	require.Contains(t, out, "│", "blockquote and table use a rail separator")
}

// TestGlamourRenderFallsBackOnTinyWidth proves the renderer clamps a tiny width
// instead of failing, so callers always get usable output.
func TestGlamourRenderFallsBackOnTinyWidth(t *testing.T) {
	out := GlamourRender("# Title\n\nbody", 1)
	require.Contains(t, out, "Title")
}
