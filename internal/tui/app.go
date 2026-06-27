package tui

import (
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/tui/anim"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- App Model ---

// App is the root TUI model.
type App struct {
	be        *backend
	width     int
	height    int
	activeTab int
	frame     int

	searchTab   SearchTab
	browseTab   BrowseTab
	graphTab    GraphTab
	driftTab    DriftTab
	statusTab   StatusTab
	settingsTab SettingsTab
}

// newApp creates a new App model backed by the shared service backend.
func newApp(be *backend) App {
	app := App{
		be:        be,
		activeTab: TabSearch,
	}
	app.buildTabs(be)
	return app
}

// applySize fans the current width and height out to every tab.
func (a *App) applySize() {
	a.searchTab.Resize(a.width, a.height)
	a.browseTab.Resize(a.width, a.height)
	a.graphTab.Resize(a.width, a.height)
	a.driftTab.Resize(a.width, a.height)
	a.statusTab.Resize(a.width, a.height)
	a.settingsTab.Resize(a.width, a.height)
}

func (a *App) buildTabs(be *backend) {
	a.searchTab = NewSearchTab(be)
	a.browseTab = NewBrowseTab(be)
	a.graphTab = NewGraphTab(be)
	a.driftTab = NewDriftTab(be)
	a.statusTab = NewStatusTab(be)
	a.settingsTab = NewSettingsTab(be)
}

// Init implements tea.Model.
func (a App) Init() tea.Cmd {
	return tea.Batch(
		anim.FlameTick(),
		a.searchTab.Init(),
		a.browseTab.Init(),
		a.graphTab.Init(),
		a.driftTab.Init(),
		a.statusTab.Init(),
		a.settingsTab.Init(),
	)
}

// Update implements tea.Model.
func (a App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width = msg.Width
		a.height = msg.Height
		a.applySize()
		return a, nil

	case tea.KeyPressMsg:
		if msg.String() == "ctrl+c" {
			return a, tea.Quit
		}

		// Tab and shift+tab cycle tabs unconditionally, so a focused tab (the
		// search box, the settings editor) can always be left even when it owns
		// the arrow keys and digits.
		switch msg.String() {
		case "tab":
			a.activeTab = (a.activeTab + 1) % len(tabNames)
			return a, nil
		case "shift+tab":
			a.activeTab = (a.activeTab - 1 + len(tabNames)) % len(tabNames)
			return a, nil
		}

		// Tab-switching keys are gated behind the active tab's focus so a tab
		// that owns text input (the settings editor, the search box) receives
		// arrow keys and digits instead of switching tabs.
		if !a.activeTabModel().Focused() {
			switch msg.String() {
			case "left":
				if a.activeTab > 0 {
					a.activeTab--
				}
				return a, nil
			case "right":
				if a.activeTab < len(tabNames)-1 {
					a.activeTab++
				}
				return a, nil
			case "1":
				a.activeTab = TabSearch
				return a, nil
			case "2":
				a.activeTab = TabBrowse
				return a, nil
			case "3":
				a.activeTab = TabGraph
				return a, nil
			case "4":
				a.activeTab = TabDrift
				return a, nil
			case "5":
				a.activeTab = TabStatus
				return a, nil
			case "6":
				a.activeTab = TabSettings
				return a, nil
			}
		}
	}

	switch msg.(type) {
	case anim.FlameTickMsg:
		a.frame++
		a.syncFrame()
		return a, anim.FlameTick()
	case searchDoneMsg, spinner.TickMsg:
		updated, cmd := a.searchTab.Update(msg)
		a.searchTab = updated.(SearchTab)
		return a, cmd
	case collectionsLoadedMsg, recordsLoadedMsg, recordLoadedMsg:
		updated, cmd := a.browseTab.Update(msg)
		a.browseTab = updated.(BrowseTab)
		return a, cmd
	case graphLoadedMsg:
		updated, cmd := a.graphTab.Update(msg)
		a.graphTab = updated.(GraphTab)
		return a, cmd
	case driftLoadedMsg:
		updated, cmd := a.driftTab.Update(msg)
		a.driftTab = updated.(DriftTab)
		return a, cmd
	case statusLoadedMsg:
		updated, cmd := a.statusTab.Update(msg)
		a.statusTab = updated.(StatusTab)
		return a, cmd
	case settingsActionMsg, settingsCollectionsMsg:
		updated, cmd := a.settingsTab.Update(msg)
		a.settingsTab = updated.(SettingsTab)
		return a, cmd
	}

	switch a.activeTab {
	case TabSearch:
		updated, cmd := a.searchTab.Update(msg)
		a.searchTab = updated.(SearchTab)
		return a, cmd
	case TabBrowse:
		updated, cmd := a.browseTab.Update(msg)
		a.browseTab = updated.(BrowseTab)
		return a, cmd
	case TabGraph:
		updated, cmd := a.graphTab.Update(msg)
		a.graphTab = updated.(GraphTab)
		return a, cmd
	case TabDrift:
		updated, cmd := a.driftTab.Update(msg)
		a.driftTab = updated.(DriftTab)
		return a, cmd
	case TabStatus:
		updated, cmd := a.statusTab.Update(msg)
		a.statusTab = updated.(StatusTab)
		return a, cmd
	case TabSettings:
		updated, cmd := a.settingsTab.Update(msg)
		a.settingsTab = updated.(SettingsTab)
		return a, cmd
	}

	return a, nil
}

