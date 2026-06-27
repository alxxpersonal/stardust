package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/index"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
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

func TestSearchResultsRenderCleanList(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")
	tab.result = service.QueryResult{
		RetrievalMode: service.RetrievalHybridSemantic,
		Hits: []index.Hit{
			{Path: "notes/alpha.md", Title: "Alpha", Snippet: "alpha match snippet", Score: 0.91},
			{Path: "notes/beta.md", Title: "Beta", Snippet: "beta match snippet", Score: 0.38},
		},
	}

	out := components.SanitizeText(tab.renderResultsList(80, 20))
	require.Contains(t, out, "Results")
	require.Contains(t, out, "◼")
	require.Contains(t, out, "◻")
	require.Contains(t, out, "alpha match snippet")
	require.NotContains(t, out, "|")
	require.NotContains(t, out, "─")
}

func TestSearchViewRendersResultsAndPreview(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")
	tab.result = service.QueryResult{
		RetrievalMode: service.RetrievalHybridSemantic,
		Hits: []index.Hit{
			{Path: "notes/alpha.md", Title: "Alpha", Snippet: "alpha match snippet", Score: 0.91},
			{Path: "notes/beta.md", Title: "Beta", Snippet: "beta match snippet", Score: 0.38},
		},
	}
	tab.previews = map[string]string{
		"notes/alpha.md": "# Alpha\n\nbody preview text",
	}

	out := components.SanitizeText(tab.View(140, 32))
	require.Contains(t, out, "retrieval: hybrid")
	require.Contains(t, out, "Alpha")
	require.Contains(t, out, "notes/alpha.md")
	require.Contains(t, out, "alpha match")
	require.Contains(t, out, "body preview text")
}

func TestSearchViewEmptyStateShowsHint(t *testing.T) {
	tab := newSearchTab(nil)

	out := components.SanitizeText(tab.View(140, 24))
	require.Contains(t, out, "Type to search your vault")
}
