package tui

import (
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
	require.Equal(t, "rendered body", tab.rendered)
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
