package graph_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/vault"
)

func TestTopPageRankRanksHubFirst(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	// hub.md is the link sink: three leaves point at it, it points back at one.
	write("hub.md", "---\ntitle: Hub\n---\n# Hub\nsee [[a]]")
	write("a.md", "---\ntitle: A\n---\n# A\nsee [[hub]]")
	write("b.md", "---\ntitle: B\n---\n# B\nsee [[hub]]")
	write("c.md", "---\ntitle: C\n---\n# C\nsee [[hub]]")

	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	top := g.TopPageRank(10)
	require.Len(t, top, 4)
	require.Equal(t, "hub.md", top[0].Path) // most central node ranks first
	require.Greater(t, top[0].Score, top[len(top)-1].Score)

	// scores are normalized PageRank, summing to roughly 1.
	var sum float64
	for _, e := range top {
		sum += e.Score
	}
	require.InDelta(t, 1.0, sum, 1e-6)
}

func TestRelatedAndInlineEdges(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		require.NoError(t, os.MkdirAll(filepath.Dir(filepath.Join(root, name)), 0o755))
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte(content), 0o644))
	}
	// a code file that a.md references in prose -> a code reference, not a graph node.
	write("internal/store/daemon.go", "package store\n")
	// a.md reaches b.md only through related: (no wikilink), wikilinks c.md, references daemon.go.
	write("a.md", "---\ntitle: A\nrelated: [\"sub/b.md\"]\n---\n# A\nsee [[c]] and internal/store/daemon.go")
	write("sub/b.md", "---\ntitle: B\n---\n# B\n")
	write("c.md", "---\ntitle: C\n---\n# C\n")

	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	// the related-only target is not an orphan and the code path is not a node.
	require.NotContains(t, g.Orphans(), "sub/b.md")
	_, daemonIsNode := g.Nodes[vault.NormalizeLink("internal/store/daemon.go")]
	require.False(t, daemonIsNode)

	// personalized PageRank from a reaches both the related target and the wikilink target.
	pr := g.PersonalizedPageRank([]string{"a.md"}, 30, 0.85)
	require.Greater(t, pr[vault.NormalizeLink("sub/b.md")], 0.0)
	require.Greater(t, pr[vault.NormalizeLink("c.md")], 0.0)

	// the doc-to-code reference is captured on a's node for later drift binding.
	require.Contains(t, g.Nodes[vault.NormalizeLink("a.md")].CodeRefs, vault.Edge{Target: "internal/store/daemon.go", Kind: "inline-path"})
}

func TestCollectionScopedLinkResolution(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	// the same slug under specs/ and plans/.
	write("docs/specs/2026-06-26-0001-game.md", "---\ntitle: Spec\n---\n# Spec\nbody")
	write("docs/plans/2026-06-26-0001-game.md", "---\ntitle: Plan\n---\n# Plan\nbody")
	// an adr that qualifies its link to the spec.
	write("docs/adr/0001-decide.md", "---\ntitle: ADR\n---\n# ADR\nsee [[specs/2026-06-26-0001-game]]")
	// a second plan that links the shared slug unqualified; it must resolve in-collection.
	write("docs/plans/2026-06-26-0002-ref.md", "---\ntitle: Ref\n---\n# Ref\nsee [[2026-06-26-0001-game]]")

	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	specKey := vault.CollectionKey("docs/specs/2026-06-26-0001-game.md")
	planKey := vault.CollectionKey("docs/plans/2026-06-26-0001-game.md")
	adrKey := vault.CollectionKey("docs/adr/0001-decide.md")
	refKey := vault.CollectionKey("docs/plans/2026-06-26-0002-ref.md")

	// the shared slug is two distinct nodes, not one collision.
	require.NotEqual(t, specKey, planKey)
	_, hasSpec := g.Nodes[specKey]
	_, hasPlan := g.Nodes[planKey]
	require.True(t, hasSpec)
	require.True(t, hasPlan)

	// the qualified wikilink resolved to the spec, never the plan.
	require.Contains(t, g.Nodes[specKey].In, adrKey)
	require.NotContains(t, g.Nodes[planKey].In, adrKey)

	// the unqualified wikilink resolved in-collection to the plan, never the spec.
	require.Contains(t, g.Nodes[planKey].In, refKey)
	require.NotContains(t, g.Nodes[specKey].In, refKey)
}

func TestGitHubWikiSlugResolution(t *testing.T) {
	root := t.TempDir()
	write := func(name, content string) {
		p := filepath.Join(root, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o755))
		require.NoError(t, os.WriteFile(p, []byte(content), 0o644))
	}
	write("Home.md", "# Home\n\nsee [[Page Name]] and [[Install guide|Install Guide]]")
	write("Page-Name.md", "# Page Name\n")
	write("Install-Guide.md", "# Install Guide\n")

	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	require.Empty(t, g.BrokenLinks())
	require.Contains(t, g.Nodes[vault.NormalizeLink("Page-Name.md")].In, vault.NormalizeLink("Home.md"))
	require.Contains(t, g.Nodes[vault.NormalizeLink("Install-Guide.md")].In, vault.NormalizeLink("Home.md"))
}

func TestWikiStructuralPagesAreNotOrphans(t *testing.T) {
	root := t.TempDir()
	write := func(name string) {
		require.NoError(t, os.WriteFile(filepath.Join(root, name), []byte("# "+name+"\n"), 0o644))
	}
	write("Home.md")
	write("_Sidebar.md")
	write("_Footer.md")
	write("Loose.md")

	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	require.NotContains(t, g.Orphans(), "Home.md")
	require.NotContains(t, g.Orphans(), "_Sidebar.md")
	require.NotContains(t, g.Orphans(), "_Footer.md")
	require.Contains(t, g.Orphans(), "Loose.md")
}

func TestTopPageRankLimitAndEmpty(t *testing.T) {
	root := t.TempDir()
	for _, n := range []string{"a", "b", "c"} {
		require.NoError(t, os.WriteFile(filepath.Join(root, n+".md"),
			[]byte("---\ntitle: "+n+"\n---\n# "+n+"\n"), 0o644))
	}
	g, err := graph.Build(root, nil)
	require.NoError(t, err)

	require.Len(t, g.TopPageRank(2), 2)   // bounded by n
	require.Len(t, g.TopPageRank(0), 3)   // non-positive returns all
	require.Len(t, g.TopPageRank(100), 3) // n larger than the graph returns all

	empty := &graph.Graph{Nodes: map[string]graph.Node{}}
	require.Nil(t, empty.TopPageRank(5))

	// the entries carry the resolved path and title from the node.
	top := g.TopPageRank(3)
	require.NotEmpty(t, top[0].Path)
	require.Equal(t, vault.NormalizeLink(top[0].Path), vault.NormalizeLink(top[0].Title))
}
