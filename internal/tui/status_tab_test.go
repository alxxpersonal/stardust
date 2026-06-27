package tui

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestStatusLoadedStoresVaultStatus(t *testing.T) {
	tab := newStatusTab(nil)
	updated, _ := tab.Update(statusLoadedMsg{status: service.VaultStatus{
		Root:        "/tmp/vault",
		Initialized: true,
		Kind:        "code repo",
		Collections: []service.CollectionInfo{
			{Name: "specs", Records: 2, Path: "docs/specs"},
			{Name: "plans", Records: 1, Path: "docs/plans"},
		},
		Index: service.IndexHealth{
			Notes:            3,
			Vectors:          true,
			CommitsBehind:    1,
			HasCommitsBehind: true,
			LastIndexed:      "abcdef1234567890",
			EmbedModel:       "bge-m3",
		},
	}})
	tab = updated.(statusTab)

	require.True(t, tab.loaded)
	require.Equal(t, "/tmp/vault", tab.status.Root)
	require.Len(t, tab.status.Collections, 2)
	require.Equal(t, 3, tab.status.Index.Notes)
}

func TestStatusCollectionsRenderCleanList(t *testing.T) {
	tab := newStatusTab(nil)
	tab.status.Collections = []service.CollectionInfo{
		{Name: "specs", Records: 2, Path: "docs/specs", Description: "system specs"},
	}

	out := components.SanitizeText(tab.collections(90, 20))
	require.Contains(t, out, "Collections")
	require.Contains(t, out, "specs")
	require.Contains(t, out, "2")
	require.NotContains(t, out, "|")
	require.NotContains(t, out, "─")
}
