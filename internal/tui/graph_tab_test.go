package tui

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/graph"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestGraphLoadedStoresReport(t *testing.T) {
	tab := newGraphTab(nil)
	updated, _ := tab.Update(graphLoadedMsg{report: service.GraphReport{
		Notes:   3,
		Links:   4,
		Orphans: []string{"orphan.md"},
		Broken:  []graph.BrokenLink{{From: "a.md", Target: "missing"}},
		PageRank: []graph.PageRankEntry{
			{Path: "a.md", Title: "A", Score: 0.5},
		},
	}})
	tab = updated.(graphTab)

	require.True(t, tab.loaded)
	require.NoError(t, tab.err)
	require.Equal(t, 3, tab.report.Notes)
	require.Len(t, tab.report.PageRank, 1)
	require.Len(t, tab.report.Broken, 1)
}

func TestGraphLoadedStoresError(t *testing.T) {
	tab := newGraphTab(nil)
	want := errors.New("graph failed")
	updated, _ := tab.Update(graphLoadedMsg{err: want})
	tab = updated.(graphTab)

	require.True(t, tab.loaded)
	require.ErrorIs(t, tab.err, want)
}

func TestGraphListsRenderCleanSections(t *testing.T) {
	tab := newGraphTab(nil)
	tab.report = service.GraphReport{
		Orphans: []string{"orphan.md"},
		Broken:  []graph.BrokenLink{{From: "a.md", Target: "missing"}},
		PageRank: []graph.PageRankEntry{
			{Path: "a.md", Title: "Alpha", Score: 0.88},
		},
	}

	out := components.SanitizeText(tab.pageRankBox(80, 20) + "\n" + tab.orphansBox(80, 20) + "\n" + tab.brokenBox(80, 20))
	require.Contains(t, out, "PageRank")
	require.Contains(t, out, "Orphans")
	require.Contains(t, out, "Broken Links")
	require.Contains(t, out, "0.8800")
	require.NotContains(t, out, "|")
	require.NotContains(t, out, "─")
}
