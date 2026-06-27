package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/config"
	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestSettingsRendersConfigValues(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.cfg = config.Config{
		EmbedModel:  "bge-m3",
		OllamaURL:   "http://localhost:11434",
		Ignore:      []string{".git"},
		RerankerURL: "",
	}
	out := components.SanitizeText(tab.View(120, 40))
	require.Contains(t, out, "Embed model")
	require.Contains(t, out, "bge-m3")
	require.Contains(t, out, "(disabled)") // empty reranker url
}

func TestSettingsCursorMoves(t *testing.T) {
	tab := newSettingsTab(nil)
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	tab = updated.(settingsTab)
	require.Equal(t, 1, tab.cursor)
}

func TestSettingsEditFocuses(t *testing.T) {
	tab := newSettingsTab(nil)
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter}) // enter on first text row
	tab = updated.(settingsTab)
	require.True(t, tab.Focused())
}

func TestSettingsCollectionsSubViewTransitions(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.cursor = 8 // "Inspect collections" action row
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(settingsTab)
	require.True(t, tab.Focused())
	require.Equal(t, settingsCollections, tab.sub)

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	tab = updated.(settingsTab)
	require.False(t, tab.Focused())
	require.Equal(t, settingsNone, tab.sub)
}

func TestSettingsCollectionsMsgStores(t *testing.T) {
	tab := newSettingsTab(nil)
	updated, _ := tab.Update(settingsCollectionsMsg{collections: []service.CollectionInfo{
		{Name: "specs", Records: 3, Path: "docs/specs"},
	}})
	tab = updated.(settingsTab)
	require.Len(t, tab.collections, 1)
	out := components.SanitizeText(tab.View(120, 40))
	require.Contains(t, out, "specs")
}

func TestSettingsActionBusyGuard(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.busy = true
	tab.cursor = 5 // "Reindex" action row
	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(settingsTab)
	require.Nil(t, cmd) // second action rejected while busy
	require.True(t, tab.busy)
}
