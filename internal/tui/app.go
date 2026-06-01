package tui

import (
	"image/color"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// App is the root TUI model. It owns the tabs and routes messages to the active
// one, with a few global keys (tab cycling, quit) handled up front.
type App struct {
	be     *backend
	width  int
	height int
	active int
	frame  int
	search searchTab
	status statusTab
	graph  graphTab
}

// newApp builds the root model and its tabs over a shared backend.
func newApp(be *backend) *App {
	return &App{
		be:     be,
		search: newSearchTab(be),
		status: newStatusTab(be),
		graph:  newGraphTab(be),
	}
}

// Init starts every tab and the animation ticker.
func (a *App) Init() tea.Cmd {
	return tea.Batch(a.search.Init(), a.status.Init(), a.graph.Init(), animTick())
}

// Update handles global keys and animation, then delegates to the active tab.
// Async search/spinner messages always route to the search tab so a tab switch
// mid-search does not strand them.
func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tickMsg:
		a.frame++
		return a, animTick()
	case tea.WindowSizeMsg:
		a.width = m.Width
		a.height = m.Height
		return a, nil
	case searchDoneMsg, spinner.TickMsg:
		var cmd tea.Cmd
		a.search, cmd = a.search.Update(msg)
		return a, cmd
	case tea.KeyPressMsg:
		switch m.String() {
		case "ctrl+c":
			return a, tea.Quit
		case "tab":
			a.active = (a.active + 1) % tabCount
			return a, nil
		case "shift+tab":
			a.active = (a.active + tabCount - 1) % tabCount
			return a, nil
		}
		// digits and q drive tab switching / quit only off the search tab, where
		// they would otherwise be swallowed as query text
		if a.active != tabSearch {
			switch m.String() {
			case "1":
				a.active = tabSearch
				return a, nil
			case "2":
				a.active = tabStatus
				return a, nil
			case "3":
				a.active = tabGraph
				return a, nil
			case "q":
				return a, tea.Quit
			}
		}
	}
	return a.delegate(msg)
}

// delegate forwards a message to the active tab.
func (a *App) delegate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch a.active {
	case tabSearch:
		a.search, cmd = a.search.Update(msg)
	case tabStatus:
		a.status, cmd = a.status.Update(msg)
	case tabGraph:
		a.graph, cmd = a.graph.Update(msg)
	}
	return a, cmd
}

// View composes the banner, tab bar, active body, and status bar.
func (a *App) View() tea.View {
	width := a.width
	if width < 40 {
		width = 80
	}
	height := a.height
	if height < 12 {
		height = 24
	}

	bodyHeight := height - 7
	if bodyHeight < 4 {
		bodyHeight = 4
	}

	var body string
	switch a.active {
	case tabSearch:
		body = a.search.View(width, bodyHeight)
	case tabStatus:
		body = a.status.View(width, bodyHeight)
	case tabGraph:
		body = a.graph.View(width, bodyHeight)
	}

	content := lipgloss.JoinVertical(
		lipgloss.Left,
		a.renderBanner(),
		"",
		renderTabBar(a.active),
		"",
		body,
	)
	full := lipgloss.JoinVertical(lipgloss.Left, content, "", statusBar(a.hints(), width))

	v := tea.NewView(lipgloss.NewStyle().Padding(0, 1).Render(full))
	v.AltScreen = true
	return v
}

// renderBanner draws the wordmark with a slow shimmer between the palette colors.
func (a *App) renderBanner() string {
	colors := []color.Color{colorPrimary, colorSecondary, colorAccent}
	c := colors[(a.frame/4)%len(colors)]
	mark := lipgloss.NewStyle().Foreground(c).Bold(true).Render("✦ STARDUST")
	return mark + "  " + mutedStyle.Render("context engine")
}

// hints returns the status-bar hints for the active tab.
func (a *App) hints() []string {
	common := []string{hint("tab", "switch"), hint("1-3", "tabs"), hint("ctrl+c", "quit")}
	switch a.active {
	case tabSearch:
		return append([]string{hint("enter", "search"), hint("↑↓", "select")}, common...)
	default:
		return append([]string{hint("r", "refresh")}, common...)
	}
}
