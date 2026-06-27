package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/stretchr/testify/require"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

func TestAppDigitSwitchingAndFocusedGating(t *testing.T) {
	app := newApp(nil)
	model, _ := app.Update(tea.KeyPressMsg{Code: '2', Text: "2"})
	got := model.(App)
	require.Equal(t, TabSearch, got.activeTab)

	got.activeTab = TabBrowse
	model, _ = got.Update(tea.KeyPressMsg{Code: '3', Text: "3"})
	got = model.(App)
	require.Equal(t, TabGraph, got.activeTab)
}

func TestAppArrowCycle(t *testing.T) {
	app := newApp(nil)
	app.activeTab = TabStatus
	model, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	got := model.(App)
	require.Equal(t, TabDrift, got.activeTab)

	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got = model.(App)
	require.Equal(t, TabStatus, got.activeTab)

	got.activeTab = TabSearch
	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	got = model.(App)
	require.Equal(t, TabSettings, got.activeTab)

	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got = model.(App)
	require.Equal(t, TabSearch, got.activeTab)
}

func TestAppHasSixTabs(t *testing.T) {
	require.Equal(t, 6, len(tabNames))
	require.Equal(t, "Settings", tabNames[TabSettings])
}

func TestAppJumpToSettings(t *testing.T) {
	app := newApp(nil)
	app.activeTab = TabStatus // not focused, so digit jumps are live
	model, _ := app.Update(tea.KeyPressMsg{Code: '6', Text: "6"})
	got := model.(App)
	require.Equal(t, TabSettings, got.activeTab)
}

func TestAppFocusedTabAllowsArrowTabSwitching(t *testing.T) {
	app := newApp(nil)
	require.True(t, app.activeTabModel().Focused())

	model, _ := app.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got := model.(App)
	require.Equal(t, TabBrowse, got.activeTab)

	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	got = model.(App)
	require.Equal(t, TabSearch, got.activeTab)

	app = newApp(nil)
	app.activeTab = TabSettings
	// drive the settings tab into the inline editor so it reports Focused()
	updated, _ := app.settingsTab.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	app.settingsTab = updated.(SettingsTab)
	require.True(t, app.activeTabModel().Focused())

	model, _ = app.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	got = model.(App)
	require.Equal(t, TabStatus, got.activeTab)

	model, _ = got.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	got = model.(App)
	require.Equal(t, TabSettings, got.activeTab)

	model, _ = got.Update(tea.KeyPressMsg{Code: '1', Text: "1"})
	got = model.(App)
	require.Equal(t, TabSettings, got.activeTab)
}

func TestAppViewRendersAllTabLabels(t *testing.T) {
	app := newApp(nil)
	app.width = 140
	app.height = 40
	out := app.View().Content
	for _, name := range tabNames {
		require.Contains(t, components.SanitizeText(out), name)
	}
	require.Contains(t, components.SanitizeText(out), "█████████")
	require.Contains(t, components.SanitizeText(out), "Local-First Markdown Context Engine")
	require.NotContains(t, components.SanitizeText(out), "STARDUST Local-First Markdown Context Engine")
}

func TestAppDriftTabRendersSingleHeaders(t *testing.T) {
	app := newApp(nil)
	app.width = 140
	app.height = 40
	app.activeTab = TabDrift
	app.driftTab.loaded = true
	app.driftTab.drift = service.DriftResult{Docs: []service.DriftDoc{
		{DocPath: "docs/specs/a.md", Title: "Spec A", Type: "spec",
			Bindings: []service.DriftBinding{{File: "internal/a.go", ChangedCommits: 3}}},
	}}
	app.driftTab.stale = service.StaleResult{Docs: []service.GovernedDoc{
		{DocPath: "docs/plans/b.md", Title: "Plan B", Type: "plan", Status: "Implemented",
			ChangedCommits: 2, Matched: []string{"internal/b.go"}},
	}}
	out := components.SanitizeText(app.View().Content)
	require.Equal(t, 1, strings.Count(out, "Drifted Docs"))
	require.Equal(t, 1, strings.Count(out, "Stale Docs"))
}

func TestAppSmokeRendersBannerAndSettingsTab(t *testing.T) {
	app := newApp(nil)
	app.activeTab = TabSettings
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(140, 40))
	time.Sleep(50 * time.Millisecond)
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(App)
	require.Equal(t, TabSettings, final.activeTab)
	out := components.SanitizeText(final.View().Content)
	require.Contains(t, out, "█████████")
	require.Contains(t, out, "Local-First Markdown Context Engine")
	for _, name := range tabNames {
		require.True(t, strings.Contains(out, name), "missing tab label %s", name)
	}
	require.Contains(t, out, "Embed model") // settings config box renders
}
