package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	teatest "github.com/charmbracelet/x/exp/teatest/v2"
	"github.com/stretchr/testify/require"

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
}

func TestAppViewRendersAllTabLabels(t *testing.T) {
	app := newApp(nil)
	app.width = 140
	app.height = 40
	out := app.View().Content
	for _, name := range tabNames {
		require.Contains(t, components.SanitizeText(out), name)
	}
	require.Contains(t, components.SanitizeText(out), "STARDUST")
}

func TestAppSmokeRendersBannerAndAllFiveTabs(t *testing.T) {
	tm := teatest.NewTestModel(t, newApp(nil), teatest.WithInitialTermSize(140, 40))
	for range tabNames[1:] {
		tm.Send(tea.KeyPressMsg{Code: tea.KeyRight})
		time.Sleep(20 * time.Millisecond)
	}
	tm.Send(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	tm.WaitFinished(t, teatest.WithFinalTimeout(2*time.Second))
	final := tm.FinalModel(t, teatest.WithFinalTimeout(2*time.Second)).(App)
	require.Equal(t, TabStatus, final.activeTab)
	out := components.SanitizeText(final.View().Content)
	require.Contains(t, out, "STARDUST")
	for _, name := range tabNames {
		require.True(t, strings.Contains(out, name), "missing tab label %s", name)
	}
}
