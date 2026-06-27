package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
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

func TestSearchPreviewViewportScrolls(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")
	tab.Resize(140, 16)

	updated, _ := tab.Update(searchDoneMsg{
		query: "alpha",
		result: service.QueryResult{
			RetrievalMode: service.RetrievalHybridSemantic,
			Hits: []index.Hit{
				{Path: "notes/alpha.md", Title: "Alpha", Snippet: "alpha match snippet", Score: 0.91},
				{Path: "notes/beta.md", Title: "Beta", Snippet: "beta match snippet", Score: 0.38},
			},
		},
		previews: map[string]string{
			"notes/alpha.md": numberedListItems(80),
			"notes/beta.md":  numberedListItems(60),
		},
	})
	tab = updated.(searchTab)
	require.Greater(t, tab.previewViewport.TotalLineCount(), tab.previewViewport.VisibleLineCount())
	require.Zero(t, tab.previewViewport.YOffset())

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	tab = updated.(searchTab)
	require.Greater(t, tab.previewViewport.YOffset(), 0)

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab = updated.(searchTab)
	require.Equal(t, 1, tab.cursor)
	require.Equal(t, "notes/beta.md", tab.previewPath)
	require.Zero(t, tab.previewViewport.YOffset())

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	tab = updated.(searchTab)
	require.True(t, tab.previewViewport.AtBottom())
}

func TestSearchEnterOpensSelectedResultFullscreenAndEscReturns(t *testing.T) {
	tab := newSearchTab(nil)
	tab.input.SetValue("alpha")
	tab.Resize(140, 32)
	tab.result = service.QueryResult{
		RetrievalMode: service.RetrievalHybridSemantic,
		Hits: []index.Hit{
			{Path: "notes/alpha.md", Title: "Alpha", Snippet: "alpha match snippet", Score: 0.91},
			{Path: "notes/beta.md", Title: "Beta", Snippet: "beta match snippet", Score: 0.38},
		},
	}
	tab.previews = map[string]string{
		"notes/alpha.md": "# Alpha\n\nalpha body text",
		"notes/beta.md":  "# Beta\n\nbeta full document body",
	}
	tab.cursor = 1
	tab.refreshPreviewViewport(true)

	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(searchTab)

	require.Equal(t, searchViewDocument, tab.mode)
	require.Equal(t, "notes/beta.md", tab.docHit.Path)
	require.Zero(t, tab.docViewport.YOffset())
	require.Contains(t, components.SanitizeText(tab.docViewport.View()), "beta full document body")

	out := components.SanitizeText(tab.View(140, 32))
	require.Contains(t, out, "Beta")
	require.Contains(t, out, "notes/beta.md")
	require.Contains(t, out, "beta full document body")
	require.NotContains(t, out, "retrieval: hybrid")
	require.NotContains(t, out, "Results")

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEsc})
	tab = updated.(searchTab)

	require.Equal(t, searchViewSplit, tab.mode)
	require.Equal(t, "alpha", tab.input.Value())
	require.Equal(t, 1, tab.cursor)
	require.Equal(t, "notes/beta.md", tab.selectedHit().Path)

	out = components.SanitizeText(tab.View(140, 32))
	require.Contains(t, out, "retrieval: hybrid")
	require.Contains(t, out, "Results")
	require.Contains(t, out, "beta full document body")
}

func TestSearchViewEmptyStateShowsHint(t *testing.T) {
	tab := newSearchTab(nil)

	out := components.SanitizeText(tab.View(140, 24))
	require.Contains(t, out, "Type to search your vault")
}
