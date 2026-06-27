package tui

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/collections"
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
		SourceRoot:  "../source",
	}
	out := components.SanitizeText(tab.View(120, 40))
	require.Contains(t, out, "Embed model")
	require.Contains(t, out, "bge-m3")
	require.Contains(t, out, "(disabled)") // empty reranker url
	require.Contains(t, out, "Source root")
	require.Contains(t, out, "../source")
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
	tab.cursor = 9 // "Manage collections" action row
	updated, _ := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(settingsTab)
	require.True(t, tab.Focused())
	require.Equal(t, settingsCollections, tab.sub)

	updated, _ = tab.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	tab = updated.(settingsTab)
	require.False(t, tab.Focused())
	require.Equal(t, settingsNone, tab.sub)
}

func TestSettingsCollectionsRowLabelIsManage(t *testing.T) {
	tab := newSettingsTab(nil)
	out := components.SanitizeText(tab.View(120, 40))
	require.Contains(t, out, "Manage collections")
	require.NotContains(t, out, "In"+"spect collections")
	require.NotContains(t, strings.ToLower(out), "in"+"spect")
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

func TestSettingsRendersSingleCollectionsHeader(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.collections = []service.CollectionInfo{
		{Name: "specs", Records: 3, Path: "docs/specs"},
		{Name: "plans", Records: 1, Path: "docs/plans"},
	}
	out := components.SanitizeText(tab.View(140, 40))
	// exactly one "Collections" keycap header, no duplicate box title.
	require.Equal(t, 1, strings.Count(out, "Collections"))
	// CONFIG keeps its single box title (plain rows, no keycap).
	require.Equal(t, 1, strings.Count(out, "CONFIG"))
	require.Contains(t, out, "Embed model")
}

func TestSettingsEmptyCollectionsStillLabeled(t *testing.T) {
	tab := newSettingsTab(nil)
	out := components.SanitizeText(tab.View(140, 40))
	// even with no collections, the section carries exactly one header.
	require.Equal(t, 1, strings.Count(out, "Collections"))
	require.Contains(t, out, "no collections configured")
}

func TestSettingsMainHintsFollowFocusedSection(t *testing.T) {
	tab := newSettingsTab(nil)
	requireSettingsHint(t, tab.Hints(), "enter", "edit/run")
	require.False(t, hasSettingsHint(tab.Hints(), "s", "schema"))

	tab = moveSettingsToCollections(t, tab)
	hints := tab.Hints()
	requireSettingsHint(t, hints, "n", "add")
	requireSettingsHint(t, hints, "enter", "edit")
	requireSettingsHint(t, hints, "s", "schema")
	requireSettingsHint(t, hints, "dd", "delete")
	require.False(t, hasSettingsHint(hints, "enter", "edit/run"))
}

func TestSettingsActionBusyGuard(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.busy = true
	tab.cursor = 6 // "Reindex" action row
	updated, cmd := tab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	tab = updated.(settingsTab)
	require.Nil(t, cmd) // second action rejected while busy
	require.True(t, tab.busy)
}

func TestSettingsMainCollectionsNavigateAndMutateThroughService(t *testing.T) {
	ctx := context.Background()
	root := settingsTestVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	tab := newSettingsTab(&backend{svc: svc})
	tab = moveSettingsToCollections(t, tab)
	require.Equal(t, settingsFocusCollections, tab.focus)
	require.Equal(t, 0, tab.colCursor)

	updated, _ := tab.Update(settingsKey("n"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionFormAdd, tab.collectionForm.mode)
	require.True(t, tab.Focused())

	tab.collectionForm.input.SetValue("notes")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.collectionForm.input.SetValue("docs/notes")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.collectionForm.input.SetValue("quick notes")
	updated, cmd := tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	require.False(t, tab.busy)
	require.Equal(t, settingsFocusCollections, tab.focus)
	got, err := svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, "docs/notes", got.Path)

	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionFormEdit, tab.collectionForm.mode)
	tab.collectionForm.input.SetValue("docs/notes-edited")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.collectionForm.input.SetValue("edited quick notes")
	updated, cmd = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err = svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, "docs/notes-edited", got.Path)
	require.Equal(t, "edited quick notes", got.Description)

	updated, _ = tab.Update(settingsKey("s"))
	tab = updated.(settingsTab)
	require.Equal(t, settingsSchema, tab.sub)
	require.True(t, tab.schemaFromMain)
	require.Equal(t, "notes", tab.schemaName)

	updated, _ = tab.Update(settingsKey("n"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue("state")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue(collections.TypeString)
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue("false")
	updated, cmd = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err = svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, []collections.Field{{Name: "state", Type: collections.TypeString}}, got.Fields)

	updated, _ = tab.Update(settingsKey("esc"))
	tab = updated.(settingsTab)
	require.Equal(t, settingsNone, tab.sub)
	require.Equal(t, settingsFocusCollections, tab.focus)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "notes-edited"), 0o755))
	docPath := filepath.Join(root, "docs", "notes-edited", "keep.md")
	require.NoError(t, os.WriteFile(docPath, []byte("# keep\n"), 0o644))

	updated, _ = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	require.True(t, tab.confirmingCollectionDelete)
	updated, cmd = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	require.Empty(t, tab.collections)
	_, err = os.Stat(docPath)
	require.NoError(t, err)
}

