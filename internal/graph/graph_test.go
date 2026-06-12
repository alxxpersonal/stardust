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
