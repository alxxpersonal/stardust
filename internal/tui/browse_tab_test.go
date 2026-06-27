package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestBrowseNavigatesLoadedMessages(t *testing.T) {
	tab := newBrowseTab(nil)
	updated, _ := tab.Update(collectionsLoadedMsg{
		collections: []service.CollectionInfo{
			{Name: "specs", Records: 2},
			{Name: "plans", Records: 1},
		},
	})
	tab = updated.(browseTab)
	require.Equal(t, levelCollections, tab.level)
	require.Len(t, tab.collections, 2)

	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(browseTab)
	require.NotNil(t, cmd)
	require.True(t, tab.loading)

	updated, _ = tab.Update(recordsLoadedMsg{
		collection: "specs",
		records: service.RecordList{
			Collection: "specs",
			Records: []service.Record{
				{Path: "docs/specs/a.md", Title: "A"},
			},
		},
	})
	tab = updated.(browseTab)
	require.Equal(t, levelRecords, tab.level)
	require.Len(t, tab.records.Records, 1)

	updated, _ = tab.Update(recordLoadedMsg{
		record:   service.Record{Path: "docs/specs/a.md", Title: "A", Body: "# A"},
		rendered: "rendered body",
	})
	tab = updated.(browseTab)
	require.Equal(t, levelRecord, tab.level)
	require.Contains(t, tab.docViewport.GetContent(), "A")
}

func TestBrowseRecordViewportScrolls(t *testing.T) {
	tab := newBrowseTab(nil)
	tab.Resize(120, 12)

	updated, _ := tab.Update(recordLoadedMsg{
		record:   service.Record{Path: "docs/specs/a.md", Title: "A"},
		rendered: numberedLines(40),
	})
	tab = updated.(browseTab)
	require.Equal(t, levelRecord, tab.level)
	require.Greater(t, tab.docViewport.TotalLineCount(), tab.docViewport.VisibleLineCount())
	require.Zero(t, tab.docViewport.YOffset())

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab = updated.(browseTab)
	require.Equal(t, 1, tab.docViewport.YOffset())

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyPgDown})
	tab = updated.(browseTab)
	require.Greater(t, tab.docViewport.YOffset(), 1)

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	tab = updated.(browseTab)
	require.Zero(t, tab.docViewport.YOffset())

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	tab = updated.(browseTab)
	require.True(t, tab.docViewport.AtBottom())

	updated, _ = tab.Update(recordLoadedMsg{
		record:   service.Record{Path: "docs/specs/b.md", Title: "B"},
		rendered: numberedLines(30),
	})
	tab = updated.(browseTab)
	require.Zero(t, tab.docViewport.YOffset())
	require.Contains(t, tab.docViewport.GetContent(), "line 01")
}

func TestBrowseRecordsRenderCleanListDropsEmptyUpdated(t *testing.T) {
	tab := newBrowseTab(nil)
	tab.level = levelRecords
	tab.records = service.RecordList{
		Collection: "specs",
		Records: []service.Record{
			{Path: "docs/specs/alpha.md", Title: "Alpha Spec"},
			{Path: "docs/specs/beta.md", Title: "Beta Spec"},
		},
	}

	out := components.SanitizeText(tab.View(120, 30))
	require.Contains(t, out, "specs")
	require.Contains(t, out, "◼")
	require.Contains(t, out, "◻")
	require.Contains(t, out, "docs/specs/alpha.md")
	require.NotContains(t, out, "Updated")
}

func numberedLines(count int) string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = fmt.Sprintf("line %02d", i+1)
	}
	return strings.Join(lines, "\n")
}

func numberedListItems(count int) string {
	lines := make([]string, count)
	for i := range lines {
		lines[i] = fmt.Sprintf("- line %02d", i+1)
	}
	return strings.Join(lines, "\n")
}