func TestSettingsCollectionEditingMutatesThroughService(t *testing.T) {
	ctx := context.Background()
	root := settingsTestVault(t)
	svc, err := service.Open(ctx, root)
	require.NoError(t, err)
	defer func() { _ = svc.Close() }()

	tab := newSettingsTab(&backend{svc: svc})
	tab.sub = settingsCollections

	updated, _ := tab.Update(settingsKey("n"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionFormAdd, tab.collectionForm.mode)
	require.Equal(t, collectionStepName, tab.collectionForm.step)

	tab.collectionForm.input.SetValue("notes")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionStepPath, tab.collectionForm.step)

	tab.collectionForm.input.SetValue("docs/notes")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionStepDescription, tab.collectionForm.step)

	tab.collectionForm.input.SetValue("quick notes")
	updated, cmd := tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.True(t, tab.busy)
	tab = runSettingsCmd(t, tab, cmd)
	require.False(t, tab.busy)
	require.Len(t, tab.collections, 1)
	require.Equal(t, "notes", tab.collections[0].Name)

	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionFormEdit, tab.collectionForm.mode)
	require.Equal(t, collectionStepPath, tab.collectionForm.step)

	tab.collectionForm.input.SetValue("docs/notes-edited")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.collectionForm.input.SetValue("edited quick notes")
	updated, cmd = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err := svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, "docs/notes-edited", got.Path)
	require.Equal(t, "edited quick notes", got.Description)

	updated, _ = tab.Update(settingsKey("s"))
	tab = updated.(settingsTab)
	require.Equal(t, settingsSchema, tab.sub)
	require.Equal(t, "notes", tab.schemaName)

	updated, _ = tab.Update(settingsKey("n"))
	tab = updated.(settingsTab)
	require.Equal(t, fieldFormAdd, tab.fieldForm.mode)
	require.Equal(t, fieldStepName, tab.fieldForm.step)

	tab.fieldForm.input.SetValue("status")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue(collections.TypeEnum)
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue("true")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, fieldStepEnum, tab.fieldForm.step)
	tab.fieldForm.input.SetValue("draft, done")
	updated, cmd = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err = svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, []collections.Field{{
		Name:     "status",
		Type:     collections.TypeEnum,
		Required: true,
		Enum:     []string{"draft", "done"},
	}}, got.Fields)

	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, fieldFormEdit, tab.fieldForm.mode)
	tab.fieldForm.input.SetValue("state")
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue(collections.TypeString)
	updated, _ = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab.fieldForm.input.SetValue("false")
	updated, cmd = tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err = svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Equal(t, []collections.Field{{Name: "state", Type: collections.TypeString}}, got.Fields)

	updated, _ = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	require.True(t, tab.confirmingFieldDelete)
	updated, cmd = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	got, err = svc.GetCollection(ctx, "notes")
	require.NoError(t, err)
	require.Empty(t, got.Fields)

	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "notes-edited"), 0o755))
	docPath := filepath.Join(root, "docs", "notes-edited", "keep.md")
	require.NoError(t, os.WriteFile(docPath, []byte("# keep\n"), 0o644))

	updated, _ = tab.Update(settingsKey("esc"))
	tab = updated.(settingsTab)
	require.Equal(t, settingsCollections, tab.sub)
	updated, _ = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	require.True(t, tab.confirmingCollectionDelete)
	updated, cmd = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	tab = runSettingsCmd(t, tab, cmd)
	require.Empty(t, tab.collections)
	_, err = os.Stat(docPath)
	require.NoError(t, err)
}

func TestSettingsReferenceCollectionIsNotEditable(t *testing.T) {
	tab := newSettingsTab(nil)
	tab.sub = settingsCollections
	tab.collections = []service.CollectionInfo{{Name: "reference", Path: "docs", Description: "general docs"}}

	updated, _ := tab.Update(settingsKey("enter"))
	tab = updated.(settingsTab)
	require.Equal(t, collectionFormNone, tab.collectionForm.mode)
	require.Contains(t, components.SanitizeText(tab.note), "not editable")

	updated, _ = tab.Update(settingsKey("s"))
	tab = updated.(settingsTab)
	require.Equal(t, settingsCollections, tab.sub)

	updated, _ = tab.Update(settingsKey("d"))
	tab = updated.(settingsTab)
	require.False(t, tab.confirmingCollectionDelete)
}

func settingsTestVault(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, ".stardust", "cache"), 0o755))
	require.NoError(t, config.Save(config.Layout{Root: root}.Config(), config.Default()))
	return root
}

func runSettingsCmd(t *testing.T, tab settingsTab, cmd tea.Cmd) settingsTab {
	t.Helper()
	require.NotNil(t, cmd)
	updated, _ := tab.Update(cmd())
	return updated.(settingsTab)
}

func moveSettingsToCollections(t *testing.T, tab settingsTab) settingsTab {
	t.Helper()
	for range settingsRows {
		updated, _ := tab.Update(settingsKey("down"))
		tab = updated.(settingsTab)
	}
	require.Equal(t, settingsFocusCollections, tab.focus)
	return tab
}

func requireSettingsHint(t *testing.T, hints []components.HintItem, key, desc string) {
	t.Helper()
	require.True(t, hasSettingsHint(hints, key, desc), "missing hint %s %s in %#v", key, desc, hints)
}

func hasSettingsHint(hints []components.HintItem, key, desc string) bool {
	for _, hint := range hints {
		if hint.Key == key && hint.Desc == desc {
			return true
		}
	}
	return false
}

func settingsKey(s string) tea.KeyPressMsg {
	switch s {
	case "enter":
		return tea.KeyPressMsg{Code: tea.KeyEnter}
	case "esc":
		return tea.KeyPressMsg{Code: tea.KeyEscape}
	case "up":
		return tea.KeyPressMsg{Code: tea.KeyUp}
	case "down":
		return tea.KeyPressMsg{Code: tea.KeyDown}
	default:
		r := []rune(s)
		return tea.KeyPressMsg{Text: s, Code: r[0]}
	}
}
