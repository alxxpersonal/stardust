package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/service"
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

	workspaceLoaded bool
	workspaceErr    error
	workspaceStatus service.VaultStatus

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
	contentHeight := a.contentHeight()
	a.searchTab.Resize(a.width, contentHeight)
	a.browseTab.Resize(a.width, contentHeight)
	a.graphTab.Resize(a.width, contentHeight)
	a.driftTab.Resize(a.width, contentHeight)
	a.statusTab.Resize(a.width, contentHeight)
	a.settingsTab.Resize(a.width, contentHeight)
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

		// Tab and arrow tab keys cycle unconditionally so focused inputs never
		// trap the user on one tab.
		switch msg.String() {
		case "tab", "right":
			a.cycleTab(1)
			return a, nil
		case "shift+tab", "left":
			a.cycleTab(-1)
			return a, nil
		}

		// Digit jumps stay gated behind focus so text inputs can accept numeric
		// query and config values.
		if !a.activeTabModel().Focused() {
			switch msg.String() {
			case "1":
				a.activeTab = TabSearch
				a.applySize()
				return a, nil
			case "2":
				a.activeTab = TabBrowse
				a.applySize()
				return a, nil
			case "3":
				a.activeTab = TabGraph
				a.applySize()
				return a, nil
			case "4":
				a.activeTab = TabDrift
				a.applySize()
				return a, nil
			case "5":
				a.activeTab = TabStatus
				a.applySize()
				return a, nil
			case "6":
				a.activeTab = TabSettings
				a.applySize()
				return a, nil
			}
		}
	}

	switch msg := msg.(type) {
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
		a.workspaceErr = msg.err
		if msg.err == nil {
			a.workspaceLoaded = true
			a.workspaceStatus = msg.status
		}
		updated, cmd := a.statusTab.Update(msg)
		a.statusTab = updated.(StatusTab)
		return a, cmd
	case settingsActionMsg, settingsCollectionsMsg, settingsCollectionMutationMsg:
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

func (a *App) cycleTab(delta int) {
	a.activeTab = (a.activeTab + delta + len(tabNames)) % len(tabNames)
	a.applySize()
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

// --- Workspace chrome ---

func (a App) workspaceStatusLine() string {
	root := a.workspaceRoot()
	if root == "" {
		return ""
	}
	segments := []string{root}
	if context := a.workspaceContextSegment(); context != "" {
		segments = append(segments, context)
	}
	if summary := a.workspaceIndexSegment(); summary != "" {
		segments = append(segments, summary)
	}
	line := fitWorkspaceSegments(segments, a.width)
	if line == "" {
		return ""
	}
	return MutedStyle.Render(line)
}

func (a App) workspaceRoot() string {
	if a.workspaceStatus.Root != "" {
		return components.SanitizeOneLine(a.workspaceStatus.Root)
	}
	if a.be != nil && a.be.svc != nil {
		return components.SanitizeOneLine(a.be.svc.Layout.Root)
	}
	return ""
}

func (a App) workspaceContextSegment() string {
	if !a.workspaceLoaded {
		if a.workspaceErr != nil {
			return "status unavailable"
		}
		return "status loading"
	}
	status := a.workspaceStatus
	mode := workspaceMode(status)
	if !status.Repository.IsGit {
		return mode
	}
	name := status.Repository.Name
	if name == "" {
		name = filepath.Base(status.Root)
	}
	context := fmt.Sprintf("%s %s", mode, name)
	if status.Repository.Branch != "" {
		context += "@" + status.Repository.Branch
	}
	return components.SanitizeOneLine(context)
}

func (a App) workspaceIndexSegment() string {
	if !a.workspaceLoaded {
		return ""
	}
	summary := fmt.Sprintf("%d notes", a.workspaceStatus.Index.Notes)
	switch {
	case a.workspaceStatus.Repository.Head != "":
		summary += ", head " + shortSHA(a.workspaceStatus.Repository.Head)
	case a.workspaceStatus.Index.HasCommitsBehind:
		if a.workspaceStatus.Index.CommitsBehind == 0 {
			summary += ", fresh"
		} else {
			summary += fmt.Sprintf(", %d behind", a.workspaceStatus.Index.CommitsBehind)
		}
	case a.workspaceStatus.Index.LastIndexed != "":
		summary += ", index " + shortSHA(a.workspaceStatus.Index.LastIndexed)
	}
	return summary
}

func workspaceMode(status service.VaultStatus) string {
	if strings.Contains(status.Kind, "repo") {
		return "repo"
	}
	return "vault"
}

func shortSHA(sha string) string {
	sha = strings.TrimSpace(sha)
	if len(sha) <= 7 {
		return sha
	}
	return sha[:7]
}

func fitWorkspaceSegments(segments []string, width int) string {
	if len(segments) == 0 {
		return ""
	}
	if width <= 0 {
		return strings.Join(segments, " · ")
	}
	tail := compactWorkspaceSegments(segments[1:])
	for {
		line, ok := joinWorkspaceSegments(segments[0], tail, width)
		if ok {
			return line
		}
		if len(tail) == 0 {
			return truncateLeft(segments[0], width)
		}
		tail = tail[:len(tail)-1]
	}
}

func compactWorkspaceSegments(segments []string) []string {
	out := make([]string, 0, len(segments))
	for _, segment := range segments {
		if strings.TrimSpace(segment) != "" {
			out = append(out, segment)
		}
	}
	return out
}

func joinWorkspaceSegments(root string, tail []string, width int) (string, bool) {
	tailWidth := 0
	for _, segment := range tail {
		tailWidth += lipgloss.Width(segment)
	}
	separatorWidth := len(tail) * lipgloss.Width(" · ")
	rootWidth := width - tailWidth - separatorWidth
	if rootWidth <= 0 {
		return "", false
	}
	line := strings.Join(append([]string{truncateLeft(root, rootWidth)}, tail...), " · ")
	if lipgloss.Width(line) > width {
		return "", false
	}
	return line, true
}

func truncateLeft(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if lipgloss.Width(text) <= width {
		return text
	}
	if width <= 3 {
		return strings.Repeat(".", width)
	}
	runes := []rune(text)
	best := ""
	for i := len(runes) - 1; i >= 0; i-- {
		candidate := "..." + string(runes[i:])
		if lipgloss.Width(candidate) <= width {
			best = candidate
			continue
		}
		break
	}
	if best != "" {
		return best
	}
	return strings.Repeat(".", width)
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
	contentHeight := a.contentHeight()

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
	workspaceStatusLine := a.workspaceStatusLine()
	workspaceStatusRendered := ""
	if workspaceStatusLine != "" {
		workspaceStatusRendered = "\n" + centerBlockUniform(workspaceStatusLine, a.width)
	}

	v := tea.NewView(top + content + statusLineRendered + workspaceStatusRendered + "\n" + statusBar)
	v.AltScreen = true
	return v
}

func (a App) contentHeight() int {
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
	if a.workspaceStatusLine() != "" {
		extraLines++
	}
	contentHeight := a.height - topLines - statusBarLines - extraLines
	if contentHeight < 3 {
		contentHeight = 3
	}
	return contentHeight
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
