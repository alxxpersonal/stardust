package vault

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractEdges(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "store"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "store", "daemon.go"), []byte("package store\n"), 0o644))

	note := Note{
		Path:        "docs/adr/0001-x.md",
		Frontmatter: map[string]any{"related": []any{"docs/specs/x.md"}},
		Body:        "see [[Foo Bar]] and the daemon in internal/store/daemon.go and missing/dir/none.go",
	}
	edges := ExtractEdges(root, note)

	require.ElementsMatch(t, []Edge{
		{Target: "foo bar", Kind: "wikilink"},
		{Target: "docs/specs/x.md", Kind: "related"},
		{Target: "internal/store/daemon.go", Kind: "inline-path"},
	}, edges)
}

func TestContentHashStable(t *testing.T) {
	a := ContentHash([]byte("hello"))
	require.Equal(t, a, ContentHash([]byte("hello")))
	require.NotEqual(t, a, ContentHash([]byte("world")))
}

func TestExtractLinks(t *testing.T) {
	body := "see [[Foo Bar]] and [[notes/Baz|alias]] then [[Foo Bar]] dup and [[Qux#heading]]"
	require.ElementsMatch(t, []string{"foo bar", "baz", "qux"}, ExtractLinks(body))
}

func TestExtractWikilinkCandidates(t *testing.T) {
	body := "see [[notes/Baz|Alias]] and [[Install|Install Guide]] and [[https://example.com]]"
	require.Equal(t, [][]string{
		{"baz", "alias"},
		{"install", "install guide"},
	}, ExtractWikilinkCandidates(body))
}

func TestExtractLinksIgnoresMarkdownCode(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{
			name: "inline code",
			body: "format example `[[Hidden]]` only",
			want: nil,
		},
		{
			name: "backtick fence",
			body: "```markdown\n[[Hidden]]\n```\n",
			want: nil,
		},
		{
			name: "tilde fence",
			body: "~~~markdown\n[[Hidden]]\n~~~\n",
			want: nil,
		},
		{
			name: "prose",
			body: "see [[Visible]] in prose",
			want: []string{"visible"},
		},
		{
			name: "escaped backticks",
			body: "escaped \\`[[Visible]]\\` stays prose",
			want: []string{"visible"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.ElementsMatch(t, tt.want, ExtractLinks(tt.body))
		})
	}
}

func TestExtractEdgesIgnoresCodeMaskedReferences(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "internal", "store"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(root, "internal", "store", "daemon.go"), []byte("package store\n"), 0o644))

	note := Note{
		Path: "docs/adr/0001-x.md",
		Body: "see `[[Hidden]]` and `internal/store/daemon.go`\n\n```\n[[Fenced]]\ninternal/store/daemon.go\n```\n",
	}

	require.Empty(t, ExtractEdges(root, note))
}

func TestNormalizeLink(t *testing.T) {
	require.Equal(t, "foo", NormalizeLink("Foo.md"))
	require.Equal(t, "bar", NormalizeLink("notes/Bar"))
	require.Equal(t, "baz", NormalizeLink("  BAZ  "))
	require.Equal(t, "page name", GitHubWikiDisplayAlias("page-name"))
	require.Equal(t, "specs/page name", GitHubWikiDisplayAlias("specs/page-name"))
}

func TestCollectionKey(t *testing.T) {
	// cross-collection slugs key to distinct nodes.
	require.Equal(t, "specs/2026-06-26-0001-game", CollectionKey("docs/specs/2026-06-26-0001-game.md"))
	require.Equal(t, "plans/2026-06-26-0001-game", CollectionKey("docs/plans/2026-06-26-0001-game.md"))
	require.NotEqual(t, CollectionKey("docs/specs/x.md"), CollectionKey("docs/plans/x.md"))
	// a same-collection slug in two subdirs collides intentionally (a true dup).
	require.Equal(t, CollectionKey("docs/specs/x.md"), CollectionKey("docs/specs/archive/x.md"))
	// non-docs paths keep the legacy basename key for backward compatibility.
	require.Equal(t, "x", CollectionKey("notes/x.md"))
	require.Equal(t, "x", CollectionKey("x.md"))
}

func TestExtractEdgesQualifiedWikilink(t *testing.T) {
	note := Note{
		Path: "docs/adr/0001-x.md",
		Body: "see [[specs/game-state-backend]] and [[spec:game-state-backend]] and [[Plain Link]] and [[notes/Bar]]",
	}
	edges := ExtractEdges(t.TempDir(), note)
	// both qualifier forms normalize to the same path-style key (deduped to one edge).
	require.Contains(t, edges, Edge{Target: "specs/game-state-backend", Kind: "wikilink"})
	// an unqualified link and an ordinary path link keep legacy basename behavior.
	require.Contains(t, edges, Edge{Target: "plain link", Kind: "wikilink"})
	require.Contains(t, edges, Edge{Target: "bar", Kind: "wikilink"})
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

func TestScanSkipsGeneratedRegistry(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "specs"), 0o755))
	// the generated docs registry at the registry output path, carrying the marker.
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "INDEX.md"),
		[]byte("# Docs Index\n\nGenerated by `stardust registry`. Do not edit by hand.\n"), 0o644))
	// a normal note under a collection folder.
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "a.md"),
		[]byte("# Spec A\n\nbody\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "README.md"),
		[]byte("# Readme\n"), 0o644))
	// a hand-written INDEX.md without the marker stays indexed.
	require.NoError(t, os.WriteFile(filepath.Join(root, "docs", "specs", "INDEX.md"),
		[]byte("# Specs hand index\n\nnot generated\n"), 0o644))

	paths, err := Scan(root, nil)
	require.NoError(t, err)
	require.Contains(t, paths, "docs/specs/a.md")
	require.Contains(t, paths, "README.md")
	require.Contains(t, paths, "docs/specs/INDEX.md") // no marker, kept
	require.NotContains(t, paths, "docs/INDEX.md")    // generated registry, skipped
}

// TestScanIncludesMarkdownExtension pins that Scan matches the .markdown
// extension the renderer's trimMarkdownExtension switch also accepts, so a note
// saved as .markdown is indexed the same as a .md note. Matching is
// case-insensitive, and non-markdown files stay excluded.
func TestScanIncludesMarkdownExtension(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "note.md"), []byte("# md\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "long.markdown"), []byte("# markdown\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "shout.MARKDOWN"), []byte("# upper\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(root, "notes.txt"), []byte("plain\n"), 0o644))

	paths, err := Scan(root, nil)
	require.NoError(t, err)
	require.Contains(t, paths, "note.md")
	require.Contains(t, paths, "long.markdown")
	require.Contains(t, paths, "shout.MARKDOWN") // case-insensitive extension match
	require.NotContains(t, paths, "notes.txt")   // non-markdown, excluded
}
