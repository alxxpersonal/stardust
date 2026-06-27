package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Tab Constants ---

const (
	TabSearch   = 0
	TabBrowse   = 1
	TabGraph    = 2
	TabDrift    = 3
	TabStatus   = 4
	TabSettings = 5
)

var tabNames = []string{"Search", "Browse", "Graph", "Drift", "Status", "Settings"}

// TabModel is the interface that each tab must implement.
type TabModel interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (TabModel, tea.Cmd)
	View(width, height int) string
	Hints() []components.HintItem
	Focused() bool
	StatusLine() string
	HeaderLabel() string
}

// renderTabBar renders the tab bar with the active tab highlighted.
func renderTabBar(activeTab int, width int) string {
	segments := make([]string, 0, len(tabNames))
	for i, name := range tabNames {
		if i == activeTab {
			segments = append(segments, TabActiveStyle.Render(name))
		} else {
			segments = append(segments, TabInactiveStyle.Render(name))
		}
	}
	bar := lipgloss.JoinHorizontal(lipgloss.Top, segments...)
	return centerBlockUniform(bar, width)
}
