package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/service"
)

func TestSearchAppliesMatchingResults(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")

	updated, _ := tab.Update(searchDoneMsg{
		query: "alpha",
		result: service.QueryResult{
			Query:         "alpha",
			RetrievalMode: service.RetrievalHybridSemantic,
			Hits: []index.Hit{
				{Path: "a.md", Title: "Alpha"},
				{Path: "b.md", Title: "Beta"},
			},
		},
		previews: map[string]string{"a.md": "# Alpha"},
	})
	tab = updated.(searchTab)

	require.Len(t, tab.result.Hits, 2)
	require.Equal(t, 0, tab.cursor)
	require.False(t, tab.loading)
	require.Equal(t, "hybrid", tab.retrievalLabel())
}

func TestSearchIgnoresStaleResults(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")
	tab.result.Hits = []index.Hit{{Path: "old.md", Title: "Old"}}

	updated, _ := tab.Update(searchDoneMsg{
		query: "beta",
		result: service.QueryResult{
			Hits: []index.Hit{{Path: "new.md", Title: "New"}},
		},
	})
	tab = updated.(searchTab)

	require.Len(t, tab.result.Hits, 1)
	require.Equal(t, "old.md", tab.result.Hits[0].Path)
}
