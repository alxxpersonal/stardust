package tui

import (
	"context"
	"fmt"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/alxxpersonal/stardust/internal/service"
	"github.com/alxxpersonal/stardust/internal/tui/components"
)

// --- Graph Messages ---

// graphLoadedMsg carries the derived graph report back to the graph tab.
type graphLoadedMsg struct {
	report service.GraphReport
	err    error
}

// --- Graph Tab ---

// GraphTab reports the link graph: counts, PageRank, orphans, and broken links.
type GraphTab struct {
	be     *backend
	loaded bool
	err    error
	report service.GraphReport
	width  int
	height int
	frame  int
}

type graphTab = GraphTab

// NewGraphTab creates the graph tab.
func NewGraphTab(be *backend) GraphTab {
	return GraphTab{be: be}
}

func newGraphTab(be *backend) GraphTab {
	return NewGraphTab(be)
}

// Resize stores the latest terminal size.
func (t *GraphTab) Resize(width, height int) {
	t.width = width
	t.height = height
}

// Init derives the graph.
func (t GraphTab) Init() tea.Cmd { return t.load() }

func (t GraphTab) load() tea.Cmd {
	be := t.be
	return func() tea.Msg {
		if be == nil || be.svc == nil {
			return graphLoadedMsg{err: fmt.Errorf("graph unavailable: service is not open")}
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		report, err := be.svc.Graph(ctx)
		return graphLoadedMsg{report: report, err: err}
	}
}

// Update applies the derived graph and handles reload.
func (t GraphTab) Update(msg tea.Msg) (TabModel, tea.Cmd) {
	switch msg := msg.(type) {
	case graphLoadedMsg:
		t.loaded = true
		t.err = msg.err
		if msg.err == nil {
			t.report = msg.report
		}
	case tea.KeyPressMsg:
		if msg.String() == "r" {
			t.loaded = false
			return t, t.load()
		}
	}
	return t, nil
}

// View renders the graph report.
func (t GraphTab) View(width, height int) string {
	if width <= 0 {
		width = t.width
	}
	if height <= 0 {
		height = t.height
	}
	if t.err != nil {
		return centerBlockUniform(components.ErrorBox("graph failed", t.err.Error(), tableWidth(width)), width)
	}
	if !t.loaded {
		return centerOverlay(animatedBox(MutedStyle.Render("deriving graph..."), t.frame), width, height)
	}

	cardW := tableWidth(width)
	boxHeight := (height - 4) / 2
	if boxHeight < 6 {
		boxHeight = 6
	}
	leftW := (cardW / 2) - 2
	if leftW < 44 {
		leftW = 44
	}
	rightW := cardW - leftW - 4
	if rightW < 44 {
		rightW = 44
	}

	top := lipgloss.JoinHorizontal(
		lipgloss.Top,
		animatedRoundedBox("", t.pageRankBox(leftW, boxHeight), t.frame),
		"  ",
		animatedRoundedBox("", t.orphansBox(rightW, boxHeight), t.frame),
	)
	broken := animatedDoubleBox("", t.brokenBox(cardW, boxHeight), t.frame)

	return centerBlockUniform(top+"\n\n"+broken, width)
}

// Hints returns the key hints for the graph tab.
func (t GraphTab) Hints() []components.HintItem {
	return withCommonHints(components.HintItem{Key: "r", Desc: "refresh"})
}

// Focused reports whether graph owns keyboard text.
func (t GraphTab) Focused() bool { return false }

// StatusLine returns the graph tab status text.
func (t GraphTab) StatusLine() string {
	if !t.loaded {
		return MutedStyle.Render("graph loading")
	}
	return MutedStyle.Render(fmt.Sprintf("%d notes · %d links · %d orphans · %d broken",
		t.report.Notes, t.report.Links, len(t.report.Orphans), len(t.report.Broken)))
}

// HeaderLabel returns the shared animated header label.
func (t GraphTab) HeaderLabel() string {
	return "graph · notes and links"
}

func (t GraphTab) pageRankBox(width, height int) string {
	if len(t.report.PageRank) == 0 {
		return clipLines(lipgloss.NewStyle().Width(width).Render(renderCleanListHeader("PageRank", "", width)+"\n\n"+MutedStyle.Render("none")), height)
	}
	rows := make([]cleanListRow, 0, len(t.report.PageRank))
	for i, entry := range t.report.PageRank {
		title := entry.Title
		if title == "" {
			title = entry.Path
		}
		rows = append(rows, cleanListRow{Cells: []string{
			fmt.Sprintf("%02d", i+1),
			components.SanitizeOneLine(title),
			components.SanitizeOneLine(entry.Path),
			fmt.Sprintf("%.4f", entry.Score),
		}})
	}
	cols := []cleanListColumn{
		{Header: "#", MinWidth: 2, MaxWidth: 3, Align: lipgloss.Right, Muted: true},
		{Header: "Note", MinWidth: 14, Primary: true},
		{Header: "Path", MinWidth: 10, MaxWidth: 34, Muted: true, Underline: true},
		{Header: "Rank", MinWidth: 6, MaxWidth: 8, Align: lipgloss.Right, Numeric: true},
	}
	return clipLines(renderCleanList("PageRank", cleanListCountLabel(len(t.report.PageRank), "note"), cols, rows, width, -1), height)
}

func (t GraphTab) orphansBox(width, height int) string {
	if len(t.report.Orphans) == 0 {
		return clipLines(lipgloss.NewStyle().Width(width).Render(renderCleanListHeader("Orphans", "", width)+"\n\n"+SuccessStyle.Render("none")), height)
	}
	rows := make([]cleanListRow, 0, len(t.report.Orphans))
	for i, path := range t.report.Orphans {
		rows = append(rows, cleanListRow{Cells: []string{fmt.Sprintf("%02d", i+1), components.SanitizeOneLine(path)}})
	}
	cols := []cleanListColumn{
		{Header: "#", MinWidth: 2, MaxWidth: 3, Align: lipgloss.Right, Muted: true},
		{Header: "Path", MinWidth: 20, Primary: true, Underline: true},
	}
	return clipLines(renderCleanList("Orphans", cleanListCountLabel(len(t.report.Orphans), "note"), cols, rows, width, -1), height)
}

func (t GraphTab) brokenBox(width, height int) string {
	if len(t.report.Broken) == 0 {
		return clipLines(lipgloss.NewStyle().Width(width).Render(renderCleanListHeader("Broken links", "", width)+"\n\n"+SuccessStyle.Render("none")), height)
	}
	rows := make([]cleanListRow, 0, len(t.report.Broken))
	for _, broken := range t.report.Broken {
		rows = append(rows, cleanListRow{Cells: []string{
			components.SanitizeOneLine(broken.From),
			"[[" + components.SanitizeOneLine(broken.Target) + "]]",
		}})
	}
	cols := []cleanListColumn{
		{Header: "From", MinWidth: 24, Primary: true, Underline: true},
		{Header: "Target", MinWidth: 18, MaxWidth: 64, Warning: true},
	}
	return clipLines(renderCleanList("Broken Links", cleanListCountLabel(len(t.report.Broken), "link"), cols, rows, width, -1), height)
}