func (a *App) syncFrame() {
	a.searchTab.frame = a.frame
	a.browseTab.frame = a.frame
	a.graphTab.frame = a.frame
	a.driftTab.frame = a.frame
	a.statusTab.frame = a.frame
	a.settingsTab.frame = a.frame
}

// activeTabModel returns the currently active TabModel.
func (a App) activeTabModel() TabModel {
	switch a.activeTab {
	case TabSearch:
		return a.searchTab
	case TabBrowse:
		return a.browseTab
	case TabGraph:
		return a.graphTab
	case TabDrift:
		return a.driftTab
	case TabStatus:
		return a.statusTab
	case TabSettings:
		return a.settingsTab
	default:
		return a.searchTab
	}
}

// View implements tea.Model.
func (a App) View() tea.View {
	banner := centerBlockUniform(RenderBannerAnimated(a.frame), a.width)
	tabBar := renderTabBar(a.activeTab, a.width)
	statusBar := centerBlockUniform(
		components.StatusBarFromItems(a.activeTabModel().Hints(), a.width),
		a.width,
	)

	header := ""
	if label := a.activeTabModel().HeaderLabel(); label != "" {
		header = centerBlockUniform(promptHeaderBox(label, a.frame, tableWidth(a.width)), a.width) + "\n"
	}

	top := banner + "\n" + tabBar + "\n" + header
	topLines := countViewLines(top)
	statusBarLines := countViewLines(statusBar)

	extraLines := 3
	if a.activeTabModel().StatusLine() != "" {
		extraLines++
	}
	contentHeight := a.height - topLines - statusBarLines - extraLines
	if contentHeight < 3 {
		contentHeight = 3
	}

	var content string
	switch a.activeTab {
	case TabSearch:
		content = a.searchTab.View(a.width, contentHeight)
	case TabBrowse:
		content = a.browseTab.View(a.width, contentHeight)
	case TabGraph:
		content = a.graphTab.View(a.width, contentHeight)
	case TabDrift:
		content = a.driftTab.View(a.width, contentHeight)
	case TabStatus:
		content = a.statusTab.View(a.width, contentHeight)
	case TabSettings:
		content = a.settingsTab.View(a.width, contentHeight)
	}

	tabStatusLine := a.activeTabModel().StatusLine()
	statusLineRendered := ""
	if tabStatusLine != "" {
		statusLineRendered = "\n" + centerBlockUniform(tabStatusLine, a.width)
	}

	v := tea.NewView(top + content + statusLineRendered + "\n" + statusBar)
	v.AltScreen = true
	return v
}

// --- Helpers ---

func tableWidth(width int) int {
	target := (width * 80) / 100
	if target < 100 {
		target = 100
	}
	if target > 180 {
		target = 180
	}
	return target
}

func centerBlockUniform(s string, width int) string {
	if width <= 0 {
		return s
	}
	lines := strings.Split(s, "\n")
	maxWidth := 0
	for _, line := range lines {
		w := lipgloss.Width(line)
		if w > maxWidth {
			maxWidth = w
		}
	}
	if maxWidth <= 0 || maxWidth >= width {
		return s
	}
	pad := (width - maxWidth) / 2
	if pad <= 0 {
		return s
	}
	prefix := strings.Repeat(" ", pad)
	for i := range lines {
		if lines[i] != "" {
			lines[i] = prefix + lines[i]
		}
	}
	return strings.Join(lines, "\n")
}

func countViewLines(block string) int {
	if strings.TrimSpace(block) == "" {
		return 0
	}
	return strings.Count(block, "\n") + 1
}

func centerOverlay(block string, width, height int) string {
	centered := centerBlockUniform(block, width)
	pad := height - countViewLines(centered)
	if pad <= 0 {
		return centered
	}
	top := pad / 2
	return strings.Repeat("\n", top) + centered + strings.Repeat("\n", pad-top)
}

func promptHeaderBox(text string, frame, width int) string {
	a, b, c := anim.AnimatedBorderStops(frame)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForegroundBlend(a, b, c).
		Padding(0, 2).
		Render(lipgloss.NewStyle().Italic(true).Render(anim.Shimmer(text, frame)))
	return lipgloss.PlaceHorizontal(width, lipgloss.Center, box)
}
