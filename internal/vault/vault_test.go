package vault

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestContentHashStable(t *testing.T) {
	a := ContentHash([]byte("hello"))
	require.Equal(t, a, ContentHash([]byte("hello")))
	require.NotEqual(t, a, ContentHash([]byte("world")))
}

func TestExtractLinks(t *testing.T) {
	body := "see [[Foo Bar]] and [[notes/Baz|alias]] then [[Foo Bar]] dup and [[Qux#heading]]"
	require.ElementsMatch(t, []string{"foo bar", "baz", "qux"}, ExtractLinks(body))
}

func TestNormalizeLink(t *testing.T) {
	require.Equal(t, "foo", NormalizeLink("Foo.md"))
	require.Equal(t, "bar", NormalizeLink("notes/Bar"))
	require.Equal(t, "baz", NormalizeLink("  BAZ  "))
}

func TestChunksHeaderSplit(t *testing.T) {
	n := Note{
		Path:  "x.md",
		Title: "X",
		Tags:  []string{"t1"},
		Body:  "intro text\n\n## Section A\nalpha body\n\n## Section B\nbeta body",
	}
	chunks := Chunks(n)
	require.Len(t, chunks, 3)
	require.Equal(t, "", chunks[0].Heading)
	require.Equal(t, "Section A", chunks[1].Heading)
	require.Equal(t, "Section B", chunks[2].Heading)
	for _, c := range chunks {
		require.Equal(t, "x.md", c.NotePath)
		require.Equal(t, "t1", c.Tags)
	}
}

func TestChunksTitleOnlyFallback(t *testing.T) {
	chunks := Chunks(Note{Path: "e.md", Title: "Empty", Body: "   \n  "})
	require.Len(t, chunks, 1)
	require.Equal(t, "Empty", chunks[0].Body)
}
